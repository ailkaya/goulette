package main

import (
	"log"
	"time"

	"github.com/ailkaya/goulette/internal/etcd"
	"github.com/ailkaya/goulette/internal/utils"
)

func TestEtcdHeartBeat() {
	// 初始化日志
	if err := utils.InitLogger("debug", ""); err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}

	// 创建etcd管理器
	etcdManager, err := etcd.NewEtcdManager([]string{"localhost:2379"})
	if err != nil {
		log.Fatalf("创建etcd管理器失败: %v", err)
	}
	defer etcdManager.Close()

	// 启动心跳监听
	etcdManager.WatchBrokerHeartbeats(func(brokerID string, eventType string, broker *etcd.BrokerInfo) {
		switch eventType {
		case "PUT":
			if broker != nil {
				utils.Infof("检测到broker上线: %s (%s)", brokerID, broker.Address)
			}
		case "DELETE":
			utils.Infof("检测到broker下线: %s", brokerID)
		}
	})

	// 模拟broker注册心跳
	brokerID := "broker-001"
	address := "localhost:9090"
	ttl := int64(10) // 10秒TTL

	utils.Info("开始测试etcd lease心跳机制...")

	// 注册broker心跳
	utils.Info("注册broker心跳...")
	if err := etcdManager.RegisterBrokerHeartbeat(brokerID, address, ttl); err != nil {
		log.Fatalf("注册broker心跳失败: %v", err)
	}

	// 等待一段时间
	time.Sleep(5 * time.Second)

	// 续约broker心跳
	utils.Info("续约broker心跳...")
	if err := etcdManager.RenewBrokerHeartbeat(brokerID, address, ttl); err != nil {
		log.Fatalf("续约broker心跳失败: %v", err)
	}

	// 等待一段时间
	time.Sleep(5 * time.Second)

	// 获取所有broker心跳信息
	utils.Info("获取所有broker心跳信息...")
	brokers, err := etcdManager.GetBrokerHeartbeats()
	if err != nil {
		log.Fatalf("获取broker心跳信息失败: %v", err)
	}

	utils.Infof("当前活跃broker数量: %d", len(brokers))
	for _, broker := range brokers {
		utils.Infof("Broker: %s (%s), 状态: %d", broker.BrokerID, broker.Address, broker.Status)
	}

	// 等待TTL过期，观察自动删除
	utils.Info("等待TTL过期...")
	time.Sleep(15 * time.Second)

	// 再次获取broker心跳信息
	brokers, err = etcdManager.GetBrokerHeartbeats()
	if err != nil {
		log.Fatalf("获取broker心跳信息失败: %v", err)
	}

	utils.Infof("TTL过期后活跃broker数量: %d", len(brokers))

	// 手动注销broker心跳
	utils.Info("手动注销broker心跳...")
	if err := etcdManager.UnregisterBrokerHeartbeat(brokerID); err != nil {
		log.Fatalf("注销broker心跳失败: %v", err)
	}

	// 等待一段时间
	time.Sleep(2 * time.Second)

	// 最终检查
	brokers, err = etcdManager.GetBrokerHeartbeats()
	if err != nil {
		log.Fatalf("获取broker心跳信息失败: %v", err)
	}

	utils.Infof("最终活跃broker数量: %d", len(brokers))

	utils.Info("etcd lease心跳机制测试完成")
}
