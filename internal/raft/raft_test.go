package raft

import (
	"testing"
	"time"

	"github.com/ailkaya/goulette/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestRaftNode_Basic(t *testing.T) {
	// 初始化日志
	utils.InitLogger("debug", "")

	// 创建peers映射
	peers := map[string]string{
		"node1": "localhost:8081",
		"node2": "localhost:8082",
		"node3": "localhost:8083",
	}

	// 创建Raft节点
	node := NewRaftNode("node1", peers, func(command interface{}) error {
		utils.Infof("应用命令: %v", command)
		return nil
	}, func(peerID string, message interface{}) error {
		utils.Debugf("发送消息到 %s: %T", peerID, message)
		return nil
	})

	// 启动节点
	node.Start()
	defer node.Stop()

	// 等待一段时间让节点运行
	time.Sleep(100 * time.Millisecond)

	// 检查初始状态
	term, isLeader := node.GetState()
	assert.Equal(t, uint64(0), term)
	assert.False(t, isLeader)

	utils.Infof("Raft节点测试完成")
}

func TestRaftManager_Basic(t *testing.T) {
	// 初始化日志
	utils.InitLogger("debug", "")

	// 创建Raft管理器
	manager := NewRaftManager("broker1", "localhost:8081")

	// 测试设置Topic角色
	err := manager.SetTopicRole("test-topic", 1, []string{"localhost:8082", "localhost:8083"}, "localhost:8081")
	assert.NoError(t, err)

	// 检查角色
	assert.True(t, manager.IsLeader("test-topic"))
	assert.False(t, manager.IsFollower("test-topic"))

	// 测试提议消息
	err = manager.ProposeMessage("test-topic", 1, []byte("test message"))
	assert.NoError(t, err)

	// 清理
	manager.RemoveTopic("test-topic")
	manager.Close()

	utils.Infof("Raft管理器测试完成")
}
