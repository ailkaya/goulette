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

// Producer 生产者
type Producer struct {
	sentinelAddress string
	brokerClients   map[string]pb.BrokerServiceClient
	mu              sync.RWMutex
	messageCounter  uint64
	windowSize      int
	window          *SlidingWindow
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// SlidingWindow 滑动窗口
type SlidingWindow struct {
	size   int
	window map[uint64]bool
	mu     sync.Mutex
	cond   *sync.Cond
}

// NewSlidingWindow 创建滑动窗口
func NewSlidingWindow(size int) *SlidingWindow {
	sw := &SlidingWindow{
		size:   size,
		window: make(map[uint64]bool),
	}
	sw.cond = sync.NewCond(&sw.mu)
	return sw
}

// WaitForSlot 等待可用槽位
func (sw *SlidingWindow) WaitForSlot() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	for len(sw.window) >= sw.size {
		sw.cond.Wait()
	}
}

// Add 添加消息ID到窗口
func (sw *SlidingWindow) Add(messageID uint64) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.window[messageID] = true
}

// Remove 从窗口移除消息ID
func (sw *SlidingWindow) Remove(messageID uint64) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	delete(sw.window, messageID)
	sw.cond.Signal()
}

// NewProducer 创建新的生产者
func NewProducer(sentinelAddress string, windowSize int) *Producer {
	// 初始化日志系统
	if !utils.IsInitialized() {
		logPath := "logs/producer.log"
		if err := utils.InitLogger("info", logPath); err != nil {
			// 如果文件日志初始化失败，使用控制台日志
			utils.InitLogger("info", "")
		}
	}

	return &Producer{
		sentinelAddress: sentinelAddress,
		brokerClients:   make(map[string]pb.BrokerServiceClient),
		windowSize:      windowSize,
		window:          NewSlidingWindow(windowSize),
		stopChan:        make(chan struct{}),
	}
}

// Connect 连接到Broker
func (p *Producer) Connect(brokerAddress string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.brokerClients[brokerAddress]; exists {
		return nil
	}

	conn, err := grpc.NewClient(brokerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("连接Broker失败: %w", err)
	}

	// 这里需要实现BrokerServiceClient
	client := pb.NewBrokerServiceClient(conn)
	p.brokerClients[brokerAddress] = client

	utils.Infof("连接到Broker: %s", brokerAddress)
	return nil
}

// getTopicLeader 从Sentinel获取Topic Leader
func (p *Producer) getTopicLeader(topic string, replicaCount uint32) (string, error) {
	conn, err := grpc.NewClient(p.sentinelAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

// SendMessage 发送消息
func (p *Producer) SendMessage(topic string, payload []byte) (uint64, error) {
	// 获取Topic Leader
	leaderAddress, err := p.getTopicLeader(topic, 3) // 3副本
	if err != nil {
		return 0, err
	}

	if leaderAddress == "" {
		return 0, fmt.Errorf("没有可用的Topic Leader")
	}

	// 连接到Leader Broker
	if err := p.Connect(leaderAddress); err != nil {
		return 0, fmt.Errorf("连接Leader Broker失败: %w", err)
	}

	// 生成消息ID
	p.mu.Lock()
	p.messageCounter++
	messageID := p.messageCounter
	p.mu.Unlock()

	// 等待滑动窗口槽位
	p.window.WaitForSlot()
	p.window.Add(messageID)

	// 发送消息到Leader Broker
	client := p.brokerClients[leaderAddress]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := client.SendMessage(ctx)
	if err != nil {
		p.window.Remove(messageID)
		return 0, fmt.Errorf("创建发送流失败: %w", err)
	}

	// 发送消息
	req := &pb.MessageRequest{
		Topic:     topic,
		Payload:   payload,
		MessageId: messageID,
		Timestamp: &pb.Timestamp{
			Seconds: time.Now().Unix(),
		},
		Type: pb.MessageType_MESSAGE_TYPE_NORMAL,
	}

	if err := stream.Send(req); err != nil {
		p.window.Remove(messageID)
		return 0, fmt.Errorf("发送消息失败: %w", err)
	}

	// 接收响应
	resp, err := stream.Recv()
	if err != nil {
		p.window.Remove(messageID)
		return 0, fmt.Errorf("接收响应失败: %w", err)
	}

	// 关闭流
	if err := stream.CloseSend(); err != nil {
		utils.Warnf("关闭发送流失败: %v", err)
	}

	// 检查响应状态
	if resp.Status == pb.ResponseStatus_RESPONSE_STATUS_RETRY {
		// Topic被移除，重新向sentinel获取leader
		p.window.Remove(messageID)
		utils.Infof("Topic被移除，重新向sentinel获取leader: %s", resp.ErrorMessage)

		// 清除当前broker连接缓存，强制重新获取
		p.mu.Lock()
		delete(p.brokerClients, leaderAddress)
		p.mu.Unlock()

		// 递归调用自身，重新获取leader并发送
		return p.SendMessage(topic, payload)
	} else if resp.Status != pb.ResponseStatus_RESPONSE_STATUS_SUCCESS {
		p.window.Remove(messageID)
		return 0, fmt.Errorf("发送消息失败: %s", resp.ErrorMessage)
	}

	// 从滑动窗口移除
	p.window.Remove(messageID)

	utils.Debugf("消息发送成功: topic=%s, id=%d, size=%d", topic, messageID, len(payload))
	return messageID, nil
}

// SendMessageAsync 异步发送消息
func (p *Producer) SendMessageAsync(topic string, payload []byte, callback func(uint64, error)) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		messageID, err := p.SendMessage(topic, payload)
		if callback != nil {
			callback(messageID, err)
		}
	}()
}

// Close 关闭生产者
func (p *Producer) Close() {
	close(p.stopChan)
	p.wg.Wait()
	utils.Info("生产者已关闭")
}
