package client

import (
	// "fmt"

	pb "github.com/ailkaya/goport/broker"
	kn "github.com/ailkaya/goport/client/kernel"
)

var (
	// 确保被接收
	KernelTypeOfSend = "send"
	KernelTypeOfRecv = "recv"
	// 确保被处理
	KernelTypeOfSureProcessed = "sureProcessed"
	DefaultContainerConfig    = &StreamGroupConfig{
		InitialNofSC: 1,
		MinNofSC:     1,
		MaxNofSC:     1,
		RewardLimit:  0.1,
		PenaltyLimit: 0.1,
	}
)

// 某个broker下的某一topic所对应的stream组
type StreamGroup struct {
	topic string
	// 评估者
	evaluator *GroupEvaluator
	// 正在运行的StreamContainer数量
	numOfSC int64
	// 由自己创建的所有正在运行的StreamContainer
	containers chan *StreamContainer
	// 数据输入/输出通道
	InOrOut chan []byte
	// 数据处理函数(type为KernelTypeOfSureProcessed时)
	ProcessFunc func([]byte) error
	// 与broker service的连接
	serviceClient pb.BrokerServiceClient
	// container核心逻辑函数类型
	KernelType string
	// 配置项
	config *StreamGroupConfig
}

type StreamGroupConfig struct {
	// 初始启动的StreamContainer数量
	InitialNofSC int64
	// StreamContainer最小数量，最小为1
	MinNofSC int64
	// StreamContainer最大数量
	MaxNofSC int64
	// Reward界限
	RewardLimit float64
	// Penalty界限
	PenaltyLimit float64
}

func NewStreamGroup(topic, kernelType string, inOrOut chan []byte, processFunc func([]byte) error, serviceClient pb.BrokerServiceClient, config *StreamGroupConfig) *StreamGroup {
	// config.parseConfig()
	sg := &StreamGroup{
		topic:         topic,
		evaluator:     NewEvaluator(config.RewardLimit, config.PenaltyLimit),
		numOfSC:       0,
		containers:    make(chan *StreamContainer, config.MaxNofSC),
		InOrOut:       inOrOut,
		ProcessFunc:   processFunc,
		serviceClient: serviceClient,
		KernelType:    kernelType,
		config:        config,
	}
	// fmt.Println(config)
	// 初始化StreamContainer
	for i := int64(0); i < config.InitialNofSC; i++ {
		// fmt.Println("create sc")
		sg.CreateSC()
	}
	// 开始周期性评估
	// go sg.evaluator.Evaluate(sg)
	return sg
}

func (s *StreamGroup) GetBrokerClient() pb.BrokerServiceClient {
	return s.serviceClient
}

func (s *StreamGroup) GetNofSC() int64 {
	return s.numOfSC
}

func (s *StreamGroup) UpdateNofSC(addition int64) {
	s.numOfSC += addition
}

// 创建一个StreamContainer
func (s *StreamGroup) CreateSC() {
	if s.numOfSC >= s.config.MaxNofSC {
		return
	}

	var kernel kn.IKernel
	// 初始化kernel对象
	if s.KernelType == KernelTypeOfSend {
		// fmt.Println("create send kernel")
		kernel = kn.NewSendMessageKernel(s.topic, s.InOrOut)
	} else if s.KernelType == KernelTypeOfRecv {
		// fmt.Println("create recv kernel")
		kernel = kn.NewRecvMessageKernel(s.topic, s.InOrOut)
	} else if s.KernelType == KernelTypeOfSureProcessed {
		// fmt.Println("create sure processed kernel")
		kernel = kn.NewRecvWithSureProcessed(s.topic, s.ProcessFunc)
	} else {
		panic("Invalid KernelType")
	}
	sc := GetStreamContainer()
	sc.InitStreamContainer(s, kernel)
	s.containers <- sc
	s.UpdateNofSC(1)
}

// 获取一个StreamContainer
func (s *StreamGroup) GetSC() *StreamContainer {
	return <-s.containers
}

// 放回一个StreamContainer
func (s *StreamGroup) PutSC(sc *StreamContainer) {
	s.containers <- sc
}

// 释放一个StreamContainer
func (s *StreamGroup) ReleaseSC(sc *StreamContainer) {
	if s.GetNofSC() <= s.config.MinNofSC {
		return
	}
	if !sc.IsClosed() {
		sc.Close()
	}
	PutStreamContainer(sc)
	s.UpdateNofSC(-1)
}

func (s *StreamGroup) Close() {
	s.evaluator.Close(s)
}

func parseStreamGroupConfig(config *StreamGroupConfig) *StreamGroupConfig {
	if config == nil {
		return DefaultContainerConfig
	}
	if config.InitialNofSC <= 0 {
		config.InitialNofSC = DefaultContainerConfig.InitialNofSC
	}
	if config.MinNofSC <= 0 {
		config.MinNofSC = DefaultContainerConfig.MinNofSC
	}
	if config.MaxNofSC <= 0 {
		config.MaxNofSC = DefaultContainerConfig.MaxNofSC
	}
	if config.MinNofSC > config.MaxNofSC {
		panic("MinNofSC should be less than MaxNofSC")
	}
	if config.RewardLimit <= 0 {
		config.RewardLimit = DefaultContainerConfig.RewardLimit
	}
	if config.PenaltyLimit <= 0 {
		config.PenaltyLimit = DefaultContainerConfig.PenaltyLimit
	}
	return config
}
