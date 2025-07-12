package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/ailkaya/goulette/broker"
	"github.com/ailkaya/goulette/internal/utils"
)

func main() {
	var (
		brokerID = flag.String("id", "broker-1", "Broker ID")
		address  = flag.String("address", ":50051", "Broker监听地址")
		dataDir  = flag.String("data", "./data/broker", "数据目录")
		logLevel = flag.String("log-level", "info", "日志级别")
	)
	flag.Parse()

	// 设置日志级别
	utils.SetLogLevel(*logLevel)

	// 创建Broker服务器
	server, err := broker.NewBrokerServer(*brokerID, *address, *dataDir)
	if err != nil {
		utils.Fatalf("创建Broker服务器失败: %v", err)
	}

	// 启动服务器
	if err := server.Start(); err != nil {
		utils.Fatalf("启动Broker服务器失败: %v", err)
	}

	utils.Infof("Broker服务器已启动: ID=%s, Address=%s", *brokerID, *address)

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	utils.Info("收到中断信号，正在关闭服务器...")
	server.Stop()
}
