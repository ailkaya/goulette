package etcd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/server/v3/embed"
)

// TestEtcdServer 测试用的etcd服务器
type TestEtcdServer struct {
	Server *embed.Etcd
	Config *embed.Config
}

// NewTestEtcdServer 创建测试用的etcd服务器
func NewTestEtcdServer(t *testing.T) *TestEtcdServer {
	cfg := embed.NewConfig()
	cfg.Dir = t.TempDir()
	clientURL, _ := url.Parse("http://localhost:0")
	peerURL, _ := url.Parse("http://localhost:0")
	cfg.ListenClientUrls = []url.URL{*clientURL}
	cfg.ListenPeerUrls = []url.URL{*peerURL}
	cfg.InitialCluster = cfg.InitialClusterFromName(cfg.Name)

	etcdServer, err := embed.StartEtcd(cfg)
	require.NoError(t, err)

	// 等待etcd服务器启动
	select {
	case <-etcdServer.Server.ReadyNotify():
	case <-time.After(10 * time.Second):
		t.Fatal("etcd服务器启动超时")
	}

	return &TestEtcdServer{
		Server: etcdServer,
		Config: cfg,
	}
}

// Close 关闭测试服务器
func (ts *TestEtcdServer) Close() {
	if ts.Server != nil {
		ts.Server.Close()
	}
}

// GetClientEndpoints 获取客户端端点
func (ts *TestEtcdServer) GetClientEndpoints() []string {
	return []string{ts.Config.ListenClientUrls[0].String()}
}

// NewTestEtcdManager 创建测试用的EtcdManager
func NewTestEtcdManager(t *testing.T) (*EtcdManager, *TestEtcdServer) {
	server := NewTestEtcdServer(t)

	endpoints := server.GetClientEndpoints()
	manager, err := NewEtcdManager(endpoints)
	require.NoError(t, err)

	// 等待leader选举
	time.Sleep(2 * time.Second)

	return manager, server
}

// CreateTestTopicReplicaInfo 创建测试用的Topic副本信息
func CreateTestTopicReplicaInfo(topic string, brokerCount int) TopicReplicaInfo {
	brokers := make([]BrokerInfo, brokerCount)
	for i := 0; i < brokerCount; i++ {
		brokers[i] = BrokerInfo{
			BrokerID: fmt.Sprintf("broker%d", i+1),
			Address:  fmt.Sprintf("localhost:%d", 8080+i),
			Status:   1,
		}
	}

	return TopicReplicaInfo{
		Topic:        topic,
		Brokers:      brokers,
		Version:      time.Now().UnixNano(),
		Timestamp:    time.Now().Unix(),
		ReplicaCount: brokerCount,
	}
}

// CreateTestOperationLog 创建测试用的操作日志
func CreateTestOperationLog(id int64, function string, params interface{}) OperationLog {
	paramsBytes, _ := json.Marshal(params)
	return OperationLog{
		ID:        id,
		Function:  function,
		Params:    string(paramsBytes),
		Timestamp: time.Now().Unix(),
	}
}
