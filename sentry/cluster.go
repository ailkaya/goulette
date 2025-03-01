package sentry

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coreos/etcd/storage/storagepb"
	"github.com/google/uuid"
	"go.etcd.io/etcd/clientv3"
	"log"
	"time"
)

const (
	OperationBasePath = "goulette/sentry/operate/"

	RegisterTopicFunc         = 1
	RegisterBrokerFunc        = 2
	GetBrokersFunc            = 3
	RemoveTopicFunc           = 4
	RemoveBrokerFunc          = 5
	RemoveTopicFromBrokerFunc = 6
)

type EtcdCli struct {
	ctx    context.Context
	cancel context.CancelFunc
	cli    *clientv3.Client
	cosh   *ConsistentHash
}

func NewEtcdCli(cli *clientv3.Client, cosh *ConsistentHash) *EtcdCli {
	ctx, cancel := context.WithCancel(context.Background())
	ec := &EtcdCli{
		ctx:    ctx,
		cancel: cancel,
		cli:    cli,
		cosh:   cosh,
	}
	go ec.listen()
	return ec
}

type operation struct {
	// 指明操作类型(removeTopicFromBroker/register/remove)
	meta     int32
	contents []any
}

func (ec *EtcdCli) RegisterTopic(topic string) {
	//ec.cosh.RegisterTopic(topic)
	op := &operation{
		meta:     RegisterTopicFunc,
		contents: []any{topic},
	}
	_, err := ec.cli.Put(ec.ctx, stringJoin(OperationBasePath, uuid.NewString(), false), serialize(op))
	if err != nil {
		// TODO: 错误处理
	}
}

func (ec *EtcdCli) RegisterBroker(x any) {
	//ec.cosh.RegisterBroker(x)
	op := &operation{
		meta:     RegisterBrokerFunc,
		contents: []any{x},
	}
	_, err := ec.cli.Put(ec.ctx, stringJoin(OperationBasePath, uuid.NewString(), false), serialize(op))
	if err != nil {
		// TODO: 错误处理
	}
}

func (ec *EtcdCli) GetBrokers(x any) ([]string, error) {
	op := &operation{
		meta:     GetBrokersFunc,
		contents: []any{x},
	}
	_, err := ec.cli.Put(ec.ctx, stringJoin(OperationBasePath, uuid.NewString(), false), serialize(op))
	if err != nil {
		// TODO: 错误处理
	}
	return ec.GetBrokers(x)
}

func (ec *EtcdCli) RemoveTopic(topic string) {
	//ec.cosh.RemoveTopic(topic)
	op := &operation{
		meta:     RemoveTopicFunc,
		contents: []any{topic},
	}
	_, err := ec.cli.Put(ec.ctx, stringJoin(OperationBasePath, uuid.NewString(), false), serialize(op))
	if err != nil {
		// TODO: 错误处理
	}
}

func (ec *EtcdCli) RemoveBroker(addr string) {
	//ec.cosh.RemoveBroker(addr)
	op := &operation{
		meta:     RemoveBrokerFunc,
		contents: []any{addr},
	}
	_, err := ec.cli.Put(ec.ctx, stringJoin(OperationBasePath, uuid.NewString(), false), serialize(op))
	if err != nil {
		// TODO: 错误处理
	}
}

func (ec *EtcdCli) RemoveTopicFromBroker(addr string, topic string) {
	//ec.cosh.RemoveTopicFromBroker(addr, topic)
	op := &operation{
		meta:     RemoveTopicFromBrokerFunc,
		contents: []any{addr, topic},
	}
	_, err := ec.cli.Put(ec.ctx, stringJoin(OperationBasePath, uuid.NewString(), false), serialize(op))
	if err != nil {
		// TODO: 错误处理
	}
}

func (ec *EtcdCli) Close() {
	ec.cancel()
	ec.cli.Close()
}

func (ec *EtcdCli) listen() {
	watcher := clientv3.NewWatcher(ec.cli)
	defer watcher.Close()
	for events := range watcher.Watch(ec.ctx, OperationBasePath, clientv3.WithPrefix()) {
		for _, event := range events.Events {
			switch event.Type {
			case storagepb.PUT:
				deserialized := deserialize(event.Kv.Value)
				switch deserialized.meta {
				case RegisterTopicFunc:
					ec.cosh.RegisterTopic(deserialized.contents[0].(string))
				case RegisterBrokerFunc:
					ec.cosh.RegisterBroker(deserialized.contents[0])
				case GetBrokersFunc:
					ec.cosh.GetBrokers(deserialized.contents[0])
				case RemoveTopicFunc:
					ec.cosh.RemoveTopic(deserialized.contents[0].(string))
				case RemoveBrokerFunc:
					ec.cosh.RemoveBroker(deserialized.contents[0].(string))
				case RemoveTopicFromBrokerFunc:
					ec.cosh.RemoveTopicFromBroker(deserialized.contents[0].(string), deserialized.contents[1].(string))
				}
			}
		}
	}
}

func serialize(op *operation) string {
	marshalled, err := json.Marshal(op)
	if err != nil {
		log.Fatal(err)
	}
	return string(marshalled)
}

func deserialize(op []byte) *operation {
	unmarshalled := &operation{}
	err := json.Unmarshal(op, unmarshalled)
	if err != nil {
		log.Fatal(err)
	}
	return unmarshalled
}

// GetEtcdLeader 查询 etcd 集群中的 leader
func GetEtcdLeader(endpoints []string) (string, error) {
	// 创建 etcd 客户端
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create etcd client: %w", err)
	}
	defer cli.Close()

	// 获取成员信息
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := cli.MemberLeader(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get member list: %w", err)
	}

	return resp.PeerURLs[0], nil
}
