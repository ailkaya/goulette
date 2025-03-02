package broker

import (
	"github.com/ailkaya/goulette/broker/common/config"
	"github.com/ailkaya/goulette/broker/core"
	pb "github.com/ailkaya/goulette/broker/pb"
	"google.golang.org/grpc"
	//"google.golang.org/grpc"
)

type Service struct {
	pb.UnimplementedBrokerServiceServer
	core *core.Core
}

func NewService() *Service {
	return &Service{
		core: core.NewCore(config.Conf.APP.BufferSize),
	}
}

func (s *Service) Produce(stream grpc.BidiStreamingServer[pb.ProduceRequest, pb.ProduceResponse]) error {
	init, err := stream.Recv()
	if err != nil {
		return err
	}
	s.core.RegisterTopic(init.InitTopic)
	to := s.core.GetOutput(init.InitTopic)
	handler := newProduceHandler(to, stream)
	handler.start()
	return nil
}

func (s *Service) Consume(stream grpc.BidiStreamingServer[pb.ConsumeRequest, pb.ConsumeResponse]) error {
	init, err := stream.Recv()
	if err != nil {
		return err
	}
	s.core.RegisterTopic(init.InitTopic)
	from := s.core.GetOutput(init.InitTopic)
	handler := newConsumerHandler(from, stream)
	handler.start()
	return nil
}
