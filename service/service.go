package service

import (
	"context"
	"fmt"
	"io"

	// "time"

	// "sync/atomic"

	"github.com/ailkaya/goport/broker"
	"github.com/ailkaya/goport/common/config"
	"github.com/ailkaya/goport/common/logger"
	"github.com/ailkaya/goport/controller"
	"google.golang.org/grpc"
)

type Service struct {
	broker.UnimplementedBrokerServiceServer
	controller controller.IController
	logger     logger.ILog
}

func NewService() *Service {
	return &Service{
		controller: controller.NewController(config.Conf.APP.BufferSize),
		logger:     logger.Log,
	}
}

// 创建一个新的topic(如果已经存在则忽略)
func (s *Service) RegisterTopic(ctx context.Context, req *broker.TopicRequest) (*broker.Empty, error) {
	s.controller.RegisterTopic(req.GetTopic())
	return nil, nil
}

// 向指定topic持续发送消息
func (s *Service) SendMessage(stream grpc.ClientStreamingServer[broker.SendMessageRequest, broker.Empty]) error {
	var topic string
	// 从流中接收消息
	for {
		msg, err := stream.Recv()
		// fmt.Println("Service Received message: ", msg)
		if err == io.EOF {
			fmt.Println("Client closed the stream")
			// 客户端关闭了发送，正常结束
			break
		}
		if err != nil {
			s.logger.Errorf("failed to receive message: %v", err)
			return fmt.Errorf("failed to receive message: %v", err)
		}
		topic = msg.GetTopic() // 获取主题
		// 将消息存储到相应的主题
		// fmt.Println(topic, "Add to channel:", msg.GetMsg())
		err = s.controller.SendMessage(topic, msg.GetMsg())
		if err != nil {
			s.logger.Errorf("failed to send message: %v", err)
			return fmt.Errorf("failed to send message: %v", err)
		}
	}
	// 发送一个空的响应表示成功
	return stream.SendAndClose(&broker.Empty{})
}

// 从指定topic中持续获取消息
func (s *Service) RetrieveMessage(req *broker.TopicRequest, stream grpc.ServerStreamingServer[broker.RetrieveMessageResponse]) error {
	topic := req.GetTopic()
	response := &broker.RetrieveMessageResponse{}
	// time.Sleep(time.Second * 2)
	// fmt.Println("Service receiving message start")
	// stream.Send(nil)
	// stream.Send(nil)
	for i := 0; i < 10; i++ {
		stream.Send(nil)
	}
	// 从控制器中获取消息
	for {
		message, err := s.controller.RetrieveMessage(topic)
		if err != nil {
			s.logger.Errorf("failed to retrieve message: %v", err)
			return fmt.Errorf("failed to retrieve message: %v", err)
		}
		// 创建响应并发送
		response.Msg = message
		// fmt.Println("Service Sending message:", response)

		err = stream.Send(response)
		if err != nil {
			return fmt.Errorf("failed to send message: %v", err)
		}
	}
}
