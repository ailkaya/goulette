package sentinel

import (
	"context"
	"testing"

	pb "github.com/ailkaya/goulette/proto"
	"github.com/stretchr/testify/assert"
)

func TestSentinelServer_Cache(t *testing.T) {
	server := NewSentinelServer("test-sentinel", ":50051", []string{})

	// 注册一些测试broker
	ctx := context.Background()

	// 注册broker1
	_, err := server.RegisterBroker(ctx, &pb.RegisterBrokerRequest{
		BrokerId: "broker1",
		Address:  "localhost:8081",
		Topics:   []string{"topic1", "topic2"},
	})
	assert.NoError(t, err)

	// 注册broker2
	_, err = server.RegisterBroker(ctx, &pb.RegisterBrokerRequest{
		BrokerId: "broker2",
		Address:  "localhost:8082",
		Topics:   []string{"topic1", "topic3"},
	})
	assert.NoError(t, err)

	// 注册broker3
	_, err = server.RegisterBroker(ctx, &pb.RegisterBrokerRequest{
		BrokerId: "broker3",
		Address:  "localhost:8083",
		Topics:   []string{"topic2", "topic3"},
	})
	assert.NoError(t, err)

	// 测试第一次查询 - 应该从哈希环获取并缓存
	resp1, err := server.GetTopicLeader(ctx, &pb.GetTopicLeaderRequest{
		Topic:        "topic1",
		ReplicaCount: 2,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp1.Leader)

	// 验证缓存已创建
	server.cacheMu.RLock()
	cache, exists := server.replicationCache["topic1"]
	server.cacheMu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, int64(1), cache.Version)
	assert.Len(t, cache.Brokers, 2)
	assert.Equal(t, 2, cache.ReplicaCount)
	assert.NotNil(t, cache.Leader)

	// 测试第二次查询 - 应该从缓存返回
	resp2, err := server.GetTopicLeader(ctx, &pb.GetTopicLeaderRequest{
		Topic:        "topic1",
		ReplicaCount: 2,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp2.Leader)

	// 验证缓存版本没有变化
	server.cacheMu.RLock()
	cache, exists = server.replicationCache["topic1"]
	server.cacheMu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, int64(1), cache.Version)

	// 测试报告无法连接的broker - 应该寻找替换broker
	resp3, err := server.GetTopicLeader(ctx, &pb.GetTopicLeaderRequest{
		Topic:              "topic1",
		ReplicaCount:       2,
		UnreachableBrokers: []string{"broker1"},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp3.Leader) // 应该找到替换broker作为leader

	// 验证broker1状态已更新
	server.mu.RLock()
	broker1, exists := server.brokers["broker1"]
	server.mu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, BrokerStatusDown, broker1.Status)

	// 验证缓存已更新
	server.cacheMu.RLock()
	cache, exists = server.replicationCache["topic1"]
	server.cacheMu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, int64(2), cache.Version) // 版本应该增加
	assert.Len(t, cache.Brokers, 2)          // 缓存中应该有2个broker（包括替换的）
	assert.NotNil(t, cache.Leader)           // 应该有leader

	// 验证返回的broker不包含不可达的broker1
	brokerIDs := make(map[string]bool)
	for _, broker := range cache.Brokers {
		brokerIDs[broker.BrokerID] = true
	}
	assert.False(t, brokerIDs["broker1"])
}

func TestSentinelServer_ConcurrentAccess(t *testing.T) {
	server := NewSentinelServer("test-sentinel", ":50052", []string{})

	// 注册测试broker
	ctx := context.Background()
	_, err := server.RegisterBroker(ctx, &pb.RegisterBrokerRequest{
		BrokerId: "broker1",
		Address:  "localhost:8081",
		Topics:   []string{"topic1"},
	})
	assert.NoError(t, err)

	_, err = server.RegisterBroker(ctx, &pb.RegisterBrokerRequest{
		BrokerId: "broker2",
		Address:  "localhost:8082",
		Topics:   []string{"topic1"},
	})
	assert.NoError(t, err)

	// 并发查询测试
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			resp, err := server.GetTopicLeader(ctx, &pb.GetTopicLeaderRequest{
				Topic:        "topic1",
				ReplicaCount: 2,
			})
			assert.NoError(t, err)
			assert.NotNil(t, resp.Leader)
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证缓存一致性
	server.cacheMu.RLock()
	cache, exists := server.replicationCache["topic1"]
	server.cacheMu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, int64(1), cache.Version) // 所有并发查询应该共享同一个版本
}

func TestSentinelServer_ReplicaCountMismatch(t *testing.T) {
	server := NewSentinelServer("test-sentinel", ":50053", []string{})

	// 注册测试broker
	ctx := context.Background()
	_, err := server.RegisterBroker(ctx, &pb.RegisterBrokerRequest{
		BrokerId: "broker1",
		Address:  "localhost:8081",
		Topics:   []string{"topic1"},
	})
	assert.NoError(t, err)

	// 第一次查询
	resp1, err := server.GetTopicLeader(ctx, &pb.GetTopicLeaderRequest{
		Topic:        "topic1",
		ReplicaCount: 1,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp1.Leader)

	// 验证缓存已创建
	server.cacheMu.RLock()
	cache, exists := server.replicationCache["topic1"]
	server.cacheMu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, int64(1), cache.Version)
	assert.Equal(t, 1, cache.ReplicaCount)

	// 使用不同的副本数量查询 - 应该重新查询
	resp2, err := server.GetTopicLeader(ctx, &pb.GetTopicLeaderRequest{
		Topic:        "topic1",
		ReplicaCount: 2,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp2.Leader)

	// 验证缓存版本已更新
	server.cacheMu.RLock()
	cache, exists = server.replicationCache["topic1"]
	server.cacheMu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, int64(2), cache.Version) // 版本应该增加
	assert.Equal(t, 2, cache.ReplicaCount)   // 副本数量应该更新
}

func TestSentinelServer_ReplacementBrokers(t *testing.T) {
	server := NewSentinelServer("test-sentinel", ":50055", []string{})

	// 注册多个测试broker
	ctx := context.Background()
	_, err := server.RegisterBroker(ctx, &pb.RegisterBrokerRequest{
		BrokerId: "broker1",
		Address:  "localhost:8081",
		Topics:   []string{"topic1"},
	})
	assert.NoError(t, err)

	_, err = server.RegisterBroker(ctx, &pb.RegisterBrokerRequest{
		BrokerId: "broker2",
		Address:  "localhost:8082",
		Topics:   []string{"topic1"},
	})
	assert.NoError(t, err)

	_, err = server.RegisterBroker(ctx, &pb.RegisterBrokerRequest{
		BrokerId: "broker3",
		Address:  "localhost:8083",
		Topics:   []string{"topic1"},
	})
	assert.NoError(t, err)

	// 第一次查询建立缓存
	resp1, err := server.GetTopicLeader(ctx, &pb.GetTopicLeaderRequest{
		Topic:        "topic1",
		ReplicaCount: 2,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp1.Leader)

	// 报告一个broker不可达，应该找到替换broker
	resp2, err := server.GetTopicLeader(ctx, &pb.GetTopicLeaderRequest{
		Topic:              "topic1",
		ReplicaCount:       2,
		UnreachableBrokers: []string{"broker1"},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp2.Leader) // 应该找到替换broker作为leader

	// 验证返回的broker不包含不可达的broker1
	server.cacheMu.RLock()
	cache, exists := server.replicationCache["topic1"]
	server.cacheMu.RUnlock()
	assert.True(t, exists)

	brokerIDs := make(map[string]bool)
	for _, broker := range cache.Brokers {
		brokerIDs[broker.BrokerID] = true
	}
	assert.False(t, brokerIDs["broker1"])
}
