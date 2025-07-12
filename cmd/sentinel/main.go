package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ailkaya/goulette/internal/utils"
	"github.com/ailkaya/goulette/sentinel"
	"go.etcd.io/etcd/server/v3/embed"
)

const (
	DefaultEtcdPort = 2379
	DefaultGrpcPort = 2380
)

func main() {
	var (
		sentinelID     = flag.String("id", "sentinel-1", "Sentinel ID")
		address        = flag.String("address", ":50052", "Sentinel监听地址")
		logLevel       = flag.String("log-level", "info", "日志级别")
		etcdPort       = flag.Int("etcd-port", DefaultEtcdPort, "etcd客户端端口")
		etcdPeerPort   = flag.Int("etcd-peer-port", DefaultGrpcPort, "etcd对等端口")
		etcdDataDir    = flag.String("etcd-data-dir", "default.etcd", "etcd数据目录")
		enableEmbedded = flag.Bool("enable-embedded-etcd", true, "是否启用内嵌etcd服务")
		etcdEndpoints  = flag.String("etcd-endpoints", "", "外部etcd端点列表，用逗号分隔（当enable-embedded-etcd=false时使用）")
	)
	flag.Parse()

	// 设置日志级别
	utils.SetLogLevel(*logLevel)

	// 解析etcd端点
	var endpoints []string
	if *enableEmbedded {
		// 使用内嵌etcd
		endpoints = []string{fmt.Sprintf("localhost:%d", *etcdPort)}
	} else if *etcdEndpoints != "" {
		// 使用外部etcd
		endpoints = strings.Split(*etcdEndpoints, ",")
		for i, endpoint := range endpoints {
			endpoints[i] = strings.TrimSpace(endpoint)
		}
	}

	// 启动内嵌etcd服务
	var etcdServer *embed.Etcd
	if *enableEmbedded {
		etcdServer = startEmbeddedEtcd(*etcdPort, *etcdPeerPort, *etcdDataDir, *sentinelID)
		defer etcdServer.Close()
	}

	// 创建Sentinel服务器
	server := sentinel.NewSentinelServer(*sentinelID, *address, endpoints)

	// 启动服务器
	if err := server.Start(); err != nil {
		utils.Fatalf("启动Sentinel服务器失败: %v", err)
	}

	utils.Infof("Sentinel服务器已启动: ID=%s, Address=%s, EtcdEndpoints=%v, EmbeddedEtcd=%v",
		*sentinelID, *address, endpoints, *enableEmbedded)

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	utils.Info("收到中断信号，正在关闭服务器...")
	server.Stop()
}

// startEmbeddedEtcd 启动内嵌的etcd服务
func startEmbeddedEtcd(clientPort, peerPort int, dataDir, nodeName string) *embed.Etcd {
	cfg := embed.NewConfig()
	cfg.Dir = dataDir
	cfg.Name = nodeName

	// 设置客户端监听地址
	clientURL, _ := url.Parse(fmt.Sprintf("http://localhost:%d", clientPort))
	cfg.ListenClientUrls = []url.URL{*clientURL}
	cfg.AdvertiseClientUrls = []url.URL{*clientURL}

	// 设置对等节点监听地址
	peerURL, _ := url.Parse(fmt.Sprintf("http://localhost:%d", peerPort))
	cfg.ListenPeerUrls = []url.URL{*peerURL}
	cfg.AdvertisePeerUrls = []url.URL{*peerURL}

	// 设置初始集群配置（单节点模式）
	cfg.InitialCluster = fmt.Sprintf("%s=http://localhost:%d", nodeName, peerPort)
	cfg.ClusterState = embed.ClusterStateFlagNew
	cfg.InitialClusterToken = "goulette-etcd-cluster"

	// 设置日志级别
	cfg.LogLevel = "info"

	// 设置自动压缩
	cfg.AutoCompactionMode = "revision"
	cfg.AutoCompactionRetention = "1000"

	// 设置存储配额
	cfg.QuotaBackendBytes = 8 * 1024 * 1024 * 1024 // 8GB

	utils.Infof("启动内嵌etcd服务: 客户端端口=%d, 对等端口=%d, 数据目录=%s", clientPort, peerPort, dataDir)

	e, err := embed.StartEtcd(cfg)
	if err != nil {
		utils.Fatalf("启动内嵌etcd失败: %v", err)
	}

	// 等待etcd服务就绪
	select {
	case <-e.Server.ReadyNotify():
		utils.Info("内嵌etcd服务已就绪!")
	case <-time.After(60 * time.Second):
		e.Server.Stop()
		utils.Fatal("内嵌etcd服务启动超时!")
	}

	return e
}
