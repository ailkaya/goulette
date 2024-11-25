package client

import (
	"context"
	"fmt"

	pb "github.com/ailkaya/goport/broker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	DefaultClientConfig = &ClientConfig{
		InBufferSize:  1000,
		OutBufferSize: 1000,
	}
)

type Client struct {
	topic   string
	brokers map[string]*sendAndRecv
	in      chan []byte
	out     chan []byte
}

type sendAndRecv struct {
	serviceClient pb.BrokerServiceClient
	send          *StreamGroup
	recv          *StreamGroup
}

type ClientConfig struct {
	InBufferSize  int64
	OutBufferSize int64
}

func NewClient(topic string, brokers []string, config *ClientConfig) (*Client, error) {
	config = parseClientConfig(config)
	tc := &Client{
		topic:   topic,
		brokers: make(map[string]*sendAndRecv),
		in:      make(chan []byte, config.InBufferSize),
		out:     make(chan []byte, config.OutBufferSize),
	}
	for _, broker := range brokers {
		// 连接到broker
		conn, err := connectBroker(broker)
		if err != nil {
			return nil, err
		}
		tc.brokers[broker] = &sendAndRecv{
			serviceClient: conn,
		}
		// 注册topic
		_, err = conn.RegisterTopic(context.Background(), &pb.TopicRequest{Topic: topic})
		if err != nil {
			return nil, err
		}
	}
	return tc, nil
}

// 开启发送流
func (tc *Client) OpenSend(brokers []string, groupConfig *StreamGroupConfig) error {
	groupConfig = parseStreamGroupConfig(groupConfig)
	for _, broker := range brokers {
		if _, ok := tc.brokers[broker]; !ok {
			return fmt.Errorf("broker %s not found", broker)
		}
		if tc.brokers[broker].send == nil {
			tc.brokers[broker].send = NewStreamGroup(tc.topic, KernelTypeOfSend, tc.in, nil, tc.brokers[broker].serviceClient, groupConfig)
		}
	}
	return nil
}

// 开启接收流
func (tc *Client) OpenRecv(brokers []string, groupConfig *StreamGroupConfig) error {
	groupConfig = parseStreamGroupConfig(groupConfig)
	for _, broker := range brokers {
		if _, ok := tc.brokers[broker]; !ok {
			return fmt.Errorf("broker %s not found", broker)
		}
		if tc.brokers[broker].recv == nil {
			tc.brokers[broker].recv = NewStreamGroup(tc.topic, KernelTypeOfRecv, tc.out, nil, tc.brokers[broker].serviceClient, groupConfig)
		}
	}
	return nil
}

func (tc *Client) OpenRecvWithSureProcessed(brokers []string, groupConfig *StreamGroupConfig, processFunc func([]byte) error) error {
	groupConfig = parseStreamGroupConfig(groupConfig)
	for _, broker := range brokers {
		if _, ok := tc.brokers[broker]; !ok {
			return fmt.Errorf("broker %s not found", broker)
		}
		if tc.brokers[broker].recv == nil {
			tc.brokers[broker].recv = NewStreamGroup(tc.topic, KernelTypeOfSureProcessed, nil, processFunc, tc.brokers[broker].serviceClient, groupConfig)
		}
	}
	return nil
}

func (tc *Client) Push(v []byte) {
	tc.in <- v
}

func (tc *Client) Pull() []byte {
	return <-tc.out
}

func (tc *Client) Close() {
	for _, broker := range tc.brokers {
		broker.send.Close()
		broker.recv.Close()
	}
}

// 连接到broker
func connectBroker(addr string) (pb.BrokerServiceClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return pb.NewBrokerServiceClient(conn), nil
}

func parseClientConfig(config *ClientConfig) *ClientConfig {
	if config == nil {
		return DefaultClientConfig
	}
	if config.InBufferSize <= 0 {
		config.InBufferSize = DefaultClientConfig.InBufferSize
	}
	if config.OutBufferSize <= 0 {
		config.OutBufferSize = DefaultClientConfig.OutBufferSize
	}
	return config
}
