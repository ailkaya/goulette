package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ailkaya/goulette/internal/utils"
	pb "github.com/ailkaya/goulette/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Consumer 消费者
type Consumer struct {
	sentinelAddress string
	brokerClients   map[string]pb.BrokerServiceClient
	mu              sync.RWMutex
	windowSize      int
	window          *SlidingWindow
	stopChan        chan struct{}
	wg              sync.WaitGroup
	messageHandler  func(*pb.Message) error
}

// NewConsumer 创建新的消费者
func NewConsumer(sentinelAddress string, windowSize int, messageHandler func(*pb.Message) error) *Consumer {
	// 初始化日志系统
	if !utils.IsInitialized() {
		logPath := "logs/consumer.log"
		if err := utils.InitLogger("info", logPath); err != nil {
			// 如果文件日志初始化失败，使用控制台日志
			utils.InitLogger("info", "")
		}
	}

	return &Consumer{
		sentinelAddress: sentinelAddress,
		brokerClients:   make(map[string]pb.BrokerServiceClient),
		windowSize:      windowSize,
		window:          NewSlidingWindow(windowSize),
		stopChan:        make(chan struct{}),
		messageHandler:  messageHandler,
	}
}

// Connect 连接到Broker
func (c *Consumer) Connect(brokerAddress string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.brokerClients[brokerAddress]; exists {
		return nil
	}

	conn, err := grpc.NewClient(brokerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("连接Broker失败: %w", err)
	}

	// 这里需要实现BrokerServiceClient
	client := pb.NewBrokerServiceClient(conn)
	c.brokerClients[brokerAddress] = client

	utils.Infof("连接到Broker: %s", brokerAddress)
	return nil
}

// getTopicLeader 从Sentinel获取Topic Leader
func (c *Consumer) getTopicLeader(topic string, replicaCount uint32) (string, error) {
	conn, err := grpc.NewClient(c.sentinelAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", fmt.Errorf("连接Sentinel失败: %w", err)
	}
	defer conn.Close()

	// 这里需要实现SentinelServiceClient
	client := pb.NewSentinelServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.GetTopicLeader(ctx, &pb.GetTopicLeaderRequest{
		Topic:        topic,
		ReplicaCount: replicaCount,
	})
	if err != nil {
		return "", fmt.Errorf("获取Topic Leader失败: %w", err)
	}

	if resp.Leader == nil {
		return "", fmt.Errorf("没有可用的Topic Leader")
	}

	return resp.Leader.Address, nil
}

// StartConsuming 开始消费消息
func (c *Consumer) StartConsuming(topic string, consumerGroup string, offset uint64) error {
	// 获取Topic Leader
	leaderAddress, err := c.getTopicLeader(topic, 3) // 3副本
	if err != nil {
		return err
	}

	if leaderAddress == "" {
		return fmt.Errorf("没有可用的Topic Leader")
	}

	// 连接到Leader Broker
	if err := c.Connect(leaderAddress); err != nil {
		return fmt.Errorf("连接Leader Broker失败: %w", err)
	}

	// 启动消费协程
	c.wg.Add(1)
	go c.consumeFromBroker(topic, consumerGroup, offset, leaderAddress)

	utils.Infof("开始消费消息: topic=%s, group=%s, offset=%d", topic, consumerGroup, offset)
	return nil
}

// consumeFromBroker 从指定Broker消费消息
func (c *Consumer) consumeFromBroker(topic, consumerGroup string, offset uint64, brokerAddress string) {
	defer c.wg.Done()

	client := c.brokerClients[brokerAddress]
	currentOffset := offset

	for {
		select {
		case <-c.stopChan:
			return
		default:
			// 等待滑动窗口槽位
			c.window.WaitForSlot()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			// 创建拉取请求
			req := &pb.PullRequest{
				Topic:         topic,
				Offset:        currentOffset,
				BatchSize:     10, // 每次拉取10条消息
				ConsumerGroup: consumerGroup,
			}

			// 拉取消息
			stream, err := client.PullMessage(ctx, req)
			if err != nil {
				cancel()

				// 检查是否是Topic被移除的错误
				if err.Error() == fmt.Sprintf("Topic已被移除: %s", topic) {
					utils.Infof("Topic被移除，重新向sentinel获取broker: %s", err.Error())

					// 清除当前broker连接缓存，强制重新获取
					c.mu.Lock()
					delete(c.brokerClients, brokerAddress)
					c.mu.Unlock()

					// 重新获取broker并重新开始消费
					if err := c.restartConsuming(topic, consumerGroup, currentOffset); err != nil {
						utils.Errorf("重新开始消费失败: %v", err)
						time.Sleep(5 * time.Second)
					}
					return
				}

				utils.Errorf("拉取消息失败: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			// 处理消息流
			for {
				select {
				case <-c.stopChan:
					cancel()
					return
				default:
					message, err := stream.Recv()
					if err != nil {
						cancel()
						if err.Error() == "EOF" {
							// 流结束，等待一段时间后继续
							time.Sleep(100 * time.Millisecond)
							break
						}
						utils.Errorf("接收消息失败: %v", err)
						time.Sleep(1 * time.Second)
						break
					}

					// 添加到滑动窗口
					c.window.Add(message.MessageId)

					// 处理消息
					c.wg.Add(1)
					go func(msg *pb.Message) {
						defer c.wg.Done()
						defer c.window.Remove(msg.MessageId)

						if c.messageHandler != nil {
							if err := c.messageHandler(msg); err != nil {
								utils.Errorf("处理消息失败: %v", err)
							}
						}

						utils.Debugf("处理消息: topic=%s, id=%d, offset=%d",
							msg.Topic, msg.MessageId, msg.Offset)
					}(message)

					currentOffset = message.Offset + 1
				}
			}
		}
	}
}

// restartConsuming 重新开始消费
func (c *Consumer) restartConsuming(topic, consumerGroup string, offset uint64) error {
	// 获取新的Topic Leader
	leaderAddress, err := c.getTopicLeader(topic, 3) // 3副本
	if err != nil {
		return fmt.Errorf("重新获取Topic Leader失败: %w", err)
	}

	if leaderAddress == "" {
		return fmt.Errorf("没有可用的Topic Leader")
	}

	// 连接到新的Leader Broker
	if err := c.Connect(leaderAddress); err != nil {
		return fmt.Errorf("连接Leader Broker失败: %w", err)
	}

	// 启动消费协程
	c.wg.Add(1)
	go c.consumeFromBroker(topic, consumerGroup, offset, leaderAddress)

	utils.Infof("重新开始消费消息: topic=%s, group=%s, offset=%d", topic, consumerGroup, offset)
	return nil
}

// StopConsuming 停止消费
func (c *Consumer) StopConsuming() {
	close(c.stopChan)
	c.wg.Wait()
	utils.Info("消费者已停止")
}

// Close 关闭消费者
func (c *Consumer) Close() {
	c.StopConsuming()
	utils.Info("消费者已关闭")
}
