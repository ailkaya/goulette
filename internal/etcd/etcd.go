package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/ailkaya/goulette/internal/utils"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	// Topic副本前缀
	TopicReplicaPrefix = "/goulette/topics/"
	// 操作日志前缀
	OperationLogPrefix = "/goulette/operations/"
	// 递增ID前缀
	IncrementIDPrefix = "/goulette/increment/"
	// Broker心跳前缀
	BrokerHeartbeatPrefix = "/goulette/brokers/"
)

// EtcdManager etcd管理器
type EtcdManager struct {
	client     *clientv3.Client
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
	isLeader   bool
	leaderAddr string

	// lease管理
	leaseMu sync.RWMutex
	leases  map[string]clientv3.LeaseID // brokerID -> leaseID
}

// TopicReplicaInfo Topic副本信息
type TopicReplicaInfo struct {
	Topic        string       `json:"topic"`
	Brokers      []BrokerInfo `json:"brokers"`
	Version      int64        `json:"version"`
	Timestamp    int64        `json:"timestamp"`
	ReplicaCount int          `json:"replica_count"`
}

// BrokerInfo Broker信息
type BrokerInfo struct {
	BrokerID string `json:"broker_id"`
	Address  string `json:"address"`
	Status   int    `json:"status"`
}

// OperationLog 操作日志
type OperationLog struct {
	ID        int64  `json:"id"`
	Function  string `json:"function"`
	Params    string `json:"params"`
	Timestamp int64  `json:"timestamp"`
}

// NewEtcdManager 创建新的etcd管理器
func NewEtcdManager(endpoints []string) (*EtcdManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
		Context:     ctx,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建etcd客户端失败: %w", err)
	}

	manager := &EtcdManager{
		client: client,
		ctx:    ctx,
		cancel: cancel,
		leases: make(map[string]clientv3.LeaseID),
	}

	// 启动leader选举
	go manager.electLeader()

	return manager, nil
}

// Close 关闭etcd管理器
func (em *EtcdManager) Close() error {
	em.cancel()
	return em.client.Close()
}

// electLeader 选举leader
func (em *EtcdManager) electLeader() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-em.ctx.Done():
			return
		case <-ticker.C:
			em.checkLeader()
		}
	}
}

// checkLeader 检查leader状态
func (em *EtcdManager) checkLeader() {
	resp, err := em.client.Status(em.ctx, em.client.Endpoints()[0])
	if err != nil {
		utils.Warnf("检查etcd状态失败: %v", err)
		return
	}

	em.mu.Lock()
	em.isLeader = resp.Leader == resp.Header.MemberId

	// 通过成员ID获取leader地址
	em.leaderAddr = ""
	if resp.Leader != 0 {
		// 获取集群成员信息
		members, err := em.client.MemberList(em.ctx)
		if err == nil {
			for _, member := range members.Members {
				if member.ID == resp.Leader && len(member.ClientURLs) > 0 {
					em.leaderAddr = member.ClientURLs[0]
					break
				}
			}
		}
	}
	em.mu.Unlock()

	if em.isLeader {
		utils.Debugf("当前节点为etcd leader")
	}
}

// IsLeader 检查是否为leader
func (em *EtcdManager) IsLeader() bool {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.isLeader
}

// GetLeaderAddr 获取leader地址
func (em *EtcdManager) GetLeaderAddr() string {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.leaderAddr
}

// getNextID 获取下一个递增ID
func (em *EtcdManager) getNextID(key string) (int64, error) {
	resp, err := em.client.Get(em.ctx, key)
	if err != nil {
		return 0, fmt.Errorf("获取递增ID失败: %w", err)
	}

	var currentID int64
	if len(resp.Kvs) > 0 {
		currentID, err = strconv.ParseInt(string(resp.Kvs[0].Value), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("解析递增ID失败: %w", err)
		}
	}

	nextID := currentID + 1

	// 使用事务确保原子性
	txn := em.client.Txn(em.ctx)
	txn.If(clientv3.Compare(clientv3.Value(key), "=", strconv.FormatInt(currentID, 10)))
	txn.Then(clientv3.OpPut(key, strconv.FormatInt(nextID, 10)))

	resp2, err := txn.Commit()
	if err != nil {
		return 0, fmt.Errorf("更新递增ID失败: %w", err)
	}

	if !resp2.Succeeded {
		// 重试
		return em.getNextID(key)
	}

	return nextID, nil
}

// LogOperation 记录操作日志
func (em *EtcdManager) LogOperation(function string, params interface{}) error {
	// 获取递增ID
	idKey := IncrementIDPrefix + "operation"
	nextID, err := em.getNextID(idKey)
	if err != nil {
		return fmt.Errorf("获取操作ID失败: %w", err)
	}

	// 序列化参数
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("序列化参数失败: %w", err)
	}

	operation := OperationLog{
		ID:        nextID,
		Function:  function,
		Params:    string(paramsBytes),
		Timestamp: time.Now().Unix(),
	}

	operationBytes, err := json.Marshal(operation)
	if err != nil {
		return fmt.Errorf("序列化操作日志失败: %w", err)
	}

	// 写入etcd
	key := fmt.Sprintf("%s%d", OperationLogPrefix, nextID)
	_, err = em.client.Put(em.ctx, key, string(operationBytes))
	if err != nil {
		return fmt.Errorf("写入操作日志失败: %w", err)
	}

	utils.Debugf("记录操作日志: %s, ID: %d", function, nextID)
	return nil
}

// GetTopicReplicas 获取Topic副本信息
func (em *EtcdManager) GetTopicReplicas(topic string) (*TopicReplicaInfo, error) {
	key := TopicReplicaPrefix + topic

	resp, err := em.client.Get(em.ctx, key)
	if err != nil {
		if err == rpctypes.ErrKeyNotFound {
			return nil, nil // 返回nil表示没有找到
		}
		return nil, fmt.Errorf("获取Topic副本信息失败: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, nil
	}

	var replicaInfo TopicReplicaInfo
	err = json.Unmarshal(resp.Kvs[0].Value, &replicaInfo)
	if err != nil {
		return nil, fmt.Errorf("解析Topic副本信息失败: %w", err)
	}

	return &replicaInfo, nil
}

// SetTopicReplicas 设置Topic副本信息
func (em *EtcdManager) SetTopicReplicas(topic string, brokers []BrokerInfo, replicaCount int) error {
	key := TopicReplicaPrefix + topic

	replicaInfo := TopicReplicaInfo{
		Topic:        topic,
		Brokers:      brokers,
		Version:      time.Now().UnixNano(),
		Timestamp:    time.Now().Unix(),
		ReplicaCount: replicaCount,
	}

	replicaBytes, err := json.Marshal(replicaInfo)
	if err != nil {
		return fmt.Errorf("序列化Topic副本信息失败: %w", err)
	}

	_, err = em.client.Put(em.ctx, key, string(replicaBytes))
	if err != nil {
		return fmt.Errorf("设置Topic副本信息失败: %w", err)
	}

	utils.Debugf("设置Topic副本信息: %s, 副本数: %d", topic, len(brokers))
	return nil
}

// DeleteTopicReplicas 删除Topic副本信息
func (em *EtcdManager) DeleteTopicReplicas(topic string) error {
	key := TopicReplicaPrefix + topic

	_, err := em.client.Delete(em.ctx, key)
	if err != nil {
		return fmt.Errorf("删除Topic副本信息失败: %w", err)
	}

	utils.Debugf("删除Topic副本信息: %s", topic)
	return nil
}

// GetAllOperations 获取所有操作日志
func (em *EtcdManager) GetAllOperations() ([]OperationLog, error) {
	resp, err := em.client.Get(em.ctx, OperationLogPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("获取操作日志失败: %w", err)
	}

	operations := make([]OperationLog, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var operation OperationLog
		err := json.Unmarshal(kv.Value, &operation)
		if err != nil {
			utils.Warnf("解析操作日志失败: %v", err)
			continue
		}
		operations = append(operations, operation)
	}

	return operations, nil
}

// ReplayOperations 重放操作日志
func (em *EtcdManager) ReplayOperations(handler func(OperationLog) error) error {
	operations, err := em.GetAllOperations()
	if err != nil {
		return fmt.Errorf("获取操作日志失败: %w", err)
	}

	for _, operation := range operations {
		err := handler(operation)
		if err != nil {
			utils.Errorf("重放操作失败: %v, 操作: %+v", err, operation)
			return fmt.Errorf("重放操作失败: %w", err)
		}
	}

	utils.Infof("成功重放 %d 个操作", len(operations))
	return nil
}

// GetEtcdEndpoints 获取etcd端点列表
func (em *EtcdManager) GetEtcdEndpoints() []string {
	return em.client.Endpoints()
}

// RegisterBrokerHeartbeat 注册broker心跳
func (em *EtcdManager) RegisterBrokerHeartbeat(brokerID, address string, ttl int64) error {
	em.leaseMu.Lock()
	defer em.leaseMu.Unlock()

	// 创建lease
	lease, err := em.client.Grant(em.ctx, ttl)
	if err != nil {
		return fmt.Errorf("创建lease失败: %w", err)
	}

	// 准备broker信息
	brokerInfo := BrokerInfo{
		BrokerID: brokerID,
		Address:  address,
		Status:   0, // 健康状态
	}

	brokerBytes, err := json.Marshal(brokerInfo)
	if err != nil {
		return fmt.Errorf("序列化broker信息失败: %w", err)
	}

	// 使用lease存储broker信息
	key := BrokerHeartbeatPrefix + brokerID
	_, err = em.client.Put(em.ctx, key, string(brokerBytes), clientv3.WithLease(lease.ID))
	if err != nil {
		return fmt.Errorf("存储broker心跳信息失败: %w", err)
	}

	// 保存lease ID
	em.leases[brokerID] = lease.ID

	utils.Debugf("注册broker心跳: %s (%s), TTL: %d秒", brokerID, address, ttl)
	return nil
}

// RenewBrokerHeartbeat 续约broker心跳
func (em *EtcdManager) RenewBrokerHeartbeat(brokerID, address string, ttl int64) error {
	em.leaseMu.Lock()
	defer em.leaseMu.Unlock()

	leaseID, exists := em.leases[brokerID]
	if !exists {
		// 如果lease不存在，重新注册
		return em.RegisterBrokerHeartbeat(brokerID, address, ttl)
	}

	// 续约lease
	_, err := em.client.KeepAliveOnce(em.ctx, leaseID)
	if err != nil {
		// 如果续约失败，重新创建lease
		utils.Warnf("续约broker心跳失败: %s, 重新创建lease", brokerID)
		return em.RegisterBrokerHeartbeat(brokerID, address, ttl)
	}

	// 更新broker信息
	brokerInfo := BrokerInfo{
		BrokerID: brokerID,
		Address:  address,
		Status:   0, // 健康状态
	}

	brokerBytes, err := json.Marshal(brokerInfo)
	if err != nil {
		return fmt.Errorf("序列化broker信息失败: %w", err)
	}

	key := BrokerHeartbeatPrefix + brokerID
	_, err = em.client.Put(em.ctx, key, string(brokerBytes), clientv3.WithLease(leaseID))
	if err != nil {
		return fmt.Errorf("更新broker心跳信息失败: %w", err)
	}

	utils.Debugf("续约broker心跳: %s (%s)", brokerID, address)
	return nil
}

// UnregisterBrokerHeartbeat 注销broker心跳
func (em *EtcdManager) UnregisterBrokerHeartbeat(brokerID string) error {
	em.leaseMu.Lock()
	defer em.leaseMu.Unlock()

	leaseID, exists := em.leases[brokerID]
	if !exists {
		utils.Warnf("broker心跳不存在: %s", brokerID)
		return nil
	}

	// 撤销lease
	_, err := em.client.Revoke(em.ctx, leaseID)
	if err != nil {
		return fmt.Errorf("撤销lease失败: %w", err)
	}

	// 删除broker信息
	key := BrokerHeartbeatPrefix + brokerID
	_, err = em.client.Delete(em.ctx, key)
	if err != nil {
		return fmt.Errorf("删除broker心跳信息失败: %w", err)
	}

	// 从leases map中删除
	delete(em.leases, brokerID)

	utils.Debugf("注销broker心跳: %s", brokerID)
	return nil
}

// GetBrokerHeartbeats 获取所有broker心跳信息
func (em *EtcdManager) GetBrokerHeartbeats() ([]BrokerInfo, error) {
	resp, err := em.client.Get(em.ctx, BrokerHeartbeatPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("获取broker心跳信息失败: %w", err)
	}

	brokers := make([]BrokerInfo, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var broker BrokerInfo
		err := json.Unmarshal(kv.Value, &broker)
		if err != nil {
			utils.Warnf("解析broker心跳信息失败: %v", err)
			continue
		}
		brokers = append(brokers, broker)
	}

	return brokers, nil
}

// WatchBrokerHeartbeats 监听broker心跳变化
func (em *EtcdManager) WatchBrokerHeartbeats(handler func(brokerID string, eventType string, broker *BrokerInfo)) {
	watchChan := em.client.Watch(em.ctx, BrokerHeartbeatPrefix, clientv3.WithPrefix())

	go func() {
		for {
			select {
			case <-em.ctx.Done():
				return
			case watchResp := <-watchChan:
				for _, ev := range watchResp.Events {
					brokerID := string(ev.Kv.Key)[len(BrokerHeartbeatPrefix):]

					var broker *BrokerInfo
					var eventType string

					switch ev.Type {
					case clientv3.EventTypePut:
						eventType = "PUT"
						var brokerInfo BrokerInfo
						if err := json.Unmarshal(ev.Kv.Value, &brokerInfo); err == nil {
							broker = &brokerInfo
						}
					case clientv3.EventTypeDelete:
						eventType = "DELETE"
					}

					if handler != nil {
						handler(brokerID, eventType, broker)
					}
				}
			}
		}
	}()
}
