package raft

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/ailkaya/goulette/internal/utils"
	pb "github.com/ailkaya/goulette/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TopicRaftCluster Topic的Raft集群
type TopicRaftCluster struct {
	topic      string
	raftNode   *RaftNode
	peers      map[string]string // peerID -> address
	role       pb.TopicRole
	leaderAddr string
	followers  []string
	mu         sync.RWMutex
}

// RaftManager Raft管理器
type RaftManager struct {
	brokerID    string
	brokerAddr  string
	clusters    map[string]*TopicRaftCluster // topic -> cluster
	mu          sync.RWMutex
	grpcClients map[string]pb.BrokerServiceClient // peerID -> client
	clientMu    sync.RWMutex
	fragmentMgr interface{} // FragmentManager接口，用于写入消息
}

// NewRaftManager 创建新的Raft管理器
func NewRaftManager(brokerID, brokerAddr string) *RaftManager {
	return &RaftManager{
		brokerID:    brokerID,
		brokerAddr:  brokerAddr,
		clusters:    make(map[string]*TopicRaftCluster),
		grpcClients: make(map[string]pb.BrokerServiceClient),
	}
}

// SetFragmentManager 设置Fragment管理器
func (rm *RaftManager) SetFragmentManager(fragmentMgr interface{}) {
	rm.fragmentMgr = fragmentMgr
}

// SetTopicRole 设置Topic角色
func (rm *RaftManager) SetTopicRole(topic string, role pb.TopicRole, followers []string, leaderAddr string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	utils.Infof("设置Topic %s 角色: %v, leader: %s, followers: %v", topic, role, leaderAddr, followers)

	// 如果集群已存在，先停止
	if cluster, exists := rm.clusters[topic]; exists {
		cluster.raftNode.Stop()
	}

	// 创建peers映射
	peers := make(map[string]string)
	if role == pb.TopicRole_TOPIC_ROLE_LEADER {
		// Leader需要知道所有follower的地址
		for i, followerAddr := range followers {
			peerID := fmt.Sprintf("follower_%d", i)
			peers[peerID] = followerAddr
		}
	} else if role == pb.TopicRole_TOPIC_ROLE_FOLLOWER {
		// Follower需要知道leader的地址
		peers["leader"] = leaderAddr
	}

	// 创建Raft节点
	raftNode := NewRaftNode(
		rm.brokerID,
		peers,
		rm.createApplyFunc(topic),
		rm.createSendFunc(topic),
	)

	// 创建集群
	cluster := &TopicRaftCluster{
		topic:      topic,
		raftNode:   raftNode,
		peers:      peers,
		role:       role,
		leaderAddr: leaderAddr,
		followers:  followers,
	}

	rm.clusters[topic] = cluster

	// 启动Raft节点
	raftNode.Start()

	return nil
}

// RemoveTopic 移除Topic
func (rm *RaftManager) RemoveTopic(topic string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if cluster, exists := rm.clusters[topic]; exists {
		cluster.raftNode.Stop()
		delete(rm.clusters, topic)
		utils.Infof("移除Topic Raft集群: %s", topic)
	}

	return nil
}

// ProposeMessage 提议消息
func (rm *RaftManager) ProposeMessage(topic string, messageID uint64, payload []byte) error {
	rm.mu.RLock()
	cluster, exists := rm.clusters[topic]
	rm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("Topic %s 的Raft集群不存在", topic)
	}

	// 检查是否是leader
	if cluster.role != pb.TopicRole_TOPIC_ROLE_LEADER {
		return fmt.Errorf("只有leader可以提议消息")
	}

	// 创建消息命令
	command := &MessageCommand{
		Topic:     topic,
		MessageID: messageID,
		Payload:   payload,
		Timestamp: time.Now().Unix(),
	}

	return cluster.raftNode.Propose(command)
}

// IsLeader 检查是否是leader
func (rm *RaftManager) IsLeader(topic string) bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	cluster, exists := rm.clusters[topic]
	if !exists {
		return false
	}

	return cluster.role == pb.TopicRole_TOPIC_ROLE_LEADER
}

// IsFollower 检查是否是follower
func (rm *RaftManager) IsFollower(topic string) bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	cluster, exists := rm.clusters[topic]
	if !exists {
		return false
	}

	return cluster.role == pb.TopicRole_TOPIC_ROLE_FOLLOWER
}

// GetTopicRole 获取Topic角色
func (rm *RaftManager) GetTopicRole(topic string) pb.TopicRole {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	cluster, exists := rm.clusters[topic]
	if !exists {
		return pb.TopicRole_TOPIC_ROLE_UNSPECIFIED
	}

	return cluster.role
}

// HandleRaftMessage 处理Raft消息
func (rm *RaftManager) HandleRaftMessage(topic string, message interface{}) error {
	rm.mu.RLock()
	cluster, exists := rm.clusters[topic]
	rm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("Topic %s 的Raft集群不存在", topic)
	}

	switch msg := message.(type) {
	case *RequestVoteRequest:
		response := cluster.raftNode.HandleRequestVote(msg)
		// 发送投票响应
		if err := rm.sendRaftResponse(topic, "voter", response); err != nil {
			utils.Errorf("发送投票响应失败: %v", err)
		}
		utils.Debugf("处理投票请求: topic=%s, term=%d", topic, msg.Term)
		return nil

	case *RequestVoteResponse:
		cluster.raftNode.HandleRequestVoteResponse("peer", msg)
		return nil

	case *AppendEntriesRequest:
		response := cluster.raftNode.HandleAppendEntries(msg)
		// 发送追加日志响应
		if err := rm.sendRaftResponse(topic, "appender", response); err != nil {
			utils.Errorf("发送追加日志响应失败: %v", err)
		}
		utils.Debugf("处理追加日志请求: topic=%s, term=%d", topic, msg.Term)
		return nil

	case *AppendEntriesResponse:
		cluster.raftNode.HandleAppendEntriesResponse("peer", msg)
		return nil

	default:
		return fmt.Errorf("未知的Raft消息类型: %T", message)
	}
}

// sendRaftResponse 发送Raft响应
func (rm *RaftManager) sendRaftResponse(topic, peerID string, response interface{}) error {
	rm.mu.RLock()
	cluster, exists := rm.clusters[topic]
	rm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("Topic %s 的Raft集群不存在", topic)
	}

	// 获取peer地址
	peerAddr, exists := cluster.peers[peerID]
	if !exists {
		return fmt.Errorf("Peer %s 不存在", peerID)
	}

	// 获取或创建gRPC客户端
	client, err := rm.getOrCreateClient(peerAddr)
	if err != nil {
		return fmt.Errorf("创建gRPC客户端失败: %w", err)
	}

	// 序列化响应
	payload, messageType, err := rm.serializeRaftMessage(response)
	if err != nil {
		return fmt.Errorf("序列化Raft响应失败: %w", err)
	}

	// 创建Raft请求
	raftRequest := &pb.RaftRequest{
		Topic:       topic,
		FromPeerId:  rm.brokerID,
		MessageType: messageType,
		Payload:     payload,
	}

	// 发送Raft响应
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.HandleRaftRequest(ctx, raftRequest)
	if err != nil {
		return fmt.Errorf("发送Raft响应失败: %w", err)
	}

	utils.Debugf("发送Raft响应到 %s: %T", peerAddr, response)
	return nil
}

// createApplyFunc 创建应用函数
func (rm *RaftManager) createApplyFunc(topic string) func(interface{}) error {
	return func(command interface{}) error {
		if msgCmd, ok := command.(*MessageCommand); ok {
			utils.Debugf("应用消息命令: topic=%s, messageID=%d", topic, msgCmd.MessageID)

			// 如果设置了FragmentManager，写入消息
			if rm.fragmentMgr != nil {
				// 使用反射调用WriteMessage方法
				writeMethod := reflect.ValueOf(rm.fragmentMgr).MethodByName("WriteMessage")
				if writeMethod.IsValid() {
					args := []reflect.Value{
						reflect.ValueOf(msgCmd.Topic),
						reflect.ValueOf(msgCmd.MessageID),
						reflect.ValueOf(msgCmd.Payload),
					}
					results := writeMethod.Call(args)
					if len(results) > 0 && !results[0].IsNil() {
						return results[0].Interface().(error)
					}
				}
			}
		}
		return nil
	}
}

// createSendFunc 创建发送函数
func (rm *RaftManager) createSendFunc(topic string) func(string, interface{}) error {
	return func(peerID string, message interface{}) error {
		rm.mu.RLock()
		cluster, exists := rm.clusters[topic]
		rm.mu.RUnlock()

		if !exists {
			return fmt.Errorf("Topic %s 的Raft集群不存在", topic)
		}

		// 获取peer地址
		peerAddr, exists := cluster.peers[peerID]
		if !exists {
			return fmt.Errorf("Peer %s 不存在", peerID)
		}

		// 获取或创建gRPC客户端
		client, err := rm.getOrCreateClient(peerAddr)
		if err != nil {
			return fmt.Errorf("创建gRPC客户端失败: %w", err)
		}

		// 序列化消息
		payload, messageType, err := rm.serializeRaftMessage(message)
		if err != nil {
			return fmt.Errorf("序列化Raft消息失败: %w", err)
		}

		// 创建Raft请求
		raftRequest := &pb.RaftRequest{
			Topic:       topic,
			FromPeerId:  rm.brokerID,
			MessageType: messageType,
			Payload:     payload,
		}

		// 发送Raft消息
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = client.HandleRaftRequest(ctx, raftRequest)
		if err != nil {
			return fmt.Errorf("发送Raft消息失败: %w", err)
		}

		utils.Debugf("发送Raft消息到 %s: %T", peerAddr, message)
		return nil
	}
}

// getOrCreateClient 获取或创建gRPC客户端
func (rm *RaftManager) getOrCreateClient(addr string) (pb.BrokerServiceClient, error) {
	rm.clientMu.RLock()
	if client, exists := rm.grpcClients[addr]; exists {
		rm.clientMu.RUnlock()
		return client, nil
	}
	rm.clientMu.RUnlock()

	rm.clientMu.Lock()
	defer rm.clientMu.Unlock()

	// 双重检查
	if client, exists := rm.grpcClients[addr]; exists {
		return client, nil
	}

	// 创建连接
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("连接失败: %w", err)
	}

	client := pb.NewBrokerServiceClient(conn)
	rm.grpcClients[addr] = client

	return client, nil
}

// Close 关闭Raft管理器
func (rm *RaftManager) Close() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for topic, cluster := range rm.clusters {
		cluster.raftNode.Stop()
		utils.Infof("停止Topic Raft集群: %s", topic)
	}

	rm.clusters = make(map[string]*TopicRaftCluster)
}

// serializeRaftMessage 序列化Raft消息
func (rm *RaftManager) serializeRaftMessage(message interface{}) ([]byte, pb.RaftMessageType, error) {
	switch msg := message.(type) {
	case *RequestVoteRequest:
		payload, err := json.Marshal(msg)
		if err != nil {
			return nil, pb.RaftMessageType_RAFT_MESSAGE_TYPE_UNSPECIFIED, err
		}
		return payload, pb.RaftMessageType_RAFT_MESSAGE_TYPE_REQUEST_VOTE, nil

	case *RequestVoteResponse:
		payload, err := json.Marshal(msg)
		if err != nil {
			return nil, pb.RaftMessageType_RAFT_MESSAGE_TYPE_UNSPECIFIED, err
		}
		return payload, pb.RaftMessageType_RAFT_MESSAGE_TYPE_REQUEST_VOTE_RESPONSE, nil

	case *AppendEntriesRequest:
		payload, err := json.Marshal(msg)
		if err != nil {
			return nil, pb.RaftMessageType_RAFT_MESSAGE_TYPE_UNSPECIFIED, err
		}
		return payload, pb.RaftMessageType_RAFT_MESSAGE_TYPE_APPEND_ENTRIES, nil

	case *AppendEntriesResponse:
		payload, err := json.Marshal(msg)
		if err != nil {
			return nil, pb.RaftMessageType_RAFT_MESSAGE_TYPE_UNSPECIFIED, err
		}
		return payload, pb.RaftMessageType_RAFT_MESSAGE_TYPE_APPEND_ENTRIES_RESPONSE, nil

	case *MessageCommand:
		payload, err := json.Marshal(msg)
		if err != nil {
			return nil, pb.RaftMessageType_RAFT_MESSAGE_TYPE_UNSPECIFIED, err
		}
		return payload, pb.RaftMessageType_RAFT_MESSAGE_TYPE_COMMAND, nil

	default:
		return nil, pb.RaftMessageType_RAFT_MESSAGE_TYPE_UNSPECIFIED, fmt.Errorf("未知的Raft消息类型: %T", message)
	}
}

// deserializeRaftMessage 反序列化Raft消息
func (rm *RaftManager) deserializeRaftMessage(messageType pb.RaftMessageType, payload []byte) (interface{}, error) {
	switch messageType {
	case pb.RaftMessageType_RAFT_MESSAGE_TYPE_REQUEST_VOTE:
		var msg RequestVoteRequest
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil, err
		}
		return &msg, nil

	case pb.RaftMessageType_RAFT_MESSAGE_TYPE_REQUEST_VOTE_RESPONSE:
		var msg RequestVoteResponse
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil, err
		}
		return &msg, nil

	case pb.RaftMessageType_RAFT_MESSAGE_TYPE_APPEND_ENTRIES:
		var msg AppendEntriesRequest
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil, err
		}
		return &msg, nil

	case pb.RaftMessageType_RAFT_MESSAGE_TYPE_APPEND_ENTRIES_RESPONSE:
		var msg AppendEntriesResponse
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil, err
		}
		return &msg, nil

	case pb.RaftMessageType_RAFT_MESSAGE_TYPE_COMMAND:
		var msg MessageCommand
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil, err
		}
		return &msg, nil

	default:
		return nil, fmt.Errorf("未知的Raft消息类型: %v", messageType)
	}
}

// MessageCommand 消息命令
type MessageCommand struct {
	Topic     string
	MessageID uint64
	Payload   []byte
	Timestamp int64
}

// HandleRaftRequest 处理来自其他节点的Raft请求
func (rm *RaftManager) HandleRaftRequest(request *pb.RaftRequest) (*pb.RaftResponse, error) {
	// 反序列化Raft消息
	message, err := rm.deserializeRaftMessage(request.MessageType, request.Payload)
	if err != nil {
		return &pb.RaftResponse{
			Topic:        request.Topic,
			ToPeerId:     request.FromPeerId,
			MessageType:  request.MessageType,
			Success:      false,
			ErrorMessage: fmt.Sprintf("反序列化失败: %v", err),
		}, nil
	}

	// 处理Raft消息
	err = rm.HandleRaftMessage(request.Topic, message)
	if err != nil {
		return &pb.RaftResponse{
			Topic:        request.Topic,
			ToPeerId:     request.FromPeerId,
			MessageType:  request.MessageType,
			Success:      false,
			ErrorMessage: fmt.Sprintf("处理Raft消息失败: %v", err),
		}, nil
	}

	return &pb.RaftResponse{
		Topic:       request.Topic,
		ToPeerId:    request.FromPeerId,
		MessageType: request.MessageType,
		Success:     true,
	}, nil
}
