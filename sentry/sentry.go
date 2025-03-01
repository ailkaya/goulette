package sentry

import (
	"context"
	"github.com/ailkaya/goulette/sentry/pb"
)

type Sentry struct {
	pb.UnimplementedSentryServiceServer
	lb ILoadBalancer
	//existTopics map[string]bool
}

func NewService(lb ILoadBalancer) *Sentry {
	return &Sentry{
		lb: lb,
		//existTopics: make(map[string]bool),
	}
}

func (s *Sentry) RegisterTopic(ctx context.Context, param *pb.OnlyTopic) (*pb.Empty, error) {
	s.lb.RegisterTopic(param.Topic)
	return nil, nil
}

func (s *Sentry) RegisterBroker(ctx context.Context, param *pb.RegisterBrokerParams) (*pb.Empty, error) {
	s.lb.RegisterBroker(&ConsistentHashRegisterBrokerParams{
		Addr:         param.Addr,
		WeightFactor: float64(param.WeightFactor),
	})
	return nil, nil
}

func (s *Sentry) GetBrokersForProducer(ctx context.Context, param *pb.OnlyTopic) (*pb.QueryResp, error) {
	resp, err := s.lb.GetBrokers(&ConsistentHashGetBrokersParamsOfProducer{
		Topic: param.Topic,
	})
	if err != nil {
		return nil, err
	}
	return &pb.QueryResp{
		Brokers: resp,
	}, nil
}

func (s *Sentry) GetBrokersForConsumer(ctx context.Context, param *pb.AddrAndTopic) (*pb.QueryResp, error) {
	resp, err := s.lb.GetBrokers(&ConsistentHashGetBrokersParamsOfConsumer{
		Topic:        param.Topic,
		ConsumerAddr: param.Addr,
	})
	if err != nil {
		return nil, err
	}
	return &pb.QueryResp{
		Brokers: resp,
	}, nil
}

// 仅broker可调用，用于将自己设置为producer不可见状态
func (s *Sentry) LogOff(ctx context.Context, param *pb.OnlyAddr) (*pb.Empty, error) {
	s.lb.RemoveBroker(param.Addr)
	return nil, nil
}
