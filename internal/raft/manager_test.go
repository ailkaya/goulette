package raft

import (
	"testing"
	"time"

	"github.com/ailkaya/goulette/internal/utils"
	pb "github.com/ailkaya/goulette/proto"
)

func TestRaftManager(t *testing.T) {
	// 初始化日志
	utils.InitLogger("debug", "")

	// 创建Raft管理器
	manager := NewRaftManager("broker1", "localhost:50051")

	// 测试设置Topic角色
	err := manager.SetTopicRole("test-topic", pb.TopicRole_TOPIC_ROLE_LEADER,
		[]string{"localhost:50052", "localhost:50053"}, "localhost:50051")
	if err != nil {
		t.Fatalf("设置Topic角色失败: %v", err)
	}

	// 检查角色设置
	if !manager.IsLeader("test-topic") {
		t.Error("期望是Leader角色")
	}

	if manager.IsFollower("test-topic") {
		t.Error("不应该是Follower角色")
	}

	// 测试提议消息
	err = manager.ProposeMessage("test-topic", 1, []byte("test message"))
	if err != nil {
		t.Fatalf("提议消息失败: %v", err)
	}

	// 测试序列化
	request := &RequestVoteRequest{
		Term:         1,
		CandidateID:  "broker1",
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	payload, messageType, err := manager.serializeRaftMessage(request)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	if messageType != pb.RaftMessageType_RAFT_MESSAGE_TYPE_REQUEST_VOTE {
		t.Errorf("期望消息类型为REQUEST_VOTE，实际为: %v", messageType)
	}

	// 测试反序列化
	deserialized, err := manager.deserializeRaftMessage(messageType, payload)
	if err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	deserializedRequest, ok := deserialized.(*RequestVoteRequest)
	if !ok {
		t.Fatal("反序列化结果类型错误")
	}

	if deserializedRequest.Term != request.Term {
		t.Errorf("期望Term为%d，实际为%d", request.Term, deserializedRequest.Term)
	}

	// 测试移除Topic
	err = manager.RemoveTopic("test-topic")
	if err != nil {
		t.Fatalf("移除Topic失败: %v", err)
	}

	// 检查Topic是否已移除
	if manager.IsLeader("test-topic") {
		t.Error("Topic应该已被移除")
	}

	// 清理
	manager.Close()
}

func TestRaftManagerFollower(t *testing.T) {
	// 初始化日志
	utils.InitLogger("debug", "")

	// 创建Raft管理器
	manager := NewRaftManager("broker2", "localhost:50052")

	// 测试设置Follower角色
	err := manager.SetTopicRole("test-topic", pb.TopicRole_TOPIC_ROLE_FOLLOWER,
		[]string{}, "localhost:50051")
	if err != nil {
		t.Fatalf("设置Topic角色失败: %v", err)
	}

	// 检查角色设置
	if !manager.IsFollower("test-topic") {
		t.Error("期望是Follower角色")
	}

	if manager.IsLeader("test-topic") {
		t.Error("不应该是Leader角色")
	}

	// 测试Follower不能提议消息
	err = manager.ProposeMessage("test-topic", 1, []byte("test message"))
	if err == nil {
		t.Error("Follower不应该能够提议消息")
	}

	// 清理
	manager.Close()
}

func TestRaftMessageSerialization(t *testing.T) {
	// 初始化日志
	utils.InitLogger("debug", "")

	// 创建Raft管理器
	manager := NewRaftManager("broker1", "localhost:50051")

	// 测试各种消息类型的序列化和反序列化
	testCases := []struct {
		name    string
		message interface{}
		msgType pb.RaftMessageType
	}{
		{
			name: "RequestVoteRequest",
			message: &RequestVoteRequest{
				Term:         1,
				CandidateID:  "broker1",
				LastLogIndex: 0,
				LastLogTerm:  0,
			},
			msgType: pb.RaftMessageType_RAFT_MESSAGE_TYPE_REQUEST_VOTE,
		},
		{
			name: "RequestVoteResponse",
			message: &RequestVoteResponse{
				Term:        1,
				VoteGranted: true,
			},
			msgType: pb.RaftMessageType_RAFT_MESSAGE_TYPE_REQUEST_VOTE_RESPONSE,
		},
		{
			name: "AppendEntriesRequest",
			message: &AppendEntriesRequest{
				Term:         1,
				LeaderID:     "broker1",
				PrevLogIndex: 0,
				PrevLogTerm:  0,
				Entries:      []LogEntry{},
				LeaderCommit: 0,
			},
			msgType: pb.RaftMessageType_RAFT_MESSAGE_TYPE_APPEND_ENTRIES,
		},
		{
			name: "AppendEntriesResponse",
			message: &AppendEntriesResponse{
				Term:    1,
				Success: true,
			},
			msgType: pb.RaftMessageType_RAFT_MESSAGE_TYPE_APPEND_ENTRIES_RESPONSE,
		},
		{
			name: "MessageCommand",
			message: &MessageCommand{
				Topic:     "test-topic",
				MessageID: 1,
				Payload:   []byte("test message"),
				Timestamp: time.Now().Unix(),
			},
			msgType: pb.RaftMessageType_RAFT_MESSAGE_TYPE_COMMAND,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 序列化
			payload, messageType, err := manager.serializeRaftMessage(tc.message)
			if err != nil {
				t.Fatalf("序列化失败: %v", err)
			}

			if messageType != tc.msgType {
				t.Errorf("期望消息类型为%v，实际为%v", tc.msgType, messageType)
			}

			// 反序列化
			deserialized, err := manager.deserializeRaftMessage(messageType, payload)
			if err != nil {
				t.Fatalf("反序列化失败: %v", err)
			}

			// 检查反序列化结果不为空
			if deserialized == nil {
				t.Error("反序列化结果为空")
			}
		})
	}

	// 清理
	manager.Close()
}
