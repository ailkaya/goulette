package client

import (
	// "context"
	// "fmt"

	pb "github.com/ailkaya/goport/broker"
	// "google.golang.org/grpc"
)

// implement: IContainer
type StreamContainer struct {
	client        pb.BrokerServiceClient
	kernel        IKernel
	preProcessor  IPreProcessor
	postProcessor IPostProcessor
}

func NewStreamContainer(client pb.BrokerServiceClient) *StreamContainer {
	return &StreamContainer{
		client: client,
	}
}

func (s *StreamContainer) Cite(kernel IKernel, preProcessor IPreProcessor, postProcessor IPostProcessor) {
	s.kernel = kernel
	s.preProcessor = preProcessor
	s.postProcessor = postProcessor
}

func (s *StreamContainer) Run() error {
	// fmt.Println("7")
	if err := s.kernel.InitConfig(); err != nil {
		return err
	}
	// fmt.Println("8")
	// go s.kernel.Send(s.PreProcess())
	// go s.kernel.Receive(s.PostProcess())
	// go s.kernel.Compare()
	s.kernel.Need(s.PostProcess(), s.PreProcess())
	s.kernel.Run()
	// fmt.Println("9")
	return nil
}

func (s *StreamContainer) Close() {
	s.kernel.Close()
	// 等待服务停止后再关闭 前/后置处理器
	if s.preProcessor != nil {
		s.preProcessor.Close()
	}
	if s.postProcessor != nil {
		s.postProcessor.Close()
	}
}

func (s *StreamContainer) PreProcess() func() []byte {
	if s.preProcessor == nil {
		return nil
	}
	return s.preProcessor.Process
}

func (s *StreamContainer) PostProcess() func([]byte) {
	if s.postProcessor == nil {
		return nil
	}
	return s.postProcessor.Process
}
