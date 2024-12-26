package client

import (
	"context"
	"fmt"

	pb "github.com/ailkaya/goport/broker"
	kn "github.com/ailkaya/goport/client/kernel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Scheduler struct {
	topic string
	// brokers                []string
	config        *Config
	brokerClients map[string]pb.BrokerServiceClient
	containers    []*StreamContainer
	// maxReceive             int
	// reception              IReception
	preProcessorController IPreProcessorController
	in                     chan []byte
	out                    chan []byte
}

func NewScheduler(topic string, config *Config) (*Client, error) {
	config, err := parseConfig(config)
	if err != nil {
		return nil, err
	}
	s := &Scheduler{
		topic:  topic,
		config: config,
		// brokers:                config.Brokers,
		brokerClients: make(map[string]pb.BrokerServiceClient),
		containers:    make([]*StreamContainer, 0),
		// maxReceive:             config.MaxReceive,
		// reception:              NewReception(config),
		preProcessorController: NewPreProcessorController(),
		in:                     make(chan []byte, config.InBufferSize),
		out:                    make(chan []byte, config.OutBufferSize),
	}
	for _, broker := range config.Brokers {
		// 连接到broker
		conn, err := connectBroker(broker)
		if err != nil {
			return nil, err
		}
		// 注册topic
		_, err = conn.RegisterTopic(context.Background(), &pb.TopicRequest{Topic: topic})
		if err != nil {
			return nil, err
		}
		s.brokerClients[broker] = conn
	}
	return NewClient(s), nil
}

func (s *Scheduler) getIn() chan []byte {
	return s.in
}

func (s *Scheduler) getOut() chan []byte {
	return s.out
}

// 连接到broker
func connectBroker(addr string) (pb.BrokerServiceClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return pb.NewBrokerServiceClient(conn), nil
}

func (s *Scheduler) kernelFactory(brokerClient pb.BrokerServiceClient, demand string) (IKernel, error) {
	if demand == kn.SendWithGuaranteedArrival {
		return kn.NewSendArrive("send", s.topic, s.config.SendConcurrency, s.config.AckInterval, brokerClient)
	} else if demand == kn.ReceiveWithGuaranteedReception {
		return kn.NewReceiveArrive("receive", s.topic, s.config.RecvConcurrency, s.config.AckInterval, brokerClient)
	} else {
		return nil, fmt.Errorf("unknown kernel type: %s", demand)
	}
}

// 创建连接某个broker的容器
func (s *Scheduler) containerFactory(broker string) *StreamContainer {
	return NewStreamContainer(s.brokerClients[broker])
}

// 组装
func (s *Scheduler) assemble(container *StreamContainer, kernel IKernel, preProcessor IPreProcessor, postProcessor IPostProcessor) *StreamContainer {
	// 根据不同的kernel类型，注册不同的数据通道
	if kernel.Type() == "send" {
		// 将container注册到消息发送前置控制器中
		s.preProcessorController.RegisterContainer(preProcessor.Register(s.in))
	} else if kernel.Type() == "receive" {
		postProcessor.Register(s.out)
	}
	container.Cite(kernel, preProcessor, postProcessor)
	return container
}

func (s *Scheduler) AuthSend() {
	for broker, client := range s.brokerClients {
		for i := 0; i < s.config.MaxConnections; i++ {
			// fmt.Println("auth send to broker: ", broker)
			// 创建send kernel
			kernel, err := s.kernelFactory(client, kn.SendWithGuaranteedArrival)
			if err != nil {
				panic(err)
			}
			// 创建container
			container := s.containerFactory(broker)
			// 创建preProcessor
			// receive := int(float64(s.config.BaseReceive) * s.config.BrokerWeights[i])
			receive := int(float64(s.config.BaseReceive) * s.config.BrokerWeights[broker])
			preProcessor := NewPreProcessor(receive).Cite(s.preProcessorController)
			// 组装
			container = s.assemble(container, kernel, preProcessor, nil)
			// fmt.Println("auth send container: ", container)
			container.Run()
			s.containers = append(s.containers, container)
		}
	}
	s.preProcessorController.Start()
}

func (s *Scheduler) AuthReceive() {
	for broker, client := range s.brokerClients {
		for i := 0; i < s.config.MaxConnections; i++ {
			// receive kernel
			kernel, err := s.kernelFactory(client, kn.ReceiveWithGuaranteedReception)
			if err != nil {
				panic(err)
			}
			// 创建container
			container := s.containerFactory(broker)
			// 创建postProcessor
			postProcessor := NewPostProcessor()
			// 组装
			container = s.assemble(container, kernel, nil, postProcessor)
			container.Run()
			s.containers = append(s.containers, container)
		}
	}
}

func (s *Scheduler) Close() {
	// 此处不考虑in, out中剩余未处理的数据，直接关闭
	close(s.in)
	// 关闭所有容器
	for _, container := range s.containers {
		container.Close()
	}
	close(s.out)
}
