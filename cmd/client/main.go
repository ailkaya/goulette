package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/ailkaya/goulette/proto"
	"github.com/ailkaya/goulette/client"
	"github.com/ailkaya/goulette/internal/utils"
)

func main() {
	var (
		mode            = flag.String("mode", "producer", "运行模式: producer 或 consumer")
		sentinelAddress = flag.String("sentinel", "localhost:50052", "Sentinel地址")
		topic           = flag.String("topic", "test-topic", "Topic名称")
		message         = flag.String("message", "Hello Goulette!", "要发送的消息")
		consumerGroup   = flag.String("group", "test-group", "消费者组")
		windowSize      = flag.Int("window", 100, "滑动窗口大小")
		logLevel        = flag.String("log-level", "info", "日志级别")
	)
	flag.Parse()

	// 设置日志级别
	utils.SetLogLevel(*logLevel)

	switch *mode {
	case "producer":
		runProducer(*sentinelAddress, *topic, *message, *windowSize)
	case "consumer":
		runConsumer(*sentinelAddress, *topic, *consumerGroup, *windowSize)
	default:
		utils.Fatalf("无效的运行模式: %s", *mode)
	}
}

func runProducer(sentinelAddress, topic, message string, windowSize int) {
	// 创建生产者
	producer := client.NewProducer(sentinelAddress, windowSize)
	defer producer.Close()

	utils.Infof("生产者已启动，连接到Sentinel: %s", sentinelAddress)

	// 发送消息
	messageID, err := producer.SendMessage(topic, []byte(message))
	if err != nil {
		utils.Fatalf("发送消息失败: %v", err)
	}

	utils.Infof("消息发送成功: ID=%d, Topic=%s, Message=%s", messageID, topic, message)
}

func runConsumer(sentinelAddress, topic, consumerGroup string, windowSize int) {
	// 创建消费者
	consumer := client.NewConsumer(sentinelAddress, windowSize, func(msg *pb.Message) error {
		utils.Infof("收到消息: ID=%d, Topic=%s, Payload=%s",
			msg.MessageId, msg.Topic, string(msg.Payload))
		return nil
	})
	defer consumer.Close()

	// 开始消费
	if err := consumer.StartConsuming(topic, consumerGroup, 0); err != nil {
		utils.Fatalf("开始消费失败: %v", err)
	}

	utils.Infof("消费者已启动，监听Topic: %s", topic)

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	utils.Info("收到中断信号，正在关闭消费者...")
}
