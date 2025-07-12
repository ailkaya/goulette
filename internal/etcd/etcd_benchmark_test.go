package etcd

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/server/v3/embed"
)

// BenchmarkEtcdManager 性能基准测试
func BenchmarkEtcdManager(b *testing.B) {
	// 启动测试用的etcd服务器
	cfg := embed.NewConfig()
	cfg.Dir = b.TempDir()

	clientURL, _ := url.Parse("http://localhost:0")
	peerURL, _ := url.Parse("http://localhost:0")
	cfg.ListenClientUrls = []url.URL{*clientURL}
	cfg.ListenPeerUrls = []url.URL{*peerURL}
	cfg.InitialCluster = cfg.InitialClusterFromName(cfg.Name)

	etcdServer, err := embed.StartEtcd(cfg)
	require.NoError(b, err)
	defer etcdServer.Close()

	select {
	case <-etcdServer.Server.ReadyNotify():
	case <-time.After(10 * time.Second):
		b.Fatal("etcd服务器启动超时")
	}

	endpoints := []string{cfg.ListenClientUrls[0].String()}
	manager, err := NewEtcdManager(endpoints)
	require.NoError(b, err)
	defer manager.Close()

	time.Sleep(2 * time.Second)

	b.Run("BenchmarkLogOperation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			function := fmt.Sprintf("benchmark_function_%d", i)
			params := map[string]interface{}{
				"iteration": i,
				"timestamp": time.Now().Unix(),
			}
			err := manager.LogOperation(function, params)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("BenchmarkGetTopicReplicas", func(b *testing.B) {
		// 先设置一些测试数据
		topic := "benchmark-topic"
		brokers := []BrokerInfo{
			{BrokerID: "broker1", Address: "localhost:8080", Status: 1},
		}
		err := manager.SetTopicReplicas(topic, brokers, 1)
		require.NoError(b, err)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := manager.GetTopicReplicas(topic)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("BenchmarkSetTopicReplicas", func(b *testing.B) {
		brokers := []BrokerInfo{
			{BrokerID: "broker1", Address: "localhost:8080", Status: 1},
			{BrokerID: "broker2", Address: "localhost:8081", Status: 1},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			topic := fmt.Sprintf("benchmark-topic-%d", i)
			err := manager.SetTopicReplicas(topic, brokers, 2)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("BenchmarkGetAllOperations", func(b *testing.B) {
		// 先记录一些操作日志
		for i := 0; i < 100; i++ {
			function := fmt.Sprintf("benchmark_operation_%d", i)
			params := map[string]interface{}{
				"index": i,
			}
			err := manager.LogOperation(function, params)
			require.NoError(b, err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := manager.GetAllOperations()
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("BenchmarkGetNextID", func(b *testing.B) {
		key := IncrementIDPrefix + "benchmark_increment"

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := manager.getNextID(key)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkConcurrentOperations 并发操作基准测试
func BenchmarkConcurrentOperations(b *testing.B) {
	// 启动测试用的etcd服务器
	cfg := embed.NewConfig()
	cfg.Dir = b.TempDir()

	clientURL, _ := url.Parse("http://localhost:0")
	peerURL, _ := url.Parse("http://localhost:0")
	cfg.ListenClientUrls = []url.URL{*clientURL}
	cfg.ListenPeerUrls = []url.URL{*peerURL}
	cfg.InitialCluster = cfg.InitialClusterFromName(cfg.Name)

	etcdServer, err := embed.StartEtcd(cfg)
	require.NoError(b, err)
	defer etcdServer.Close()

	select {
	case <-etcdServer.Server.ReadyNotify():
	case <-time.After(10 * time.Second):
		b.Fatal("etcd服务器启动超时")
	}

	endpoints := []string{cfg.ListenClientUrls[0].String()}
	manager, err := NewEtcdManager(endpoints)
	require.NoError(b, err)
	defer manager.Close()

	time.Sleep(2 * time.Second)

	b.Run("BenchmarkConcurrentLogOperation", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				function := fmt.Sprintf("concurrent_benchmark_function_%d", i)
				params := map[string]interface{}{
					"iteration": i,
					"timestamp": time.Now().Unix(),
				}
				err := manager.LogOperation(function, params)
				if err != nil {
					b.Fatal(err)
				}
				i++
			}
		})
	})

	b.Run("BenchmarkConcurrentSetTopicReplicas", func(b *testing.B) {
		brokers := []BrokerInfo{
			{BrokerID: "broker1", Address: "localhost:8080", Status: 1},
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				topic := fmt.Sprintf("concurrent_benchmark_topic_%d", i)
				err := manager.SetTopicReplicas(topic, brokers, 1)
				if err != nil {
					b.Fatal(err)
				}
				i++
			}
		})
	})
}
