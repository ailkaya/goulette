package service

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	pb "github.com/ailkaya/goport/broker"
	// "github.com/ailkaya/goport/client"
	"github.com/ailkaya/goport/server/common/config"
	"github.com/ailkaya/goport/server/common/logger"
	"github.com/ailkaya/goport/server/core"
	"github.com/ailkaya/goport/server/handle"
	"google.golang.org/grpc"
)

// var (
// 	sendBucketSize int32 = 20
// 	recvBucketSize int32 = 20
// )

type Service struct {
	pb.UnimplementedBrokerServiceServer
	core   *core.Core
	logger logger.ILog
	// 每个reception都对应者某个client的连接(特指双向流连接)
	receptions map[int32]IReception
	// 为最新reception分配的id
	latest int32
}

func NewService() *Service {
	return &Service{
		core:       core.NewCore(config.Conf.APP.BufferSize),
		logger:     logger.Log,
		receptions: make(map[int32]IReception),
		latest:     0,
	}
}

func (s *Service) RegisterTopic(ctx context.Context, request *pb.TopicRequest) (*pb.Empty, error) {
	s.core.RegisterTopic(request.GetTopic())
	return nil, nil
}

// 消息ack，独立于正常消息请求，避免被干扰
func (s *Service) Acknowledge(ctx context.Context, request *pb.AcknowledgeRequest) (*pb.Empty, error) {
	// fmt.Println("ack", request.GetAckId(), request.GetAckTarget())
	if request.GetAckTarget() == 0 {
		s.logger.Errorf("ack target id is 0")
		return nil, fmt.Errorf("ack target id is 0")
	}
	s.receptions[request.GetAckTarget()].Ack(request.GetAckId())
	return nil, nil
}

func (s *Service) Close(ctx context.Context, request *pb.CloseRequest) (*pb.Empty, error) {
	if request.GetTarget() == 0 {
		s.logger.Errorf("ack target id is 0")
		return nil, fmt.Errorf("ack target id is 0")
	}
	// fmt.Println("close reception: ", request.GetTarget())
	s.receptions[request.GetTarget()].Close()
	return nil, nil
}

// 从client端接收相关传输信息以初始化
func (s *Service) initProduceConn(stream grpc.BidiStreamingServer[pb.ProduceRequestOption, pb.Acknowledgment]) (topic string, id int32, e error) {
	msg, err := stream.Recv()
	if err != nil {
		e = err
		return
	}
	topic = msg.GetInitTopic()
	id = atomic.AddInt32(&s.latest, 1)
	err = stream.Send(&pb.Acknowledgment{
		InitTargetId: id,
	})
	if err != nil {
		e = err
	}
	return
}

func (s *Service) initConsumeConn(stream grpc.BidiStreamingServer[pb.ConsumeRequestOption, pb.DataResponse]) (topic string, id int32, e error) {
	msg, err := stream.Recv()
	if err != nil {
		e = err
		return
	}
	topic = msg.GetInitTopic()
	id = atomic.AddInt32(&s.latest, 1)
	err = stream.Send(&pb.DataResponse{
		InitTargetId: id,
	})
	if err != nil {
		e = err
	}
	return
}

func (s *Service) Produce(stream grpc.BidiStreamingServer[pb.ProduceRequestOption, pb.Acknowledgment]) error {
	topic, id, err := s.initProduceConn(stream)
	// fmt.Print(1)
	if err != nil {
		s.logger.Errorf("service init: failed to init send connection: %v", err)
		return err
	}
	if !s.core.IsTopicExist(topic) {
		return fmt.Errorf("topic %s not exist", topic)
	}
	// fmt.Print(2)
	handler := handle.NewSendHandle(s.core, topic, config.Conf.APP.SendBucketSizePerStream)
	producer := NewProducer(stream, handler)
	s.receptions[id] = producer
	// return producer.Run()
	// 此处若不sleep,则可能导致bug
	time.Sleep(1 * time.Second)
	err = producer.Run()
	handler.Close()
	return err
}

func (s *Service) Consume(stream grpc.BidiStreamingServer[pb.ConsumeRequestOption, pb.DataResponse]) error {
	topic, id, err := s.initConsumeConn(stream)
	if err != nil {
		s.logger.Errorf("service init: failed to init recv connection: %v", err)
		return err
	}
	if !s.core.IsTopicExist(topic) {
		return fmt.Errorf("topic %s not exist", topic)
	}
	// 用来控制退出consume和handler
	ctx, cancel := context.WithCancel(context.Background())
	handler := handle.NewRecvHandle(s.core, ctx, topic, config.Conf.APP.RecvBucketSizePerStream)
	consumer := NewConsumer(stream, handler, ctx, cancel)
	s.receptions[id] = consumer
	// return consumer.Run()
	time.Sleep(1 * time.Second)
	err = consumer.Run()
	handler.Close()
	return err
}
