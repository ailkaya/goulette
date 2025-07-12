package broker

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/ailkaya/goulette/internal/raft"
	"github.com/ailkaya/goulette/internal/storage"
	"github.com/ailkaya/goulette/internal/utils"
	pb "github.com/ailkaya/goulette/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// BrokerServer Broker服务器
type BrokerServer struct {
	pb.UnimplementedBrokerServiceServer
	brokerID       string
	address        string
	grpcServer     *grpc.Server
	fragmentMgr    *storage.FragmentManager
	wal            *storage.WAL
	raftManager    *raft.RaftManager
	messageCounter uint64
	mu             sync.RWMutex
	topics         map[string]bool
	topicRoles     map[string]pb.TopicRole // topic -> role
	stopChan       chan struct{}
	wg             sync.WaitGroup
}

// NewBrokerServer 创建新的Broker服务器
func NewBrokerServer(brokerID, address, dataDir string) (*BrokerServer, error) {
	// 初始化日志系统
	if !utils.IsInitialized() {
		logPath := fmt.Sprintf("logs/broker_%s.log", brokerID)
		if err := utils.InitLogger("info", logPath); err != nil {
			// 如果文件日志初始化失败，使用控制台日志
			utils.InitLogger("info", "")
		}
	}

	// 创建Fragment管理器
	fragmentMgr := storage.NewFragmentManager(
		dataDir+"/fragments",
		128*1024*1024,  // 128MB max fragment size
		10000,          // 10000 messages max per fragment
		7*24*time.Hour, // 7天TTL
	)

	// 创建WAL
	wal, err := storage.NewWAL(
		dataDir+"/wal",
		128*1024*1024, // 128MB max WAL file size
		2*time.Second, // 2秒刷盘间隔
	)
	if err != nil {
		return nil, fmt.Errorf("创建WAL失败: %w", err)
	}

	// 创建Raft管理器
	raftManager := raft.NewRaftManager(brokerID, address)
	raftManager.SetFragmentManager(fragmentMgr)

	server := &BrokerServer{
		brokerID:    brokerID,
		address:     address,
		fragmentMgr: fragmentMgr,
		wal:         wal,
		raftManager: raftManager,
		topics:      make(map[string]bool),
		topicRoles:  make(map[string]pb.TopicRole),
		stopChan:    make(chan struct{}),
	}

	// 重放WAL
	if err := server.replayWAL(); err != nil {
		utils.Warnf("重放WAL失败: %v", err)
	}

	return server, nil
}

// replayWAL 重放WAL文件
func (server *BrokerServer) replayWAL() error {
	return server.wal.Replay(func(entry *storage.WALEntry) error {
		if entry.Type == 1 { // 消息类型
			// 写入Fragment
			if err := server.fragmentMgr.WriteMessage(entry.Topic, entry.MessageID, entry.Payload); err != nil {
				return fmt.Errorf("重放消息到Fragment失败: %w", err)
			}

			// 更新消息计数器
			server.mu.Lock()
			if entry.MessageID > server.messageCounter {
				server.messageCounter = entry.MessageID
			}
			server.topics[entry.Topic] = true
			// 从raftManager获取topic角色
			server.topicRoles[entry.Topic] = server.raftManager.GetTopicRole(entry.Topic)
			server.mu.Unlock()
		} else if entry.Type == 2 { // 移除Topic类型
			// 从topics映射中移除
			server.mu.Lock()
			delete(server.topics, entry.Topic)
			delete(server.topicRoles, entry.Topic)
			server.mu.Unlock()

			// 关闭该Topic相关的Fragment
			if err := server.fragmentMgr.CloseTopic(entry.Topic); err != nil {
				utils.Warnf("重放时关闭Topic Fragment失败: %v", err)
			}

			utils.Infof("重放时移除Topic: %s, 原因: %s", entry.Topic, string(entry.Payload))
		}
		return nil
	})
}

// Start 启动Broker服务器
func (server *BrokerServer) Start() error {
	lis, err := net.Listen("tcp", server.address)
	if err != nil {
		return fmt.Errorf("监听端口失败: %w", err)
	}

	server.grpcServer = grpc.NewServer()

	// 注册服务
	pb.RegisterBrokerServiceServer(server.grpcServer, server)

	// 启用反射服务（用于调试）
	reflection.Register(server.grpcServer)

	utils.Infof("Broker服务器启动: %s", server.address)

	// 启动gRPC服务器
	go func() {
		if err := server.grpcServer.Serve(lis); err != nil {
			utils.Errorf("gRPC服务器启动失败: %v", err)
		}
	}()

	return nil
}

// Stop 停止Broker服务器
func (server *BrokerServer) Stop() {
	close(server.stopChan)
	server.wg.Wait()

	if server.grpcServer != nil {
		server.grpcServer.GracefulStop()
	}

	if server.fragmentMgr != nil {
		server.fragmentMgr.Close()
	}

	if server.wal != nil {
		server.wal.Close()
	}

	if server.raftManager != nil {
		server.raftManager.Close()
	}

	utils.Info("Broker服务器已停止")
}

// SendMessage 实现发送消息服务
func (server *BrokerServer) SendMessage(stream pb.BrokerService_SendMessageServer) error {
	ctx := stream.Context()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-server.stopChan:
			return nil
		default:
			// 接收消息请求
			req, err := stream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					return nil
				}
				return fmt.Errorf("接收消息失败: %w", err)
			}

			// 处理消息
			response, err := server.processMessage(req)
			if err != nil {
				utils.Errorf("处理消息失败: %v", err)
				response = &pb.MessageResponse{
					MessageId:    req.MessageId,
					Status:       pb.ResponseStatus_RESPONSE_STATUS_ERROR,
					ErrorMessage: err.Error(),
				}
			}

			// 发送响应
			if err := stream.Send(response); err != nil {
				return fmt.Errorf("发送响应失败: %w", err)
			}
		}
	}
}

// processMessage 处理消息
func (server *BrokerServer) processMessage(req *pb.MessageRequest) (*pb.MessageResponse, error) {
	server.mu.Lock()
	defer server.mu.Unlock()

	// 检查Topic是否已被移除
	if !server.topics[req.Topic] {
		return &pb.MessageResponse{
			MessageId:    req.MessageId,
			Status:       pb.ResponseStatus_RESPONSE_STATUS_RETRY,
			ErrorMessage: fmt.Sprintf("Topic已被移除: %s", req.Topic),
		}, nil
	}

	// 检查broker是否是此topic的leader
	if !server.raftManager.IsLeader(req.Topic) {
		return &pb.MessageResponse{
			MessageId:    req.MessageId,
			Status:       pb.ResponseStatus_RESPONSE_STATUS_RETRY,
			ErrorMessage: fmt.Sprintf("当前broker不是Topic %s 的leader", req.Topic),
		}, nil
	}

	// 生成消息ID
	server.messageCounter++
	messageID := server.messageCounter

	// 记录Topic
	server.topics[req.Topic] = true
	// 从raftManager获取topic角色
	server.topicRoles[req.Topic] = server.raftManager.GetTopicRole(req.Topic)

	// 写入WAL
	if err := server.wal.WriteMessage(req.Topic, messageID, req.Payload); err != nil {
		return nil, fmt.Errorf("写入WAL失败: %w", err)
	}

	// 使用raft进行消息同步
	if err := server.raftManager.ProposeMessage(req.Topic, messageID, req.Payload); err != nil {
		utils.Errorf("Raft消息同步失败: %v", err)
		return &pb.MessageResponse{
			MessageId:    req.MessageId,
			Status:       pb.ResponseStatus_RESPONSE_STATUS_RETRY,
			ErrorMessage: fmt.Sprintf("Raft消息同步失败: %v", err),
		}, nil
	}

	// Leader直接写入Fragment（follower通过raft同步写入）
	if err := server.fragmentMgr.WriteMessage(req.Topic, messageID, req.Payload); err != nil {
		return nil, fmt.Errorf("写入Fragment失败: %w", err)
	}

	utils.Debugf("处理消息: topic=%s, id=%d, size=%d", req.Topic, messageID, len(req.Payload))

	return &pb.MessageResponse{
		MessageId: messageID,
		Status:    pb.ResponseStatus_RESPONSE_STATUS_SUCCESS,
	}, nil
}

// PullMessage 实现拉取消息服务
func (server *BrokerServer) PullMessage(req *pb.PullRequest, stream pb.BrokerService_PullMessageServer) error {
	ctx := stream.Context()

	// 检查Topic是否存在
	server.mu.RLock()
	if !server.topics[req.Topic] {
		server.mu.RUnlock()
		return fmt.Errorf("Topic已被移除: %s", req.Topic)
	}
	server.mu.RUnlock()

	// 检查broker是否是此topic的leader或follower
	role := server.raftManager.GetTopicRole(req.Topic)
	if role != pb.TopicRole_TOPIC_ROLE_LEADER && role != pb.TopicRole_TOPIC_ROLE_FOLLOWER {
		return fmt.Errorf("当前broker不是Topic %s 的leader或follower", req.Topic)
	}

	// 读取消息
	messages, err := server.fragmentMgr.ReadMessages(req.Topic, req.Offset, int(req.BatchSize))
	if err != nil {
		return fmt.Errorf("读取消息失败: %w", err)
	}

	// 发送消息
	for _, msg := range messages {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-server.stopChan:
			return nil
		default:
			message := &pb.Message{
				MessageId: msg.MessageID,
				Topic:     req.Topic,
				Payload:   msg.Payload,
				Offset:    req.Offset,
				Timestamp: &pb.Timestamp{
					Seconds: msg.Timestamp,
				},
				Type: pb.MessageType_MESSAGE_TYPE_NORMAL,
			}

			if err := stream.Send(message); err != nil {
				return fmt.Errorf("发送消息失败: %w", err)
			}

			req.Offset++
		}
	}

	return nil
}

// ReplicateFragment 实现Fragment复制服务
func (server *BrokerServer) ReplicateFragment(stream pb.BrokerService_ReplicateFragmentServer) error {
	// 接收第一个分片以获取元数据
	firstChunk, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("接收首个Fragment分片失败: %w", err)
	}

	// 获取目标topic和fragment的存储路径
	topic := firstChunk.Topic
	fragmentID := firstChunk.FragmentId
	targetPath := server.fragmentMgr.GetFragmentPath(topic, fragmentID)

	// 创建一个缓冲区来存储所有数据
	var buffer bytes.Buffer

	// 写入第一个分片
	if _, err := buffer.Write(firstChunk.Data); err != nil {
		return fmt.Errorf("写入Fragment分片失败: %w", err)
	}

	// 接收并写入剩余分片
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("接收Fragment分片失败: %w", err)
		}

		// 写入分片数据
		if _, err := buffer.Write(chunk.Data); err != nil {
			return fmt.Errorf("写入Fragment分片失败: %w", err)
		}

		utils.Debugf("接收Fragment分片: topic=%s, fragment=%d, chunk=%d",
			chunk.Topic, chunk.FragmentId, chunk.ChunkIndex)

		if chunk.IsLast {
			break
		}
	}

	// 将数据写入文件
	if err := server.fragmentMgr.ReplicateFragmentToFile(topic, fragmentID, buffer.Bytes(), targetPath); err != nil {
		return fmt.Errorf("保存Fragment文件失败: %w", err)
	}

	response := &pb.ReplicateResponse{
		Success: true,
	}

	return stream.SendAndClose(response)
}

// SendFragmentToTarget 将指定的Fragment发送到目标Broker
func (server *BrokerServer) SendFragmentToTarget(ctx context.Context, targetAddr string, topic string, fragmentID uint64) error {
	// 建立gRPC连接
	conn, err := grpc.Dial(targetAddr, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("连接目标Broker失败: %w", err)
	}
	defer conn.Close()

	client := pb.NewBrokerServiceClient(conn)

	// 创建Fragment复制流
	stream, err := client.ReplicateFragment(ctx)
	if err != nil {
		return fmt.Errorf("创建Fragment复制流失败: %w", err)
	}

	// 读取Fragment文件内容
	data, err := server.fragmentMgr.ReadFragmentFile(topic, fragmentID)
	if err != nil {
		return fmt.Errorf("读取Fragment文件失败: %w", err)
	}

	// 分片大小设置为1MB
	const chunkSize = 1 * 1024 * 1024
	var chunkIndex uint32 = 0

	// 分片发送数据
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}

		// 计算分片校验和
		checksum := md5.Sum(data[i:end])

		// 创建并发送分片
		chunk := &pb.FragmentChunk{
			Topic:      topic,
			FragmentId: fragmentID,
			ChunkIndex: chunkIndex,
			Data:       data[i:end],
			IsLast:     end == len(data),
			Checksum:   checksum[:],
		}

		if err := stream.Send(chunk); err != nil {
			return fmt.Errorf("发送Fragment分片失败: %w", err)
		}

		chunkIndex++
	}

	// 等待响应
	response, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("接收复制响应失败: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("Fragment复制失败")
	}

	return nil
}

// ReplicateFragments 复制指定topic下符合条件的Fragments到目标Broker
func (server *BrokerServer) ReplicateFragments(ctx context.Context, targetAddr string, topic string, idx uint64, n uint64) error {
	// 获取需要复制的fragments
	fragments := server.fragmentMgr.SelectFragmentsForReplication(topic, idx, n)

	// 遍历fragments，复制符合条件的fragment
	for _, fragment := range fragments {
		// 检查fragment是否已经被完全消费
		if server.fragmentMgr.IsFragmentFullyConsumed(topic, fragment.ID) {
			utils.Debugf("跳过已完全消费的Fragment: topic=%s, id=%d", topic, fragment.ID)
			continue
		}

		// 发送fragment到目标broker
		if err := server.SendFragmentToTarget(ctx, targetAddr, topic, fragment.ID); err != nil {
			return fmt.Errorf("复制Fragment失败 [topic=%s, id=%d]: %w", topic, fragment.ID, err)
		}

		utils.Infof("成功复制Fragment: topic=%s, id=%d 到目标Broker: %s", topic, fragment.ID, targetAddr)
	}

	return nil
}

// GetBrokerInfo 获取Broker信息
func (server *BrokerServer) GetBrokerInfo() map[string]interface{} {
	server.mu.RLock()
	defer server.mu.RUnlock()

	topics := make([]string, 0, len(server.topics))
	topicRoles := make(map[string]string)
	for topic := range server.topics {
		topics = append(topics, topic)
		role := server.topicRoles[topic]
		switch role {
		case pb.TopicRole_TOPIC_ROLE_LEADER:
			topicRoles[topic] = "leader"
		case pb.TopicRole_TOPIC_ROLE_FOLLOWER:
			topicRoles[topic] = "follower"
		default:
			topicRoles[topic] = "unknown"
		}
	}

	return map[string]interface{}{
		"broker_id":       server.brokerID,
		"address":         server.address,
		"topics":          topics,
		"topic_roles":     topicRoles,
		"message_counter": server.messageCounter,
	}
}

// GetTopicRole 获取指定Topic的角色
func (server *BrokerServer) GetTopicRole(topic string) pb.TopicRole {
	server.mu.RLock()
	defer server.mu.RUnlock()

	if role, exists := server.topicRoles[topic]; exists {
		return role
	}
	return pb.TopicRole_TOPIC_ROLE_UNSPECIFIED
}

// RemoveTopic 实现移除Topic服务
func (server *BrokerServer) RemoveTopic(ctx context.Context, req *pb.RemoveTopicRequest) (*pb.RemoveTopicResponse, error) {
	server.mu.Lock()
	defer server.mu.Unlock()

	// 检查Topic是否存在
	if !server.topics[req.Topic] {
		return &pb.RemoveTopicResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Topic不存在: %s", req.Topic),
		}, nil
	}

	// 记录移除操作到WAL
	if err := server.wal.WriteRemoveTopic(req.Topic, req.Reason); err != nil {
		utils.Errorf("记录移除Topic到WAL失败: %v", err)
		return &pb.RemoveTopicResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("记录移除Topic失败: %v", err),
		}, nil
	}

	// 从topics映射中移除
	delete(server.topics, req.Topic)
	delete(server.topicRoles, req.Topic)

	// 关闭该Topic相关的Fragment
	if err := server.fragmentMgr.CloseTopic(req.Topic); err != nil {
		utils.Errorf("关闭Topic Fragment失败: %v", err)
		// 不返回错误，因为Topic已经从内存中移除了
	}

	// 移除Raft集群
	if err := server.raftManager.RemoveTopic(req.Topic); err != nil {
		utils.Errorf("移除Topic Raft集群失败: %v", err)
	}

	utils.Infof("Topic已移除: %s, 原因: %s", req.Topic, req.Reason)

	return &pb.RemoveTopicResponse{
		Success: true,
	}, nil
}

// SetTopicRole 实现设置Topic角色服务
func (server *BrokerServer) SetTopicRole(ctx context.Context, req *pb.SetTopicRoleRequest) (*pb.SetTopicRoleResponse, error) {
	utils.Infof("设置Topic角色: topic=%s, role=%v, leader=%s, followers=%v",
		req.Topic, req.Role, req.LeaderAddress, req.Followers)

	// 设置Raft集群角色
	if err := server.raftManager.SetTopicRole(req.Topic, req.Role, req.Followers, req.LeaderAddress); err != nil {
		utils.Errorf("设置Topic角色失败: %v", err)
		return &pb.SetTopicRoleResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("设置Topic角色失败: %v", err),
		}, nil
	}

	// 确保Topic在topics映射中并记录角色
	server.mu.Lock()
	server.topics[req.Topic] = true
	server.topicRoles[req.Topic] = req.Role
	server.mu.Unlock()

	utils.Infof("Topic角色设置成功: %s, role=%v", req.Topic, req.Role)

	return &pb.SetTopicRoleResponse{
		Success: true,
	}, nil
}

// HandleRaftRequest 实现Raft请求处理服务
func (server *BrokerServer) HandleRaftRequest(ctx context.Context, req *pb.RaftRequest) (*pb.RaftResponse, error) {
	utils.Debugf("处理Raft请求: topic=%s, from=%s, type=%v",
		req.Topic, req.FromPeerId, req.MessageType)

	// 委托给Raft管理器处理
	response, err := server.raftManager.HandleRaftRequest(req)
	if err != nil {
		utils.Errorf("处理Raft请求失败: %v", err)
		return &pb.RaftResponse{
			Topic:        req.Topic,
			ToPeerId:     req.FromPeerId,
			MessageType:  req.MessageType,
			Success:      false,
			ErrorMessage: fmt.Sprintf("处理Raft请求失败: %v", err),
		}, nil
	}

	return response, nil
}

// GetBrokerTopicRole 实现获取Broker在指定Topic中角色的服务
func (server *BrokerServer) GetBrokerTopicRole(ctx context.Context, req *pb.GetBrokerTopicRoleRequest) (*pb.GetBrokerTopicRoleResponse, error) {
	utils.Debugf("获取Broker Topic角色: broker=%s, topic=%s", req.BrokerId, req.Topic)

	// 检查请求的broker_id是否匹配当前broker
	if req.BrokerId != server.brokerID {
		return &pb.GetBrokerTopicRoleResponse{
			BrokerId:     req.BrokerId,
			Topic:        req.Topic,
			Role:         pb.TopicRole_TOPIC_ROLE_UNSPECIFIED,
			Success:      false,
			ErrorMessage: fmt.Sprintf("Broker ID不匹配: 请求=%s, 当前=%s", req.BrokerId, server.brokerID),
		}, nil
	}

	// 检查Topic是否存在
	server.mu.RLock()
	if !server.topics[req.Topic] {
		server.mu.RUnlock()
		return &pb.GetBrokerTopicRoleResponse{
			BrokerId:     req.BrokerId,
			Topic:        req.Topic,
			Role:         pb.TopicRole_TOPIC_ROLE_UNSPECIFIED,
			Success:      false,
			ErrorMessage: fmt.Sprintf("Topic不存在: %s", req.Topic),
		}, nil
	}
	server.mu.RUnlock()

	// 获取Topic角色
	role := server.raftManager.GetTopicRole(req.Topic)

	utils.Debugf("Broker %s 在Topic %s 中的角色: %v", req.BrokerId, req.Topic, role)

	return &pb.GetBrokerTopicRoleResponse{
		BrokerId: req.BrokerId,
		Topic:    req.Topic,
		Role:     role,
		Success:  true,
	}, nil
}

// RequestFragmentReplication 实现请求复制Fragment的RPC
func (server *BrokerServer) RequestFragmentReplication(ctx context.Context, req *pb.RequestFragmentReplicationRequest) (*pb.RequestFragmentReplicationResponse, error) {
	utils.Infof("收到复制Fragment请求: topic=%s, targetBroker=%s", req.GetTopic(), req.GetTargetBrokerId())

	// 启动一个新的 goroutine 来处理复制，避免阻塞 RPC 调用
	go func() {
		err := server.ReplicateFragments(context.Background(), req.GetTargetBrokerAddress(), req.GetTopic(), req.GetReplicaIndex(), req.GetTotalReplicas())
		if err != nil {
			utils.Errorf("复制Fragment失败: %v", err)
		}
	}()

	return &pb.RequestFragmentReplicationResponse{
		Success: true,
	}, nil
}
