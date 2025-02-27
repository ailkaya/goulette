package sentry

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"
)

type ILoadBalancer interface {
	RegisterTopic(topic string)
	RegisterBroker(any)
	GetBrokers(any) []string
	RemoveTopic(topic string)
	RemoveBroker(addr string)
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
)

type Ring interface {
	AddTopicNode(location uint64, alive *int64)
	AddBrokerNode(addr string, location uint64, status *int64)
	RemoveNode(location uint64)
	GetNextBroker(location uint64) (string, error)
	Clean()
}

type ConsistentHash struct {
	// 环结构
	ring Ring
	// 用来标识依赖关系的图
	dag *flowGraph
	// 无接收端的broker.topic，topic->[]broker
	waitAdopt map[string][]string
	// 记录某个topic/broker是否有效，为区分topic和broker，查询时需要使用stringJoin("broker/topic", addr, true)作为key
	// 1代表活跃，2代表正常关闭，3代表非正常关闭(etcd lease)，0代表其相关信息已从哈希环上移除，2,3都代表其信息仍然在环上
	alive map[string]*int64
	// topic的默认分片数
	topicPartition int
	// broker的默认分片数
	brokerPartition int
	// 环更新任务队列，避免并发问题
	updates chan func()
}

func NewConsistentHash(opts ...Option) *ConsistentHash {
	ch := &ConsistentHash{
		ring:            newSkipList(MaxHeight, Ratio),
		waitAdopt:       make(map[string][]string),
		alive:           make(map[string]*int64),
		topicPartition:  DefaultTopicPartition,
		brokerPartition: DefaultBrokerPartition,
		updates:         make(chan func(), MaxTaskWaiting),
	}

	for _, opt := range opts {
		opt.Apply(ch)
	}
	go ch.exec()
	go ch.clean()
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
	Addr   string
	weight float64
}

func (ch *ConsistentHash) RegisterBroker(i any) {
	ch.updates <- func() {
		if param, ok := i.(*ConsistentHashRegisterBrokerParams); ok {
			addr, weight := stringJoin("broker", param.Addr, true), param.weight
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
	if val == 0 {
		return true
	} else if val == 1 || val == 2 || val == 3 {
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

func (ch *ConsistentHash) GetBrokers(i any) []string {
	switch param := i.(type) {
	case *ConsistentHashGetBrokersParamsOfProducer:
		return ch.getBrokersForProducer(param.Topic)
	case *ConsistentHashGetBrokersParamsOfConsumer:
		topic, addrTo := param.Topic, param.ConsumerAddr
		brokers := ch.getBrokersForProducer(topic)
		if len(brokers) > 0 {
			for _, addrFrom := range brokers {
				ch.dag.addEdge(stringJoin(addrFrom, topic, true), stringJoin(addrTo, topic, true))
			}
		}
	default:
		panic("invalid consistent hash param")
	}
	return nil
}

func (ch *ConsistentHash) RemoveTopic(topic string) {
	ch.updates <- func() {
		addr := stringJoin("topic", topic, true)
		if ch.dag.isBroker(addr) {
			panic("the node must be a topic node")
		}
		*ch.alive[addr] = 2
	}
}

// RemoveBroker broker正常关闭，置其alive为2
func (ch *ConsistentHash) RemoveBroker(addr string) {
	ch.updates <- func() {
		addr := stringJoin("broker", addr, true)
		if !ch.dag.isBroker(addr) {
			panic("the node must be a broker node")
		}
		// 设置为2代表正常关闭
		*ch.alive[addr] = 2
	}
}

func (ch *ConsistentHash) Close() {
	ch.updates <- func() {
		// TODO: 持久化
		close(ch.updates)
	}
}

func (ch *ConsistentHash) getBrokersForProducer(topic string) []string {
	brokers := make([]string, ch.topicPartition)
	for i := 0; i < ch.topicPartition; i++ {
		addr, err := ch.ring.GetNextBroker(ch.hashStringToUint64(stringJoin(topic, strconv.Itoa(i), true)))
		// 无broker可用时直接返回
		if err == nil {
			brokers = brokers[:0]
			return brokers
		}
		brokers[i] = addr
	}
	return brokers
}

func (ch *ConsistentHash) hashStringToUint64(s string) (result uint64) {
	// 创建 SHA-256 哈希对象
	h := fnv.New64a()
	// 写入字符串数据
	h.Write([]byte(s))
	// 计算哈希值
	hash := h.Sum(nil)
	// 使用binary包将字节转换为 uint64
	result = binary.BigEndian.Uint64(hash[:8]) // 取前8个字节
	return
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

type flowGraph struct {
	locations map[string]*graphNode
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

// 移除一条broker与producer/container之间的边，注意若from/to中有一个是broker而另一个必定为producer/consumer
// 返回出度变为0，需要被加入到全局消费队列中的broker的地址
func (g *flowGraph) removeEdge(from, to string) (string, error) {
	src, ok := g.locations[from]
	if !ok {
		return "", fmt.Errorf("cannot remove edge from %s", from)
	}
	dst, ok := g.locations[to]
	if !ok {
		return "", fmt.Errorf("cannot remove edge to %s", to)
	}
	delete(src.relate, to)
	delete(dst.relate, from)

	var val *graphNode
	if src.category == 0 {
		val = src
	} else if dst.category == 0 {
		val = dst
	} else {
		panic("invalid edge category")
	}

	val.out--
	if val.out == 0 {
		return val.key, nil
	}

	return "", nil
}

// 从图中移除一个节点
// 返回受到影响后出度变为0，需要加入到全局消费队列中的broker的地址
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

func (g *flowGraph) isBroker(key string) bool {
	if val, ok := g.locations[key]; ok {
		return val.category == 0
	} else {
		panic("broker not exist")
	}
}
