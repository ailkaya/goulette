package etcd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/server/v3/embed"
)

// TestEtcdManagerConcurrency 测试并发安全性
func TestEtcdManagerConcurrency(t *testing.T) {
	// 启动测试用的etcd服务器
	cfg := embed.NewConfig()
	cfg.Dir = t.TempDir()

	clientURL, _ := url.Parse("http://localhost:0")
	peerURL, _ := url.Parse("http://localhost:0")
	cfg.ListenClientUrls = []url.URL{*clientURL}
	cfg.ListenPeerUrls = []url.URL{*peerURL}
	cfg.InitialCluster = cfg.InitialClusterFromName(cfg.Name)

	etcdServer, err := embed.StartEtcd(cfg)
	require.NoError(t, err)
	defer etcdServer.Close()

	select {
	case <-etcdServer.Server.ReadyNotify():
	case <-time.After(10 * time.Second):
		t.Fatal("etcd服务器启动超时")
	}

	endpoints := []string{cfg.ListenClientUrls[0].String()}
	manager, err := NewEtcdManager(endpoints)
	require.NoError(t, err)
	defer manager.Close()

	time.Sleep(2 * time.Second)

	t.Run("测试并发操作日志记录", func(t *testing.T) {
		const numGoroutines = 10
		const operationsPerGoroutine = 5

		done := make(chan bool, numGoroutines)
		errors := make(chan error, numGoroutines*operationsPerGoroutine)

		// 启动多个goroutine并发记录操作日志
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer func() { done <- true }()

				for j := 0; j < operationsPerGoroutine; j++ {
					function := fmt.Sprintf("concurrent_function_%d_%d", id, j)
					params := map[string]interface{}{
						"goroutine_id": id,
						"operation_id": j,
					}

					err := manager.LogOperation(function, params)
					if err != nil {
						errors <- err
					}
				}
			}(i)
		}

		// 等待所有goroutine完成
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// 检查是否有错误
		close(errors)
		for err := range errors {
			t.Errorf("并发操作日志记录错误: %v", err)
		}

		// 验证所有操作都被记录
		allOperations, err := manager.GetAllOperations()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(allOperations), numGoroutines*operationsPerGoroutine)
	})
}

// TestEtcdManagerErrorHandling 测试错误处理
func TestEtcdManagerErrorHandling(t *testing.T) {
	t.Run("测试连接无效端点", func(t *testing.T) {
		invalidEndpoints := []string{"http://invalid-endpoint:2379"}
		_, err := NewEtcdManager(invalidEndpoints)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "创建etcd客户端失败")
	})

	t.Run("测试JSON序列化错误", func(t *testing.T) {
		// 启动测试用的etcd服务器
		cfg := embed.NewConfig()
		cfg.Dir = t.TempDir()

		clientURL, _ := url.Parse("http://localhost:0")
		peerURL, _ := url.Parse("http://localhost:0")
		cfg.ListenClientUrls = []url.URL{*clientURL}
		cfg.ListenPeerUrls = []url.URL{*peerURL}
		cfg.InitialCluster = cfg.InitialClusterFromName(cfg.Name)

		etcdServer, err := embed.StartEtcd(cfg)
		require.NoError(t, err)
		defer etcdServer.Close()

		select {
		case <-etcdServer.Server.ReadyNotify():
		case <-time.After(10 * time.Second):
			t.Fatal("etcd服务器启动超时")
		}

		endpoints := []string{cfg.ListenClientUrls[0].String()}
		manager, err := NewEtcdManager(endpoints)
		require.NoError(t, err)
		defer manager.Close()

		time.Sleep(2 * time.Second)

		// 创建一个无法序列化的参数（包含函数）
		unserializableParams := map[string]interface{}{
			"func": func() {},
		}

		err = manager.LogOperation("test_function", unserializableParams)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "序列化参数失败")
	})
}

// TestTopicReplicaInfoJSON 测试TopicReplicaInfo的JSON序列化
func TestTopicReplicaInfoJSON(t *testing.T) {
	info := TopicReplicaInfo{
		Topic: "test-topic",
		Brokers: []BrokerInfo{
			{BrokerID: "broker1", Address: "localhost:8080", Status: 1},
			{BrokerID: "broker2", Address: "localhost:8081", Status: 0},
		},
		Version:      1234567890,
		Timestamp:    1234567890,
		ReplicaCount: 2,
	}

	// 测试序列化
	data, err := json.Marshal(info)
	require.NoError(t, err)

	// 测试反序列化
	var decodedInfo TopicReplicaInfo
	err = json.Unmarshal(data, &decodedInfo)
	require.NoError(t, err)

	assert.Equal(t, info.Topic, decodedInfo.Topic)
	assert.Equal(t, info.Version, decodedInfo.Version)
	assert.Equal(t, info.Timestamp, decodedInfo.Timestamp)
	assert.Equal(t, info.ReplicaCount, decodedInfo.ReplicaCount)
	assert.Len(t, decodedInfo.Brokers, 2)
	assert.Equal(t, info.Brokers[0].BrokerID, decodedInfo.Brokers[0].BrokerID)
	assert.Equal(t, info.Brokers[1].BrokerID, decodedInfo.Brokers[1].BrokerID)
}

// TestOperationLogJSON 测试OperationLog的JSON序列化
func TestOperationLogJSON(t *testing.T) {
	log := OperationLog{
		ID:        123,
		Function:  "test_function",
		Params:    `{"key":"value"}`,
		Timestamp: 1234567890,
	}

	// 测试序列化
	data, err := json.Marshal(log)
	require.NoError(t, err)

	// 测试反序列化
	var decodedLog OperationLog
	err = json.Unmarshal(data, &decodedLog)
	require.NoError(t, err)

	assert.Equal(t, log.ID, decodedLog.ID)
	assert.Equal(t, log.Function, decodedLog.Function)
	assert.Equal(t, log.Params, decodedLog.Params)
	assert.Equal(t, log.Timestamp, decodedLog.Timestamp)
}
