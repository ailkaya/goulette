package service

import (
	"time"

	pb "github.com/ailkaya/goport/broker"
	"github.com/ailkaya/goport/server/common/logger"
	"github.com/ailkaya/goport/server/handle"
	"google.golang.org/grpc"
)

var producerErrorWaitTime = time.Microsecond * 5

type Producer struct {
	logger  logger.ILog
	stream  grpc.BidiStreamingServer[pb.ProduceRequestOption, pb.Acknowledgment]
	handler *handle.SendHandle
}

func NewProducer(stream grpc.BidiStreamingServer[pb.ProduceRequestOption, pb.Acknowledgment], handler *handle.SendHandle) *Producer {
	return &Producer{
		logger:  logger.Log,
		stream:  stream,
		handler: handler,
	}
}

func (p *Producer) Run() error {
	var (
		err      error = nil
		request        = &pb.ProduceRequestOption{}
		response       = &pb.Acknowledgment{}
		// mid: 当前获取的消息id, retryid: 表示该次请求是否为重试请求, retryid为上次请求的mid
		mid, retryid int32
		msg          []byte
	)
	// 初始化成功，开始从client接收消息
	for {
		// fmt.Print(1)
		request, err = p.stream.Recv()
		// fmt.Print(2)
		if err != nil {
			// fmt.Println("producer recv error:", err)
			p.logger.Errorf("failed to receive message: %v", err)
			if err.Error() == ErrorEOF {
				break
			}
			// fmt.Println(7)
			// 等待一段时间后重试
			time.Sleep(producerErrorWaitTime)
			// continue
			break
		}
		// fmt.Print(3)
		mid, msg, retryid = request.GetMId(), request.GetMsg(), request.GetRetryId()
		err = p.handler.Push(mid, retryid, msg)
		// fmt.Print(4)
		if err != nil {
			p.logger.Errorf("failed to push message: %v", err)
			continue
		}
		response.MId = mid
		err = p.stream.Send(response)
		// fmt.Print(5)
		if err != nil {
			// fmt.Println("producer send error:", err)
			p.logger.Errorf("failed to send message: %v", err)
			if err.Error() == ErrorRPC {
				break
			}
			time.Sleep(producerErrorWaitTime)
			continue
		}
		// fmt.Println("push message:", msg)
	}
	// fmt.Println("producer exit")
	return err
}

func (p *Producer) Ack(mId int32) {
	p.handler.Ack(mId)
}

func (p *Producer) Close() {

}
