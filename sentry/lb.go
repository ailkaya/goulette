package sentry

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/ailkaya/goulette/singleton"
	"github.com/coreos/etcd/storage/storagepb"
	"go.etcd.io/etcd/clientv3"
	"hash/fnv"
	"strconv"
	"strings"
	"time"
)

type ILoadBalancer interface {
	RegisterTopic(topic string)
	RegisterBroker(any)
	GetBrokers(any) ([]string, error)
	RemoveTopic(topic string)
	RemoveBroker(addr string)
	RemoveTopicFromBroker(addr string, topic string)
	Close()
}

type Option interface {
	Apply(hash *ConsistentHash)
}

type RingOption struct {
	ring Ring
}

func WithRing(ring Ring) *RingOption {
	return &RingOption{ring: ring}
}

func (ro *RingOption) Apply(hash *ConsistentHash) {
	hash.ring = ro.ring
}

type TopicPartitionOption struct {
	partition int
}

func WithTopicPartition(partition int) *TopicPartitionOption {
	return &TopicPartitionOption{partition}
}

func (tp *TopicPartitionOption) Apply(hash *ConsistentHash) {
	hash.topicPartition = tp.partition
}

type BrokerPartitionOption struct {
	partition int
}

func WithBrokerPartition(partition int) *BrokerPartitionOption {
	return &BrokerPartitionOption{partition}
}

func (bp *BrokerPartitionOption) Apply(hash *ConsistentHash) {
	hash.brokerPartition = bp.partition
}

const (
	MaxHeight              = 64
	Ratio                  = 2
	DefaultBrokerPartition = 1
	DefaultTopicPartition  = 1
	MaxTaskWaiting         = 10000
	SplitS                 = "\t\n\r"

	Removed = 0
	Alive   = 1
	Closed  = 2
	Closing = 3
)

type ConsistentHash struct {
	ctx    context.Context
	cancel context.CancelFunc
	// 环结构
	ring Ring
	// 用来标识依赖关系的图
	dag *flowGraph
	// etcd client
	client *clientv3.Client
	// 无接收端的broker.topic，topic->[]broker
	// 只进不出，重复的broker addr由client去重，只有当某个consumer报告某个broker.topic已不存在时才主动将其从中移除
	waitAdopt map[string][]string
	// 记录某个topic/broker是否有效，为区分topic和broker，查询时需要使用stringJoin("broker/topic", addr, true)作为key
	// 1代表活跃，2代表关闭，3代表处于只发不受状态，0代表其相关信息已从哈希环上移除，2,3都代表其信息仍然在环上
	alive map[string]*int64
	// 各个consumer所持有的topic
	// 在consumer调用查询接口时更新其拥有的topic(这里默认某个consumer从始至终所使用到的topics不会减少)
	consumer2topic map[string][]string
	// 各个broker所拥有的topic
	// consumer查询时更新
	brokers2topic map[string][]string
	// topic的默认分片数
	topicPartition int
	// broker的默认分片数
	brokerPartition int
	// 环更新任务队列，避免并发问题
	updates chan func()
}

func NewConsistentHash(opts ...Option) *ConsistentHash {
	ctx, cancel := context.WithCancel(context.Background())
	ch := &ConsistentHash{
		ctx:             ctx,
		cancel:          cancel,
		ring:            newSkipList(MaxHeight, Ratio),
		dag:             newFlowGraph(),
		client:          singleton.GetEtcdCli(),
		waitAdopt:       make(map[string][]string),
		alive:           make(map[string]*int64),
		consumer2topic:  make(map[string][]string),
		topicPartition:  DefaultTopicPartition,
		brokerPartition: DefaultBrokerPartition,
		updates:         make(chan func(), MaxTaskWaiting),
	}

	for _, opt := range opts {
		opt.Apply(ch)
	}
	go ch.exec()
	go ch.clean()
	ch.watch()
	return ch
}

func (ch *ConsistentHash) exec() {
	for f := range ch.updates {
		f()
	}
}

// 定期清理哈希环上的冗余节点
func (ch *ConsistentHash) clean() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		ch.ring.Clean()
	}
}

// 监听etcd注册表，对下线的consumer/broker做相应处理
func (ch *ConsistentHash) watch() {
	consumerPrefix := singleton.BasePath + "/consumer/"
	brokerPrefix := singleton.BasePath + "/broker/"
	//producerWatcher := ch.client.Watch(ch.ctx, singleton.BasePath+"/producer", clientv3.WithPrefix())
	consumerWatcher := ch.client.Watch(ch.ctx, consumerPrefix, clientv3.WithPrefix())
	brokerWatcher := ch.client.Watch(ch.ctx, brokerPrefix, clientv3.WithPrefix())

	go func() {
		for watchResp := range consumerWatcher {
			for _, ev := range watchResp.Events {
				switch ev.Type {
				case storagepb.EXPIRE:
					addr := strings.TrimPrefix(ev.Kv.String(), consumerPrefix)
					ch.updates <- func() {
						for _, topic := range ch.consumer2topic[addr] {
							// consumer下线时必须处理其所带来的影响
							effects := ch.dag.removeNode(stringJoin(addr, topic, true))
							for _, effect := range effects {
								t := stringParse(effect)
								brokerAddr, brokerTopic := t[0], t[1]
								joinToSlice(ch.waitAdopt[brokerTopic], brokerAddr)
							}
						}
					}
				default:
				}
			}
		}
	}()

	go func() {
		for watchResp := range brokerWatcher {
			for _, ev := range watchResp.Events {
				switch ev.Type {
				case storagepb.EXPIRE:
					addr := strings.TrimPrefix(ev.Kv.String(), brokerPrefix)
					ch.updates <- func() {
						for _, topic := range ch.brokers2topic[addr] {
							ch.dag.removeNode(stringJoin(addr, topic, true))
						}
						ch.brokerExpire(addr)
					}
				default:
				}
			}
		}
	}()
}

func (ch *ConsistentHash) RegisterTopic(topic string) {
	ch.updates <- func() {
		addr := stringJoin("topic", topic, true)
		if ch.checkAlive(addr) {
			alive := ch.alive[addr]
			for i := 0; i < ch.topicPartition; i++ {
				pos := ch.hashStringToUint64(stringJoin(addr, strconv.Itoa(i), true))
				ch.ring.AddTopicNode(pos, alive)
			}
		}
	}
}

type ConsistentHashRegisterBrokerParams struct {
	Addr         string
	WeightFactor float64
}

func (ch *ConsistentHash) RegisterBroker(i any) {
	ch.updates <- func() {
		if param, ok := i.(*ConsistentHashRegisterBrokerParams); ok {
			addr, weight := stringJoin("broker", param.Addr, true), param.WeightFactor
			if ch.checkAlive(addr) {
				alive := ch.alive[addr]
				for i := 0; i < int(float64(ch.brokerPartition)*weight); i++ {
					pos := ch.hashStringToUint64(stringJoin(addr, strconv.Itoa(i), true))
					ch.ring.AddBrokerNode(addr, pos, alive)
				}
			}
		} else {
			panic("invalid register broker param")
		}
	}
}

// 判断是否需要重新插点
func (ch *ConsistentHash) checkAlive(addr string) bool {
	if _, ok := ch.alive[addr]; !ok {
		ch.alive[addr] = new(int64)
		*ch.alive[addr] = 0
	}
	val := *ch.alive[addr]
	*ch.alive[addr] = 1
	if val == Removed || val == Closing {
		return true
	} else if val == Alive || val == Closed {
		return false
	} else {
		panic("invalid value of alive:" + strconv.Itoa(int(val)))
	}
}

type ConsistentHashGetBrokersParamsOfProducer struct {
	Topic string
}

type ConsistentHashGetBrokersParamsOfConsumer struct {
	Topic        string
	ConsumerAddr string
}

func (ch *ConsistentHash) GetBrokers(i any) ([]string, error) {
	var res []string
	switch param := i.(type) {
	case *ConsistentHashGetBrokersParamsOfProducer:
		res = ch.getBrokers(param.Topic, false)
	case *ConsistentHashGetBrokersParamsOfConsumer:
		topic, addrTo := param.Topic, param.ConsumerAddr
		if _, ok := ch.consumer2topic[addrTo]; !ok {
			ch.consumer2topic[addrTo] = make([]string, 0, 1)
		}
		joinToSlice(ch.consumer2topic[addrTo], topic)

		res = ch.getBrokers(topic, true)
		if len(res) > 0 {
			for _, addrFrom := range res {
				if _, ok := ch.brokers2topic[addrFrom]; !ok {
					ch.brokers2topic[addrFrom] = make([]string, 0, 1)
				}
				joinToSlice(ch.brokers2topic[addrFrom], topic)
				ch.dag.addEdge(stringJoin(addrFrom, topic, true), stringJoin(addrTo, topic, true))
			}
		}
	default:
		panic("invalid consistent hash param")
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no broker available")
	}
	return res, nil
}

func (ch *ConsistentHash) getBrokers(topic string, forConsumer bool) []string {
	brokers := make([]string, ch.topicPartition)
	for i := 0; i < ch.topicPartition; i++ {
		addr, err := ch.ring.GetNextBroker(ch.hashStringToUint64(stringJoin(topic, strconv.Itoa(i), true)), forConsumer)
		// 无broker可用时直接返回
		if err == nil {
			brokers = brokers[:0]
			return brokers
		}
		brokers[i] = addr
	}
	return brokers
}

func (ch *ConsistentHash) RemoveTopic(topic string) {
	ch.updates <- func() {
		addr := stringJoin("topic", topic, true)
		if ch.dag.isBroker(addr) {
			panic("the node must be a topic node")
		}
		*ch.alive[addr] = Closed
		ch.dag.removeNode(addr)
	}
}

// RemoveBroker broker正常关闭，置其alive为2
func (ch *ConsistentHash) RemoveBroker(addr string) {
	ch.updates <- func() {
		addr := stringJoin("broker", addr, true)
		if !ch.dag.isBroker(addr) {
			panic("the node must be a broker node")
		}
		// 设置为2代表关闭
		*ch.alive[addr] = Closed
		// 正常关闭可忽略返回值
		ch.dag.removeNode(addr)
	}
}

func (ch *ConsistentHash) brokerExpire(addr string) {
	ch.updates <- func() {
		addr := stringJoin("broker", addr, true)
		if !ch.dag.isBroker(addr) {
			panic("the node must be a broker node")
		}
		*ch.alive[addr] = Closed
		// 由于broker已经下线，因此也不用处理其影响
		ch.dag.removeNode(addr)
	}
}

func (ch *ConsistentHash) Close() {
	ch.updates <- func() {
		// TODO: 持久化
		close(ch.updates)
		ch.cancel()
	}
}

func (ch *ConsistentHash) RemoveTopicFromBroker(targetAddr, topic string) {
	// 从全局消费队列中移除相关topic下的targetAddr
	eraseFromSlice(ch.waitAdopt[topic], targetAddr)
}

func (ch *ConsistentHash) hashStringToUint64(s string) (result uint64) {
	h := fnv.New64a()
	h.Write([]byte(s))
	hash := h.Sum(nil)
	result = binary.BigEndian.Uint64(hash[:8]) // 取前8个字节
	return
}

func joinToSlice(slice []string, s string) {
	if !existInSlice(slice, s) {
		slice = append(slice, s)
	}
}

func eraseFromSlice(slice []string, s string) {
	for i := 0; i < len(slice); i++ {
		if slice[i] == s {
			slice = append(slice[:i], slice[i+1:]...)
		}
	}
}

func existInSlice(slice []string, s string) bool {
	for _, str := range slice {
		if str == s {
			return true
		}
	}
	return false
}

func stringJoin(s1, s2 string, withInterval bool) string {
	sb := strings.Builder{}
	sb.WriteString(s1)
	if withInterval {
		sb.WriteString(SplitS)
	}
	sb.WriteString(s2)
	return sb.String()
}

func stringParse(str string) []string {
	return strings.Split(str, SplitS)
}

type flowGraph struct {
	// 为区分consumer和broker，查询时需要使用stringJoin(consumer/broker addr, topic, true)作为key
	locations map[string]*graphNode
}

func newFlowGraph() *flowGraph {
	return &flowGraph{
		locations: make(map[string]*graphNode),
	}
}

type graphNode struct {
	// 0代表类型为broker，1代表producer/consumer
	category int
	// 出度数，仅针对broker类型
	out int
	key string
	// 当类型为broker时，relate为它所指向的producer/consumer，当类型为producer/consumer时，relate为指向该producer/consumer的brokers
	relate map[string]*graphNode
}

func newGraphNode(category int, key string) *graphNode {
	gn := &graphNode{
		category: category,
		out:      0,
		key:      key,
	}
	gn.relate = make(map[string]*graphNode)
	return gn
}

// from必须是broker类型而to必须是producer/consumer类型
// 为区分consumer和broker，查询时需要使用stringJoin(consumer/broker addr, topic, true)作为key
func (g *flowGraph) addEdge(from, to string) {
	src, ok := g.locations[from]
	if !ok {
		g.locations[from] = newGraphNode(0, from)
	}
	dst, ok := g.locations[to]
	if !ok {
		g.locations[to] = newGraphNode(1, to)
	}

	if src.category != 0 || dst.category != 1 {
		panic("wrong input")
	}

	src.relate[to], dst.relate[from] = dst, src
	src.out++
}

// 移除一条broker与producer/container之间的边，注意from的类型必须是broker而to的类型必须为producer/consumer
// 返回出度变为0，需要被加入到全局消费队列中的broker的地址(准确来说返回的是addr+SplitS+topic)，返回后调用stringParse来获取addr和topic
// 为区分consumer和broker，查询时需要使用stringJoin(consumer/broker addr, topic, true)作为key
func (g *flowGraph) removeEdge(from, to string) (string, error) {
	src, ok := g.locations[from]
	if !ok {
		return "", fmt.Errorf("cannot remove edge from %s", from)
	}
	dst, ok := g.locations[to]
	if !ok {
		return "", fmt.Errorf("cannot remove edge to %s", to)
	}
	if src.category != 0 || dst.category != 1 {
		panic("wrong input")
	}
	delete(src.relate, to)
	delete(dst.relate, from)

	src.out--
	if src.out == 0 {
		return src.key, nil
	}

	return "", nil
}

// 从图中移除一个节点
// 返回受到影响后出度变为0，需要加入到全局消费队列中的broker的地址
// 为区分consumer和broker，查询时需要使用stringJoin(consumer/broker addr, topic, true)作为key
func (g *flowGraph) removeNode(key string) []string {
	if val, ok := g.locations[key]; ok {
		res := make([]string, 0, 2)
		if val.category == 1 {
			// 解除所有对val的引用
			for _, cur := range val.relate {
				if cur != nil {
					cur.relate[key] = nil
					cur.out--
					if cur.category != 0 || cur.out < 0 {
						panic("invalid out")
					}
					if cur.out == 0 {
						res = append(res, cur.key)
					}
				}
			}
		} else {
			// 解除所有对val的引用
			for _, cur := range val.relate {
				if cur != nil {
					cur.relate[key] = nil
					if cur.category != 1 {
						panic("invalid out")
					}
				}
			}
		}
		g.locations[key] = nil
		return res
	}
	return nil
}

// 为区分consumer和broker，查询时需要使用stringJoin(consumer/broker addr, topic, true)作为key
func (g *flowGraph) hasEdge(key string) (bool, error) {
	val, ok := g.locations[key]
	if !ok {
		return false, fmt.Errorf("node %s not exist", key)
	}
	if val.category != 0 {
		return false, fmt.Errorf("cannot judge %s because it is not broker node", key)
	}

	for _, cur := range val.relate {
		if cur == nil {
			delete(val.relate, key)
		} else {
			return true, nil
		}
	}

	return false, nil
}

// 为区分consumer和broker，查询时需要使用stringJoin(consumer/broker addr, topic, true)作为key
func (g *flowGraph) isBroker(key string) bool {
	if val, ok := g.locations[key]; ok {
		return val.category == 0
	} else {
		panic("broker not exist")
	}
}
