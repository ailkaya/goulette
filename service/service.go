package service

import (
	"context"
	"fmt"

	// "time"

	pb "github.com/ailkaya/goport/broker"
	"github.com/ailkaya/goport/common/config"
	"github.com/ailkaya/goport/common/logger"
	"github.com/ailkaya/goport/core"
	"google.golang.org/grpc"
)

var (
	sendBucketSize int64 = 100
	recvBucketSize int64 = 50
)

type Service struct {
	pb.UnimplementedBrokerServiceServer
	core   *core.Core
	logger logger.ILog
}

func NewService() *Service {
	return &Service{
		core:   core.NewCore(config.Conf.APP.BufferSize, config.Conf.APP.BucketSize),
		logger: logger.Log,
	}
}

func (s *Service) RegisterTopic(ctx context.Context, req *pb.TopicRequest) (*pb.Empty, error) {
	s.core.RegisterTopic(req.GetTopic())
	return nil, nil
}

func (s *Service) SendMessage(stream grpc.BidiStreamingServer[pb.SendMessageRequestOption, pb.Acknowledgment]) error {
	topic, err := s.initSendConn(stream)
	if err != nil {
		s.logger.Errorf("service init: failed to receive message: %v", err)
		return err
	}
	if !s.core.IsTopicExist(topic) {
		return fmt.Errorf("topic %s not exist", topic)
	}

	var (
		handler = core.NewSendHandle(s.core, topic, sendBucketSize)
		req     = &pb.SendMessageRequestOption{}
		resp    = &pb.Acknowledgment{}
		// mid: 当前获取的消息id ackid: 需要ack的消息id retryid: 表示该次请求是否为重试请求, retryid为上次请求的mid
		mid, ackid, retryid int64
		msg                 []byte
	)
	// 初始化成功，开始从client接收消息
	for {
		req, err = stream.Recv()
		if err != nil {
			s.logger.Errorf("failed to receive message: %v", err)
			break
		}
		mid, msg, ackid, retryid = req.GetMid(), req.GetMsg(), req.GetAckid(), req.GetRetryid()
		if ackid != -1 {
			// 防止重复send(从client recv)
			handler.Ack(ackid)
			continue
		} else {
			err = handler.Push(mid, retryid, msg)
			if err != nil {
				s.logger.Errorf("failed to push message: %v", err)
				continue
			}
		}
		resp.Mid = mid
		err = stream.Send(resp) // send ack
		if err != nil {
			s.logger.Errorf("failed to send message: %v", err)
			break
		}
		// fmt.Println("push message:", msg)
	}
	handler.Close()
	return nil
}

func (s *Service) initSendConn(stream grpc.BidiStreamingServer[pb.SendMessageRequestOption, pb.Acknowledgment]) (topic string, e error) {
	msg, err := stream.Recv()
	if err != nil {
		e = err
		return
	}
	topic = msg.GetTopic()
	err = stream.Send(nil) // send ack
	if err != nil {
		e = err
	}
	return
}

func (s *Service) RecvMessage(stream grpc.BidiStreamingServer[pb.RecvMessageRequestOption, pb.Data]) error {
	topic, err := s.initRecvConn(stream)
	if err != nil {
		s.logger.Errorf("service init: failed to receive message: %v", err)
		return err
	}
	if !s.core.IsTopicExist(topic) {
		return fmt.Errorf("topic %s not exist", topic)
	}

	var (
		handler             = core.NewRecvHandle(s.core, topic, recvBucketSize)
		req                 = &pb.RecvMessageRequestOption{}
		resp                = &pb.Data{}
		mid, ackid, retryid int64
	)
	// 开始向client发送消息
	for {
		// fmt.Print(1)
		req, err = stream.Recv()
		if err != nil {
			s.logger.Errorf("failed to receive message: %v", err)
			break
		}
		// fmt.Println(2)
		mid, ackid, retryid = req.GetMid(), req.GetAckid(), req.GetRetryid()
		// fmt.Println(mid, ackid, retryid)
		// 如果ackid不为-1，则表示是ack请求
		if ackid != -1 {
			// 防止重复recv(send到client)
			handler.Ack(ackid)
			// resp.Msg = nil
			continue
		} else {
			resp.Msg, err = handler.Pull(mid, retryid)
			// fmt.Print(3)
			if err != nil {
				s.logger.Errorf("failed to pop message: %v", err)
				continue
			}
		}

		resp.Mid = mid
		if err = stream.Send(resp); err != nil {
			s.logger.Errorf("failed to send message: %v", err)
			break
		}
		// fmt.Println("pull message:", resp.Msg)
	}
	// fmt.Println("recv close")
	handler.Close()
	return nil
}

func (s *Service) initRecvConn(stream grpc.BidiStreamingServer[pb.RecvMessageRequestOption, pb.Data]) (topic string, e error) {
	msg, err := stream.Recv()
	if err != nil {
		e = err
		return
	}
	topic = msg.GetTopic()
	err = stream.Send(nil) // send ack
	if err != nil {
		e = err
	}
	return
}
