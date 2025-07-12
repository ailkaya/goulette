package sentinel

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ailkaya/goulette/internal/etcd"
	"github.com/ailkaya/goulette/internal/utils"
	pb "github.com/ailkaya/goulette/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

// BrokerInfo Broker信息
type BrokerInfo struct {
	BrokerID      string
	Address       string
	Status        BrokerStatus
	Topics        []string
	LastHeartbeat time.Time
	mu            sync.RWMutex
}

// BrokerStatus Broker状态
type BrokerStatus int

const (
	BrokerStatusHealthy BrokerStatus = iota
	BrokerStatusSuspicious
	BrokerStatusDown
)

// ReplicationCache 副本缓存
type ReplicationCache struct {
	Brokers      []*BrokerInfo
	Version      int64
	ReplicaCount int         // 记录副本数量要求
	Leader       *BrokerInfo // 记录当前topic replication的leader
}

// TopicQueryLock Topic查询锁
type TopicQueryLock struct {
	mu      sync.Mutex
	version int64
}

// SentinelServer Sentinel服务器
type SentinelServer struct {
	pb.UnimplementedSentinelServiceServer
	sentinelID    string
	address       string
	grpcServer    *grpc.Server
	hashRing      *utils.HashRing
	brokers       map[string]*BrokerInfo
	mu            sync.RWMutex
	stopChan      chan struct{}
	wg            sync.WaitGroup
	healthChecker *HealthChecker

	// 副本缓存相关
	replicationCache map[string]*ReplicationCache // topic -> replication cache
	cacheMu          sync.RWMutex

	// Topic查询锁
	topicLocks   map[string]*TopicQueryLock
	topicLocksMu sync.RWMutex

	// etcd管理器
	etcdManager *etcd.EtcdManager
}

// HealthChecker 健康检查器
type HealthChecker struct {
	brokers       map[string]*BrokerInfo
	mu            sync.RWMutex
	checkInterval time.Duration
	stopChan      chan struct{}
	wg            sync.WaitGroup
	sentinel      *SentinelServer // 添加对SentinelServer的引用
}

// NewSentinelServer 创建新的Sentinel服务器
func NewSentinelServer(sentinelID, address string, etcdEndpoints []string) *SentinelServer {
	// 初始化日志系统
	if !utils.IsInitialized() {
		logPath := fmt.Sprintf("logs/sentinel_%s.log", sentinelID)
		if err := utils.InitLogger("info", logPath); err != nil {
			// 如果文件日志初始化失败，使用控制台日志
			utils.InitLogger("info", "")
		}
	}

	server := &SentinelServer{
		sentinelID:       sentinelID,
		address:          address,
		hashRing:         utils.NewHashRing(150), // 每个Broker映射150个虚拟节点
		brokers:          make(map[string]*BrokerInfo),
		stopChan:         make(chan struct{}),
		replicationCache: make(map[string]*ReplicationCache),
		topicLocks:       make(map[string]*TopicQueryLock),
	}

	// 初始化etcd管理器
	if len(etcdEndpoints) > 0 {
		etcdManager, err := etcd.NewEtcdManager(etcdEndpoints)
		if err != nil {
			utils.Errorf("初始化etcd管理器失败: %v", err)
		} else {
			server.etcdManager = etcdManager
			utils.Infof("etcd管理器初始化成功，端点: %v", etcdEndpoints)
		}
	}

	server.healthChecker = NewHealthChecker(server.brokers, 30*time.Second, server)
	return server
}

// Start 启动Sentinel服务器
func (server *SentinelServer) Start() error {
	lis, err := net.Listen("tcp", server.address)
	if err != nil {
		return fmt.Errorf("监听端口失败: %w", err)
	}

	server.grpcServer = grpc.NewServer()

	// 注册服务
	pb.RegisterSentinelServiceServer(server.grpcServer, server)

	// 启用反射服务（用于调试）
	reflection.Register(server.grpcServer)

	// 从etcd恢复操作
	if server.etcdManager != nil {
		utils.Info("从etcd恢复操作...")
		if err := server.RecoverFromEtcd(); err != nil {
			utils.Errorf("从etcd恢复操作失败: %v", err)
		} else {
			utils.Info("从etcd恢复操作成功")
		}

		// 启动etcd心跳监听
		utils.Info("启动etcd心跳监听...")
		server.etcdManager.WatchBrokerHeartbeats(server.handleBrokerHeartbeatChange)
		utils.Info("etcd心跳监听启动成功")

		// 从etcd恢复broker信息
		if err := server.recoverBrokersFromEtcd(); err != nil {
			utils.Errorf("从etcd恢复broker信息失败: %v", err)
		} else {
			utils.Info("从etcd恢复broker信息成功")
		}
	}

	// 启动健康检查器
	server.healthChecker.Start()

	utils.Infof("Sentinel服务器启动: %s", server.address)

	// 启动gRPC服务器
	go func() {
		if err := server.grpcServer.Serve(lis); err != nil {
			utils.Errorf("gRPC服务器启动失败: %v", err)
		}
	}()

	return nil
}

// Stop 停止Sentinel服务器
func (server *SentinelServer) Stop() {
	close(server.stopChan)
	server.wg.Wait()

	if server.healthChecker != nil {
		server.healthChecker.Stop()
	}

	if server.etcdManager != nil {
		if err := server.etcdManager.Close(); err != nil {
			utils.Errorf("关闭etcd管理器失败: %v", err)
		}
	}

	if server.grpcServer != nil {
		server.grpcServer.GracefulStop()
	}

	utils.Info("Sentinel服务器已停止")
}

// KeepAlive 实现心跳服务
// 使用etcd的lease机制实现心跳服务
func (server *SentinelServer) KeepAlive(ctx context.Context, req *pb.KeepAliveRequest) (*pb.KeepAliveResponse, error) {
	brokerID := req.BrokerId
	address := req.Address

	// 如果etcd管理器可用，使用etcd lease机制
	if server.etcdManager != nil {
		// 设置TTL为30秒
		ttl := int64(30)

		// 尝试续约现有心跳，如果不存在则注册新的
		err := server.etcdManager.RenewBrokerHeartbeat(brokerID, address, ttl)
		if err != nil {
			utils.Errorf("处理broker心跳失败: %s, 错误: %v", brokerID, err)
			return &pb.KeepAliveResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("心跳处理失败: %v", err),
			}, nil
		}

		// 同时更新本地broker信息（用于兼容性）
		server.mu.Lock()
		broker, exists := server.brokers[brokerID]
		if !exists {
			// 新Broker注册
			broker = &BrokerInfo{
				BrokerID:      brokerID,
				Address:       address,
				Status:        BrokerStatusHealthy,
				LastHeartbeat: time.Now(),
			}
			server.brokers[brokerID] = broker

			// 添加到哈希环
			server.hashRing.AddNode(brokerID)

			utils.Infof("新Broker注册: %s (%s)", brokerID, address)
		} else {
			// 更新现有Broker信息
			broker.mu.Lock()
			broker.Address = address
			broker.Status = BrokerStatusHealthy
			broker.LastHeartbeat = time.Now()
			broker.mu.Unlock()
		}
		server.mu.Unlock()

		utils.Debugf("Broker心跳续约成功: %s (%s)", brokerID, address)
		return &pb.KeepAliveResponse{
			Success:      true,
			ErrorMessage: "心跳续约成功",
		}, nil
	}

	// 如果etcd不可用，回退到原来的实现
	server.mu.Lock()
	defer server.mu.Unlock()

	broker, exists := server.brokers[brokerID]

	if !exists {
		// 新Broker注册
		broker = &BrokerInfo{
			BrokerID:      brokerID,
			Address:       address,
			Status:        BrokerStatusHealthy,
			LastHeartbeat: time.Now(),
		}
		server.brokers[brokerID] = broker

		// 添加到哈希环
		server.hashRing.AddNode(brokerID)

		utils.Infof("新Broker注册: %s (%s)", brokerID, address)
	} else {
		// 更新现有Broker信息
		broker.mu.Lock()
		broker.Address = address
		broker.Status = BrokerStatusHealthy
		broker.LastHeartbeat = time.Now()
		broker.mu.Unlock()
	}

	return &pb.KeepAliveResponse{
		Success:      true,
		ErrorMessage: "心跳更新成功",
	}, nil
}

// getTopicLock 获取指定topic的查询锁
func (server *SentinelServer) getTopicLock(topic string) *TopicQueryLock {
	server.topicLocksMu.Lock()
	defer server.topicLocksMu.Unlock()

	lock, exists := server.topicLocks[topic]
	if !exists {
		lock = &TopicQueryLock{}
		server.topicLocks[topic] = lock
	}
	return lock
}

// updateBrokerStatus 更新broker状态
func (server *SentinelServer) updateBrokerStatus(brokerIDs []string, status BrokerStatus) {
	server.mu.Lock()
	defer server.mu.Unlock()

	for _, brokerID := range brokerIDs {
		if broker, exists := server.brokers[brokerID]; exists {
			broker.mu.Lock()
			broker.Status = status
			broker.mu.Unlock()
			utils.Warnf("更新Broker状态: %s -> %v", brokerID, status)
		}
	}
}

// findReplacementBrokers 寻找替换broker
func (server *SentinelServer) findReplacementBrokers(topic string, currentBrokers []*BrokerInfo, neededCount int) ([]*BrokerInfo, error) {
	if neededCount <= 0 {
		return []*BrokerInfo{}, nil
	}

	// 检查可用副本数是否小于n/2+1
	if len(currentBrokers) < (neededCount/2 + 1) {
		utils.Warnf("Topic %s 可用副本数 %d 小于安全阈值 %d", topic, len(currentBrokers), neededCount/2+1)
	}

	// 创建当前broker的ID集合，用于去重
	currentBrokerIDs := make(map[string]bool)
	for _, broker := range currentBrokers {
		currentBrokerIDs[broker.BrokerID] = true
	}

	// 从哈希环开始寻找新的broker
	server.mu.RLock()
	iter := server.hashRing.GetNodeIter(topic)
	server.mu.RUnlock()

	replacementBrokers := make([]*BrokerInfo, 0, neededCount)
	foundCount := 0

	for foundCount < neededCount {
		brokerID, err := iter.Next()
		if err != nil {
			// 已经遍历完所有节点
			break
		}
		if brokerID == "" {
			break
		}

		// 检查是否已经在当前副本中
		if currentBrokerIDs[brokerID] {
			continue
		}

		// 检查broker是否存在且健康
		server.mu.RLock()
		broker, exists := server.brokers[brokerID]
		server.mu.RUnlock()

		if exists {
			broker.mu.RLock()
			if broker.Status == BrokerStatusHealthy {
				replacementBrokers = append(replacementBrokers, broker)
				foundCount++
			}
			broker.mu.RUnlock()
		}
	}

	if foundCount < neededCount {
		return replacementBrokers, fmt.Errorf("无法找到足够的健康broker来补充副本，需要 %d 个，只找到 %d 个", neededCount, foundCount)
	}

	return replacementBrokers, nil
}

// invalidateCache 使指定topic的缓存失效
func (server *SentinelServer) invalidateCache(topic string) {
	server.cacheMu.Lock()
	defer server.cacheMu.Unlock()
	delete(server.replicationCache, topic)
}

// GetTopicLeader 实现获取Topic Leader服务
func (server *SentinelServer) GetTopicLeader(ctx context.Context, req *pb.GetTopicLeaderRequest) (*pb.GetTopicLeaderResponse, error) {
	topic := req.Topic
	replicaCount := int(req.ReplicaCount)
	unreachableBrokers := req.UnreachableBrokers

	// 如果没有etcd管理器，使用原有逻辑
	if server.etcdManager == nil {
		return server.getTopicLeaderLegacy(ctx, req)
	}

	// 如果unreachable_brokers为空，直接从etcd获取
	if len(unreachableBrokers) == 0 {
		return server.getTopicLeaderFromEtcd(topic, replicaCount)
	}

	// 如果有unreachable_brokers，检查是否为leader
	if server.etcdManager.IsLeader() {
		// 当前节点是leader，直接处理
		return server.handleUnreachableBrokersForLeader(topic, replicaCount, unreachableBrokers)
	} else {
		// 当前节点不是leader，转发到leader
		return server.forwardToLeader(ctx, req)
	}
}

// getTopicLeaderFromEtcd 从etcd获取topic leader
func (server *SentinelServer) getTopicLeaderFromEtcd(topic string, replicaCount int) (*pb.GetTopicLeaderResponse, error) {
	// 从etcd获取topic副本信息
	replicaInfo, err := server.etcdManager.GetTopicReplicas(topic)
	if err != nil {
		return nil, fmt.Errorf("从etcd获取topic副本信息失败: %w", err)
	}

	// 如果etcd中没有该topic的信息，使用哈希环生成
	if replicaInfo == nil {
		return server.generateAndStoreTopicLeader(topic, replicaCount)
	}

	// 检查版本是否过期（超过5分钟）
	if time.Now().Unix()-replicaInfo.Timestamp > 300 {
		utils.Warnf("Topic %s 副本信息已过期，重新生成", topic)
		return server.generateAndStoreTopicLeader(topic, replicaCount)
	}

	if len(replicaInfo.Brokers) == 0 {
		return nil, fmt.Errorf("topic %s 没有可用的broker", topic)
	}

	// 依次调用GetBrokerTopicRole RPC直到找到leader
	var leaderBroker *etcd.BrokerInfo
	for _, broker := range replicaInfo.Brokers {
		// 检查broker是否仍然健康
		server.mu.RLock()
		brokerInfo, exists := server.brokers[broker.BrokerID]
		server.mu.RUnlock()

		if !exists {
			utils.Warnf("Broker %s 不在本地broker列表中，跳过", broker.BrokerID)
			continue
		}

		brokerInfo.mu.RLock()
		if brokerInfo.Status != BrokerStatusHealthy {
			brokerInfo.mu.RUnlock()
			utils.Warnf("Broker %s 状态不健康，跳过", broker.BrokerID)
			continue
		}
		brokerInfo.mu.RUnlock()

		// 调用GetBrokerTopicRole RPC
		resp, err := server.callGetBrokerTopicRoleOnBroker(broker.Address, broker.BrokerID, topic)
		if err != nil {
			utils.Warnf("调用broker %s 的GetBrokerTopicRole失败: %v", broker.BrokerID, err)
			continue
		}

		// 检查是否是leader
		if resp.Role == pb.TopicRole_TOPIC_ROLE_LEADER {
			leaderBroker = &broker
			utils.Infof("找到Topic %s 的leader: %s", topic, broker.BrokerID)
			break
		}
	}

	// 如果没有找到leader，返回错误
	if leaderBroker == nil {
		return nil, fmt.Errorf("topic %s 没有找到leader", topic)
	}

	leaderInfo := &pb.BrokerInfo{
		BrokerId: leaderBroker.BrokerID,
		Address:  leaderBroker.Address,
		Status:   pb.BrokerStatus(leaderBroker.Status),
		LastHeartbeat: &pb.Timestamp{
			Seconds: replicaInfo.Timestamp,
		},
	}

	return &pb.GetTopicLeaderResponse{
		Leader: leaderInfo,
	}, nil
}

// generateAndStoreTopicLeader 生成并存储topic leader信息
func (server *SentinelServer) generateAndStoreTopicLeader(topic string, replicaCount int) (*pb.GetTopicLeaderResponse, error) {
	// 使用哈希环生成broker列表
	server.mu.RLock()
	iter := server.hashRing.GetNodeIter(topic)
	server.mu.RUnlock()

	brokers := make([]*BrokerInfo, 0, replicaCount)
	etcdBrokers := make([]etcd.BrokerInfo, 0, replicaCount)

	for i := 0; i < replicaCount; i++ {
		brokerID, err := iter.Next()
		if brokerID == "" || err != nil {
			break
		}

		server.mu.RLock()
		broker, exists := server.brokers[brokerID]
		server.mu.RUnlock()

		if exists {
			broker.mu.RLock()
			if broker.Status == BrokerStatusHealthy {
				brokers = append(brokers, broker)
				etcdBrokers = append(etcdBrokers, etcd.BrokerInfo{
					BrokerID: broker.BrokerID,
					Address:  broker.Address,
					Status:   int(broker.Status),
				})
			}
			broker.mu.RUnlock()
		}
	}

	// 存储到etcd
	if err := server.etcdManager.SetTopicReplicas(topic, etcdBrokers, replicaCount); err != nil {
		utils.Errorf("存储topic副本信息到etcd失败: %v", err)
	}

	// 记录操作日志
	server.etcdManager.LogOperation("generateTopicReplicas", map[string]interface{}{
		"topic":         topic,
		"replica_count": replicaCount,
		"brokers":       etcdBrokers,
	})

	// 选择第一个broker作为leader
	if len(brokers) == 0 {
		return nil, fmt.Errorf("topic %s 没有可用的broker", topic)
	}

	leader := brokers[0]
	leader.mu.RLock()
	leaderInfo := &pb.BrokerInfo{
		BrokerId: leader.BrokerID,
		Address:  leader.Address,
		Status:   pb.BrokerStatus(leader.Status),
		LastHeartbeat: &pb.Timestamp{
			Seconds: leader.LastHeartbeat.Unix(),
		},
	}
	leader.mu.RUnlock()

	// 通知broker设置角色
	if err := server.notifyBrokersSetTopicRole(topic, brokers, leader); err != nil {
		utils.Errorf("通知broker设置Topic角色失败: %v", err)
	}

	return &pb.GetTopicLeaderResponse{
		Leader: leaderInfo,
	}, nil
}

// handleUnreachableBrokersForLeader 处理无法连接的broker并返回leader
func (server *SentinelServer) handleUnreachableBrokersForLeader(topic string, replicaCount int, unreachableBrokers []string) (*pb.GetTopicLeaderResponse, error) {
	// 先尝试调用无法连接broker的RemoveTopic RPC接口
	for _, brokerID := range unreachableBrokers {
		server.mu.RLock()
		brokerInfo, exists := server.brokers[brokerID]
		server.mu.RUnlock()

		if exists {
			// 尝试调用RemoveTopic
			if err := server.callRemoveTopicOnBroker(brokerInfo.Address, topic, fmt.Sprintf("Broker %s 无法连接", brokerID)); err != nil {
				utils.Warnf("调用broker %s 的RemoveTopic失败: %v", brokerID, err)
			} else {
				utils.Infof("成功调用broker %s 的RemoveTopic，移除topic: %s", brokerID, topic)
			}
		}
	}

	// 更新无法连接broker的状态
	server.updateBrokerStatus(unreachableBrokers, BrokerStatusDown)

	// 从etcd获取当前副本信息
	replicaInfo, err := server.etcdManager.GetTopicReplicas(topic)
	if err != nil {
		return nil, fmt.Errorf("获取topic副本信息失败: %w", err)
	}

	var currentBrokers []*BrokerInfo
	if replicaInfo != nil {
		// 从副本信息中移除无法连接的broker
		availableBrokers := make([]*BrokerInfo, 0, len(replicaInfo.Brokers))
		unreachableSet := make(map[string]bool)
		for _, id := range unreachableBrokers {
			unreachableSet[id] = true
		}

		for _, broker := range replicaInfo.Brokers {
			if !unreachableSet[broker.BrokerID] {
				// 检查broker是否仍然健康
				server.mu.RLock()
				brokerInfo, exists := server.brokers[broker.BrokerID]
				server.mu.RUnlock()

				if exists {
					brokerInfo.mu.RLock()
					if brokerInfo.Status == BrokerStatusHealthy {
						availableBrokers = append(availableBrokers, brokerInfo)
					}
					brokerInfo.mu.RUnlock()
				}
			}
		}
		currentBrokers = availableBrokers
	}

	// 计算需要补充的副本数量
	neededCount := replicaCount - len(currentBrokers)
	if neededCount > 0 {
		// 寻找替换broker
		replacementBrokers, err := server.findReplacementBrokers(topic, currentBrokers, neededCount)
		if err != nil {
			return nil, fmt.Errorf("寻找替换broker失败: %w", err)
		}

		// 合并可用broker和替换broker
		currentBrokers = append(currentBrokers, replacementBrokers...)
	}

	if len(currentBrokers) != replicaCount {
		return nil, fmt.Errorf("topic %s 可用broker不足", topic)
	}

	// 更新etcd中的副本信息
	etcdBrokers := make([]etcd.BrokerInfo, 0, len(currentBrokers))
	for _, broker := range currentBrokers {
		broker.mu.RLock()
		etcdBrokers = append(etcdBrokers, etcd.BrokerInfo{
			BrokerID: broker.BrokerID,
			Address:  broker.Address,
			Status:   int(broker.Status),
		})
		broker.mu.RUnlock()
	}

	if err := server.etcdManager.SetTopicReplicas(topic, etcdBrokers, replicaCount); err != nil {
		return nil, fmt.Errorf("更新topic副本信息到etcd失败: %w", err)
	}

	// 记录操作日志
	server.etcdManager.LogOperation("handleUnreachableBrokers", map[string]interface{}{
		"topic":               topic,
		"replica_count":       replicaCount,
		"unreachable_brokers": unreachableBrokers,
		"new_brokers":         etcdBrokers,
	})

	// 选择leader（通过rpc查找）
	leader, err := server.findLeaderBrokerFromBrokers(currentBrokers, topic)
	if err != nil {
		return nil, err
	}
	leader.mu.RLock()
	leaderInfo := &pb.BrokerInfo{
		BrokerId: leader.BrokerID,
		Address:  leader.Address,
		Status:   pb.BrokerStatus(leader.Status),
		LastHeartbeat: &pb.Timestamp{
			Seconds: leader.LastHeartbeat.Unix(),
		},
	}
	leader.mu.RUnlock()

	// 通知broker设置角色
	if err := server.notifyBrokersSetTopicRole(topic, currentBrokers, leader); err != nil {
		utils.Errorf("通知broker设置Topic角色失败: %v", err)
	}

	return &pb.GetTopicLeaderResponse{
		Leader: leaderInfo,
	}, nil
}

// forwardToLeader 转发请求到leader
func (server *SentinelServer) forwardToLeader(ctx context.Context, req *pb.GetTopicLeaderRequest) (*pb.GetTopicLeaderResponse, error) {
	leaderAddr := server.etcdManager.GetLeaderAddr()
	if leaderAddr == "" {
		return nil, fmt.Errorf("无法获取leader地址")
	}

	// 创建到leader的连接
	conn, err := grpc.NewClient(leaderAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("连接leader失败: %w", err)
	}
	defer conn.Close()

	client := pb.NewSentinelServiceClient(conn)
	return client.GetTopicLeader(ctx, req)
}

// getTopicLeaderLegacy 使用原有逻辑获取topic leader
func (server *SentinelServer) getTopicLeaderLegacy(ctx context.Context, req *pb.GetTopicLeaderRequest) (*pb.GetTopicLeaderResponse, error) {
	topic := req.Topic
	replicaCount := int(req.ReplicaCount)
	unreachableBrokers := req.UnreachableBrokers

	// 获取topic查询锁
	topicLock := server.getTopicLock(topic)
	topicLock.mu.Lock()
	defer topicLock.mu.Unlock()

	// 检查缓存
	server.cacheMu.RLock()
	cache, exists := server.replicationCache[topic]
	server.cacheMu.RUnlock()

	var currentBrokers []*BrokerInfo
	if exists && cache.ReplicaCount == replicaCount {
		// 缓存有效，使用缓存的broker
		currentBrokers = cache.Brokers
	} else {
		// 缓存无效或副本数量不匹配，重新查询哈希环
		server.mu.RLock()
		iter := server.hashRing.GetNodeIter(topic)
		server.mu.RUnlock()

		// 收集健康的broker
		currentBrokers = make([]*BrokerInfo, 0, replicaCount)
		for i := 0; i < replicaCount; i++ {
			brokerID, err := iter.Next()
			if brokerID == "" || err != nil {
				break
			}

			server.mu.RLock()
			broker, exists := server.brokers[brokerID]
			server.mu.RUnlock()

			if exists {
				broker.mu.RLock()
				if broker.Status == BrokerStatusHealthy {
					currentBrokers = append(currentBrokers, broker)
				}
				broker.mu.RUnlock()
			}
		}

		// 更新缓存
		topicLock.version++
		server.cacheMu.Lock()
		var leader *BrokerInfo
		if len(currentBrokers) > 0 {
			leader = currentBrokers[0]
		}
		server.replicationCache[topic] = &ReplicationCache{
			Brokers:      currentBrokers,
			Version:      topicLock.version,
			ReplicaCount: replicaCount,
			Leader:       leader,
		}
		server.cacheMu.Unlock()
	}

	// 如果有无法连接的broker，寻找替换broker
	if len(unreachableBrokers) > 0 {
		// 更新无法连接broker的状态
		server.updateBrokerStatus(unreachableBrokers, BrokerStatusDown)

		// 从当前broker中移除无法连接的broker
		availableBrokers := make([]*BrokerInfo, 0, len(currentBrokers))
		unreachableSet := make(map[string]bool)
		for _, id := range unreachableBrokers {
			unreachableSet[id] = true
		}

		for _, broker := range currentBrokers {
			if !unreachableSet[broker.BrokerID] {
				availableBrokers = append(availableBrokers, broker)
			}
		}

		// 计算需要补充的副本数量
		neededCount := replicaCount - len(availableBrokers)
		if neededCount > 0 {
			// 寻找替换broker
			replacementBrokers, err := server.findReplacementBrokers(topic, availableBrokers, neededCount)
			if err != nil {
				return nil, fmt.Errorf("寻找替换broker失败: %w", err)
			}

			// 合并可用broker和替换broker
			currentBrokers = append(availableBrokers, replacementBrokers...)

			// 更新缓存
			topicLock.version++
			server.cacheMu.Lock()
			var leader *BrokerInfo
			if len(currentBrokers) > 0 {
				leader = currentBrokers[0]
			}
			server.replicationCache[topic] = &ReplicationCache{
				Brokers:      currentBrokers,
				Version:      topicLock.version,
				ReplicaCount: replicaCount,
				Leader:       leader,
			}
			server.cacheMu.Unlock()
		} else {
			currentBrokers = availableBrokers
		}
	}

	if len(currentBrokers) == 0 {
		return nil, fmt.Errorf("topic %s 没有可用的broker", topic)
	}

	// 选择leader（通过rpc查找）
	leader, err := server.findLeaderBrokerFromBrokers(currentBrokers, topic)
	if err != nil {
		return nil, err
	}
	leader.mu.RLock()
	leaderInfo := &pb.BrokerInfo{
		BrokerId: leader.BrokerID,
		Address:  leader.Address,
		Status:   pb.BrokerStatus(leader.Status),
		Topics:   leader.Topics,
		LastHeartbeat: &pb.Timestamp{
			Seconds: leader.LastHeartbeat.Unix(),
		},
	}
	leader.mu.RUnlock()

	// 通知broker设置角色
	if err := server.notifyBrokersSetTopicRole(topic, currentBrokers, leader); err != nil {
		utils.Errorf("通知broker设置Topic角色失败: %v", err)
	}

	return &pb.GetTopicLeaderResponse{
		Leader: leaderInfo,
	}, nil
}

// RegisterBroker 实现Broker注册服务
func (server *SentinelServer) RegisterBroker(ctx context.Context, req *pb.RegisterBrokerRequest) (*pb.RegisterBrokerResponse, error) {
	server.mu.Lock()
	defer server.mu.Unlock()

	brokerID := req.BrokerId

	// 检查Broker是否已存在
	if _, exists := server.brokers[brokerID]; exists {
		return &pb.RegisterBrokerResponse{
			Success:      false,
			ErrorMessage: "Broker已存在",
		}, nil
	}

	// 创建新Broker信息
	broker := &BrokerInfo{
		BrokerID:      brokerID,
		Address:       req.Address,
		Status:        BrokerStatusHealthy,
		Topics:        req.Topics,
		LastHeartbeat: time.Now(),
	}
	server.brokers[brokerID] = broker

	// 添加到哈希环
	server.hashRing.AddNode(brokerID)

	// 记录操作日志
	if server.etcdManager != nil {
		server.etcdManager.LogOperation("registerBroker", map[string]interface{}{
			"broker_id": brokerID,
			"address":   req.Address,
			"topics":    req.Topics,
		})
	}

	// 清除相关topic的缓存
	for _, topic := range req.Topics {
		server.invalidateCache(topic)
	}

	utils.Infof("Broker注册成功: %s (%s)", brokerID, req.Address)

	return &pb.RegisterBrokerResponse{
		Success: true,
	}, nil
}

// UnregisterBroker 实现Broker下线
// TODO: 要下线的broker寻找到一个或多个继承人之后才能下线
func (server *SentinelServer) UnregisterBroker(ctx context.Context, req *pb.UnregisterBrokerRequest) (*pb.UnregisterBrokerResponse, error) {
	brokerID := req.BrokerId

	// 如果etcd管理器可用，使用etcd lease机制注销
	if server.etcdManager != nil {
		// 注销etcd中的broker心跳
		if err := server.etcdManager.UnregisterBrokerHeartbeat(brokerID); err != nil {
			utils.Errorf("注销broker心跳失败: %s, 错误: %v", brokerID, err)
			return &pb.UnregisterBrokerResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("注销broker心跳失败: %v", err),
			}, nil
		}

		// 同时更新本地broker信息
		server.mu.Lock()
		broker, exists := server.brokers[brokerID]
		if !exists {
			server.mu.Unlock()
			return &pb.UnregisterBrokerResponse{
				Success:      false,
				ErrorMessage: "Broker不存在",
			}, nil
		}

		// 从哈希环移除
		server.hashRing.RemoveNode(brokerID)

		// 清除相关topic的缓存
		broker.mu.RLock()
		topics := make([]string, len(broker.Topics))
		copy(topics, broker.Topics)
		broker.mu.RUnlock()

		for _, topic := range topics {
			server.invalidateCache(topic)
		}

		// 从brokers映射中移除
		delete(server.brokers, brokerID)
		server.mu.Unlock()

		utils.Infof("Broker下线: %s", brokerID)
		return &pb.UnregisterBrokerResponse{
			Success: true,
		}, nil
	}

	// 如果etcd不可用，使用原来的实现
	server.mu.Lock()
	defer server.mu.Unlock()

	// 检查Broker是否存在
	broker, exists := server.brokers[brokerID]
	if !exists {
		return &pb.UnregisterBrokerResponse{
			Success:      false,
			ErrorMessage: "Broker不存在",
		}, nil
	}

	// 从哈希环移除
	server.hashRing.RemoveNode(brokerID)

	// 记录操作日志
	if server.etcdManager != nil {
		broker.mu.RLock()
		topics := make([]string, len(broker.Topics))
		copy(topics, broker.Topics)
		broker.mu.RUnlock()

		server.etcdManager.LogOperation("unregisterBroker", map[string]interface{}{
			"broker_id": brokerID,
			"topics":    topics,
		})
	}

	// 清除相关topic的缓存
	broker.mu.RLock()
	topics := make([]string, len(broker.Topics))
	copy(topics, broker.Topics)
	broker.mu.RUnlock()

	for _, topic := range topics {
		server.invalidateCache(topic)
	}

	// 从brokers映射中移除
	delete(server.brokers, brokerID)

	utils.Infof("Broker下线: %s", brokerID)

	return &pb.UnregisterBrokerResponse{
		Success: true,
	}, nil
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(brokers map[string]*BrokerInfo, checkInterval time.Duration, sentinel *SentinelServer) *HealthChecker {
	return &HealthChecker{
		brokers:       brokers,
		checkInterval: checkInterval,
		stopChan:      make(chan struct{}),
		sentinel:      sentinel,
	}
}

// Start 启动健康检查器
func (hc *HealthChecker) Start() {
	hc.wg.Add(1)
	go hc.checkRoutine()
}

// Stop 停止健康检查器
func (hc *HealthChecker) Stop() {
	close(hc.stopChan)
	hc.wg.Wait()
}

// checkRoutine 健康检查协程
func (hc *HealthChecker) checkRoutine() {
	defer hc.wg.Done()

	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.checkBrokers()
		case <-hc.stopChan:
			return
		}
	}
}

// checkBrokers 检查所有Broker的健康状态
func (hc *HealthChecker) checkBrokers() {
	hc.mu.RLock()
	brokers := make(map[string]*BrokerInfo)
	for id, broker := range hc.brokers {
		brokers[id] = broker
	}
	hc.mu.RUnlock()

	now := time.Now()
	topicsToInvalidate := make(map[string]bool)

	for brokerID, broker := range brokers {
		broker.mu.Lock()
		oldStatus := broker.Status

		// 检查心跳超时
		timeSinceLastHeartbeat := now.Sub(broker.LastHeartbeat)

		switch broker.Status {
		case BrokerStatusHealthy:
			if timeSinceLastHeartbeat > 60*time.Second {
				broker.Status = BrokerStatusSuspicious
				utils.Warnf("Broker状态变为可疑: %s", brokerID)
			}
		case BrokerStatusSuspicious:
			if timeSinceLastHeartbeat > 120*time.Second {
				broker.Status = BrokerStatusDown
				utils.Errorf("Broker状态变为宕机: %s", brokerID)
			} else if timeSinceLastHeartbeat <= 30*time.Second {
				broker.Status = BrokerStatusHealthy
				utils.Infof("Broker状态恢复健康: %s", brokerID)
			}
		case BrokerStatusDown:
			// 宕机状态，等待手动处理或自动清理
			if timeSinceLastHeartbeat > 300*time.Second { // 5分钟
				utils.Infof("清理宕机Broker: %s", brokerID)
			}
		}

		// 如果状态发生变化，记录需要清除缓存的topics
		if oldStatus != broker.Status {
			for _, topic := range broker.Topics {
				topicsToInvalidate[topic] = true
			}
		}

		broker.mu.Unlock()
	}

	// 清除相关topic的缓存
	if hc.sentinel != nil {
		for topic := range topicsToInvalidate {
			hc.sentinel.invalidateCache(topic)
		}
	}
}

// GetSentinelInfo 获取Sentinel信息
func (server *SentinelServer) GetSentinelInfo() map[string]interface{} {
	server.mu.RLock()
	defer server.mu.RUnlock()

	brokerInfos := make([]map[string]interface{}, 0, len(server.brokers))
	for _, broker := range server.brokers {
		broker.mu.RLock()
		brokerInfo := map[string]interface{}{
			"broker_id":      broker.BrokerID,
			"address":        broker.Address,
			"status":         broker.Status,
			"topics":         broker.Topics,
			"last_heartbeat": broker.LastHeartbeat,
		}
		broker.mu.RUnlock()
		brokerInfos = append(brokerInfos, brokerInfo)
	}

	return map[string]interface{}{
		"sentinel_id":  server.sentinelID,
		"address":      server.address,
		"brokers":      brokerInfos,
		"broker_count": len(server.brokers),
	}
}

// RecoverFromEtcd 从etcd恢复操作
func (server *SentinelServer) RecoverFromEtcd() error {
	if server.etcdManager == nil {
		return fmt.Errorf("etcd管理器未初始化")
	}

	return server.etcdManager.ReplayOperations(func(operation etcd.OperationLog) error {
		return server.replayOperation(operation)
	})
}

// replayOperation 重放单个操作
func (server *SentinelServer) replayOperation(operation etcd.OperationLog) error {
	switch operation.Function {
	case "addBrokerToHashRing":
		return server.replayAddBrokerToHashRing(operation.Params)
	case "registerBroker":
		return server.replayRegisterBroker(operation.Params)
	case "unregisterBroker":
		return server.replayUnregisterBroker(operation.Params)
	case "generateTopicReplicas":
		return server.replayGenerateTopicReplicas(operation.Params)
	case "handleUnreachableBrokers":
		return server.replayHandleUnreachableBrokers(operation.Params)
	default:
		utils.Warnf("未知的操作类型: %s", operation.Function)
		return nil
	}
}

// replayAddBrokerToHashRing 重放添加broker到哈希环操作
func (server *SentinelServer) replayAddBrokerToHashRing(params string) error {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(params), &data); err != nil {
		return fmt.Errorf("解析参数失败: %w", err)
	}

	brokerID := data["broker_id"].(string)
	server.hashRing.AddNode(brokerID)
	utils.Debugf("重放操作: 添加broker到哈希环: %s", brokerID)
	return nil
}

// replayRegisterBroker 重放注册broker操作
func (server *SentinelServer) replayRegisterBroker(params string) error {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(params), &data); err != nil {
		return fmt.Errorf("解析参数失败: %w", err)
	}

	brokerID := data["broker_id"].(string)
	address := data["address"].(string)
	topicsInterface := data["topics"].([]interface{})

	topics := make([]string, len(topicsInterface))
	for i, topic := range topicsInterface {
		topics[i] = topic.(string)
	}

	broker := &BrokerInfo{
		BrokerID:      brokerID,
		Address:       address,
		Status:        BrokerStatusHealthy,
		Topics:        topics,
		LastHeartbeat: time.Now(),
	}

	server.mu.Lock()
	server.brokers[brokerID] = broker
	server.hashRing.AddNode(brokerID)
	server.mu.Unlock()

	utils.Debugf("重放操作: 注册broker: %s", brokerID)
	return nil
}

// replayUnregisterBroker 重放下线broker操作
func (server *SentinelServer) replayUnregisterBroker(params string) error {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(params), &data); err != nil {
		return fmt.Errorf("解析参数失败: %w", err)
	}

	brokerID := data["broker_id"].(string)

	server.mu.Lock()
	server.hashRing.RemoveNode(brokerID)
	delete(server.brokers, brokerID)
	server.mu.Unlock()

	utils.Debugf("重放操作: 下线broker: %s", brokerID)
	return nil
}

// replayGenerateTopicReplicas 重放生成topic副本操作
func (server *SentinelServer) replayGenerateTopicReplicas(params string) error {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(params), &data); err != nil {
		return fmt.Errorf("解析参数失败: %w", err)
	}

	topic := data["topic"].(string)
	replicaCount := int(data["replica_count"].(float64))
	brokersInterface := data["brokers"].([]interface{})

	etcdBrokers := make([]etcd.BrokerInfo, len(brokersInterface))
	for i, brokerInterface := range brokersInterface {
		brokerData := brokerInterface.(map[string]interface{})
		etcdBrokers[i] = etcd.BrokerInfo{
			BrokerID: brokerData["broker_id"].(string),
			Address:  brokerData["address"].(string),
			Status:   int(brokerData["status"].(float64)),
		}
	}

	// 存储到etcd
	if err := server.etcdManager.SetTopicReplicas(topic, etcdBrokers, replicaCount); err != nil {
		return fmt.Errorf("存储topic副本信息失败: %w", err)
	}

	utils.Debugf("重放操作: 生成topic副本: %s", topic)
	return nil
}

// replayHandleUnreachableBrokers 重放处理无法连接broker操作
func (server *SentinelServer) replayHandleUnreachableBrokers(params string) error {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(params), &data); err != nil {
		return fmt.Errorf("解析参数失败: %w", err)
	}

	topic := data["topic"].(string)
	replicaCount := int(data["replica_count"].(float64))
	unreachableBrokersInterface := data["unreachable_brokers"].([]interface{})
	newBrokersInterface := data["new_brokers"].([]interface{})

	// 更新无法连接broker的状态
	unreachableBrokers := make([]string, len(unreachableBrokersInterface))
	for i, broker := range unreachableBrokersInterface {
		unreachableBrokers[i] = broker.(string)
	}
	server.updateBrokerStatus(unreachableBrokers, BrokerStatusDown)

	// 更新etcd中的副本信息
	etcdBrokers := make([]etcd.BrokerInfo, len(newBrokersInterface))
	for i, brokerInterface := range newBrokersInterface {
		brokerData := brokerInterface.(map[string]interface{})
		etcdBrokers[i] = etcd.BrokerInfo{
			BrokerID: brokerData["broker_id"].(string),
			Address:  brokerData["address"].(string),
			Status:   int(brokerData["status"].(float64)),
		}
	}

	if err := server.etcdManager.SetTopicReplicas(topic, etcdBrokers, replicaCount); err != nil {
		return fmt.Errorf("更新topic副本信息失败: %w", err)
	}

	utils.Debugf("重放操作: 处理无法连接broker: %s", topic)
	return nil
}

// handleBrokerHeartbeatChange 处理broker心跳变化
func (server *SentinelServer) handleBrokerHeartbeatChange(brokerID string, eventType string, broker *etcd.BrokerInfo) {
	switch eventType {
	case "PUT":
		// broker上线或更新
		if broker != nil {
			server.mu.Lock()
			localBroker, exists := server.brokers[brokerID]
			if !exists {
				// 新broker上线
				localBroker = &BrokerInfo{
					BrokerID:      brokerID,
					Address:       broker.Address,
					Status:        BrokerStatusHealthy,
					LastHeartbeat: time.Now(),
				}
				server.brokers[brokerID] = localBroker
				server.hashRing.AddNode(brokerID)
				utils.Infof("从etcd检测到新broker上线: %s (%s)", brokerID, broker.Address)
			} else {
				// 更新现有broker信息
				localBroker.mu.Lock()
				localBroker.Address = broker.Address
				localBroker.Status = BrokerStatusHealthy
				localBroker.LastHeartbeat = time.Now()
				localBroker.mu.Unlock()
				utils.Debugf("从etcd更新broker信息: %s (%s)", brokerID, broker.Address)
			}
			server.mu.Unlock()
		}
	case "DELETE":
		// broker下线
		server.mu.Lock()
		if _, exists := server.brokers[brokerID]; exists {
			server.hashRing.RemoveNode(brokerID)
			delete(server.brokers, brokerID)
			utils.Warnf("从etcd检测到broker下线: %s", brokerID)
		}
		server.mu.Unlock()
	}
}

// recoverBrokersFromEtcd 从etcd恢复broker信息
func (server *SentinelServer) recoverBrokersFromEtcd() error {
	if server.etcdManager == nil {
		return fmt.Errorf("etcd管理器未初始化")
	}

	brokers, err := server.etcdManager.GetBrokerHeartbeats()
	if err != nil {
		return fmt.Errorf("获取broker心跳信息失败: %w", err)
	}

	server.mu.Lock()
	defer server.mu.Unlock()

	for _, etcdBroker := range brokers {
		broker := &BrokerInfo{
			BrokerID:      etcdBroker.BrokerID,
			Address:       etcdBroker.Address,
			Status:        BrokerStatusHealthy,
			LastHeartbeat: time.Now(),
		}
		server.brokers[etcdBroker.BrokerID] = broker
		server.hashRing.AddNode(etcdBroker.BrokerID)
		utils.Debugf("从etcd恢复broker: %s (%s)", etcdBroker.BrokerID, etcdBroker.Address)
	}

	utils.Infof("从etcd恢复了 %d 个broker", len(brokers))
	return nil
}

// callRemoveTopicOnBroker 调用指定broker的RemoveTopic RPC接口
func (server *SentinelServer) callRemoveTopicOnBroker(brokerAddress, topic, reason string) error {
	// 创建到broker的连接
	conn, err := grpc.Dial(brokerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("连接broker失败: %w", err)
	}
	defer conn.Close()

	// 创建broker客户端
	client := pb.NewBrokerServiceClient(conn)

	// 设置超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 调用RemoveTopic
	req := &pb.RemoveTopicRequest{
		Topic:  topic,
		Reason: reason,
	}

	resp, err := client.RemoveTopic(ctx, req)
	if err != nil {
		return fmt.Errorf("调用RemoveTopic失败: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("RemoveTopic返回失败: %s", resp.ErrorMessage)
	}

	return nil
}

// notifyBrokersSetTopicRole 通知broker设置Topic角色
func (server *SentinelServer) notifyBrokersSetTopicRole(topic string, brokers []*BrokerInfo, leader *BrokerInfo) error {
	// 准备follower地址列表
	followerAddresses := make([]string, 0, len(brokers)-1)
	for _, broker := range brokers {
		if broker.BrokerID != leader.BrokerID {
			followerAddresses = append(followerAddresses, broker.Address)
		}
	}

	// 通知leader
	if err := server.callSetTopicRoleOnBroker(leader.Address, topic, pb.TopicRole_TOPIC_ROLE_LEADER, followerAddresses, leader.Address); err != nil {
		utils.Errorf("通知leader broker %s 设置Topic角色失败: %v", leader.BrokerID, err)
		return err
	}

	// 通知followers
	for _, broker := range brokers {
		if broker.BrokerID != leader.BrokerID {
			if err := server.callSetTopicRoleOnBroker(broker.Address, topic, pb.TopicRole_TOPIC_ROLE_FOLLOWER, followerAddresses, leader.Address); err != nil {
				utils.Errorf("通知follower broker %s 设置Topic角色失败: %v", broker.BrokerID, err)
				// 不返回错误，继续通知其他broker
			}
		}
	}

	utils.Infof("成功通知所有broker设置Topic %s 角色", topic)
	return nil
}

// callSetTopicRoleOnBroker 调用broker的SetTopicRole RPC
func (server *SentinelServer) callSetTopicRoleOnBroker(brokerAddr, topic string, role pb.TopicRole, followers []string, leaderAddr string) error {
	// 创建到broker的连接
	conn, err := grpc.Dial(brokerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("连接broker失败: %w", err)
	}
	defer conn.Close()

	client := pb.NewBrokerServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 调用SetTopicRole RPC
	req := &pb.SetTopicRoleRequest{
		Topic:         topic,
		Role:          role,
		Followers:     followers,
		LeaderAddress: leaderAddr,
	}

	resp, err := client.SetTopicRole(ctx, req)
	if err != nil {
		return fmt.Errorf("调用SetTopicRole RPC失败: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("SetTopicRole RPC返回失败: %s", resp.ErrorMessage)
	}

	utils.Debugf("成功通知broker %s 设置Topic %s 为 %v", brokerAddr, topic, role)
	return nil
}

// callGetBrokerTopicRoleOnBroker 调用broker的GetBrokerTopicRole RPC
func (server *SentinelServer) callGetBrokerTopicRoleOnBroker(brokerAddr, brokerID, topic string) (*pb.GetBrokerTopicRoleResponse, error) {
	// 创建到broker的连接
	conn, err := grpc.Dial(brokerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("连接broker失败: %w", err)
	}
	defer conn.Close()

	client := pb.NewBrokerServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 调用GetBrokerTopicRole RPC
	req := &pb.GetBrokerTopicRoleRequest{
		BrokerId: brokerID,
		Topic:    topic,
	}

	resp, err := client.GetBrokerTopicRole(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用GetBrokerTopicRole RPC失败: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("GetBrokerTopicRole RPC返回失败: %s", resp.ErrorMessage)
	}

	utils.Debugf("成功获取broker %s 在Topic %s 中的角色: %v", brokerID, topic, resp.Role)
	return resp, nil
}

// findLeaderBrokerFromBrokers 遍历brokers调用GetBrokerTopicRole，返回leader
func (server *SentinelServer) findLeaderBrokerFromBrokers(brokers []*BrokerInfo, topic string) (*BrokerInfo, error) {
	for _, broker := range brokers {
		// 检查broker是否健康
		broker.mu.RLock()
		isHealthy := broker.Status == BrokerStatusHealthy
		broker.mu.RUnlock()
		if !isHealthy {
			continue
		}
		resp, err := server.callGetBrokerTopicRoleOnBroker(broker.Address, broker.BrokerID, topic)
		if err != nil {
			utils.Warnf("调用broker %s 的GetBrokerTopicRole失败: %v", broker.BrokerID, err)
			continue
		}
		if resp.Role == pb.TopicRole_TOPIC_ROLE_LEADER {
			return broker, nil
		}
	}
	return nil, fmt.Errorf("topic %s 没有找到leader", topic)
}

// FindAndAssignTopicSuccessor 为某 broker 找到 topic 的继承人并指挥数据迁移
func (server *SentinelServer) FindAndAssignTopicSuccessor(brokerID, topic string) error {
	// 1. 获取当前 topic 的所有健康副本 broker（不包括下线的和自己）
	server.mu.RLock()
	var currentBrokers []*BrokerInfo
	for _, b := range server.brokers {
		if b.Status == BrokerStatusHealthy && b.BrokerID != brokerID {
			for _, t := range b.Topics {
				if t == topic {
					currentBrokers = append(currentBrokers, b)
					break
				}
			}
		}
	}
	server.mu.RUnlock()

	// 2. 选择继承人（不在当前副本列表且健康的 broker）
	successors, err := server.findReplacementBrokers(topic, currentBrokers, 1)
	if err != nil || len(successors) == 0 {
		return fmt.Errorf("未找到可用继承人: %v", err)
	}
	successor := successors[0]

	// 3. 指挥所有现有副本 broker 复制 fragment 到继承人
	for _, peer := range currentBrokers {
		err := server.callRequestFragmentReplication(peer.Address, topic, successor.BrokerID)
		if err != nil {
			utils.Warnf("指挥 broker %s 复制 fragment 到 %s 失败: %v", peer.BrokerID, successor.BrokerID, err)
		} else {
			utils.Infof("已指挥 broker %s 复制 fragment 到 %s", peer.BrokerID, successor.BrokerID)
		}
	}

	// 4. 可选：等待复制完成后，更新副本信息（此处可扩展）
	// TODO: 可实现回调或轮询确认复制完成

	return nil
}

// callRequestFragmentReplication 调用 broker 的 ReplicateFragment RPC
func (server *SentinelServer) callRequestFragmentReplication(brokerAddr, topic, targetBrokerID string) error {
	// 获取目标 broker 的地址
	server.mu.RLock()
	targetBroker, exists := server.brokers[targetBrokerID]
	server.mu.RUnlock()
	if !exists {
		return fmt.Errorf("目标 broker %s 不存在", targetBrokerID)
	}

	// 建立gRPC连接
	conn, err := grpc.Dial(brokerAddr, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("连接源 broker 失败: %w", err)
	}
	defer conn.Close()

	client := pb.NewBrokerServiceClient(conn)

	// 获取当前副本数量
	server.mu.RLock()
	var currentReplicas uint64 = 0
	for _, b := range server.brokers {
		if b.Status == BrokerStatusHealthy {
			for _, t := range b.Topics {
				if t == topic {
					currentReplicas++
					break
				}
			}
		}
	}
	server.mu.RUnlock()

	// 发送复制请求
	req := &pb.RequestFragmentReplicationRequest{
		Topic:               topic,
		TargetBrokerId:      targetBrokerID,
		TargetBrokerAddress: targetBroker.Address,
		ReplicaIndex:        currentReplicas - 1, // 新副本的索引
		TotalReplicas:       currentReplicas,     // 总副本数
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.RequestFragmentReplication(ctx, req)
	if err != nil {
		return fmt.Errorf("请求复制 fragment 失败: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("复制请求被拒绝: %s", resp.ErrorMessage)
	}

	return nil
}
