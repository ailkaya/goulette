package service

import (
	"context"
	"time"

	pb "github.com/ailkaya/goport/broker"
	"github.com/ailkaya/goport/server/common/logger"
	"github.com/ailkaya/goport/server/handle"
	"google.golang.org/grpc"
)

var consumerErrorWaitTime = time.Microsecond * 5

type Consumer struct {
	ctx     context.Context
	cancel  context.CancelFunc
	logger  logger.ILog
	stream  grpc.BidiStreamingServer[pb.ConsumeRequestOption, pb.DataResponse]
	handler *handle.RecvHandle
}

func NewConsumer(stream grpc.BidiStreamingServer[pb.ConsumeRequestOption, pb.DataResponse], handler *handle.RecvHandle, ctx context.Context, cancel context.CancelFunc) *Consumer {
	// ctx, cancel := context.WithCancel(ctx)
	return &Consumer{
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger.Log,
		stream:  stream,
		handler: handler,
	}
}

func (c *Consumer) Run() error {
	var (
		err          error = nil
		req                = &pb.ConsumeRequestOption{}
		resp               = &pb.DataResponse{}
		mid, retryid int32
	)
	// 开始向client发送消息
	for {
		req, err = c.stream.Recv()
		// fmt.Print(1)
		if err != nil {
			// fmt.Println("consumer recv error:", err)
			c.logger.Errorf("failed to receive message: %v", err)
			// 若err为EOF，则表示client已关闭连接，退出循环
			if err.Error() == ErrorEOF {
				break
			}
			// fmt.Println(7)
			// 等待一段时间后重试
			time.Sleep(consumerErrorWaitTime)
			// continue
			break
		}
		// fmt.Print(2)
		mid, retryid = req.GetMId(), req.GetRetryId()
		resp.Msg, err = c.handler.Pull(mid, retryid)
		if err != nil {
			c.logger.Errorf("failed to pop message: %v", err)
			continue
		}
		// fmt.Print(3)
		resp.MId = mid
		if err = c.stream.Send(resp); err != nil {
			// fmt.Println("consumer send error:", err)
			c.logger.Errorf("failed to send message: %v", err)
			if err.Error() == ErrorRPC {
				break
			}
			time.Sleep(consumerErrorWaitTime)
			continue
		}
		// fmt.Print(4)
		// fmt.Println("pull message:", resp.Msg)
	}
	// fmt.Println("consumer exit")
	return err
}

func (c *Consumer) Ack(mId int32) {
	c.handler.Ack(mId)
}

func (c *Consumer) Close() {
	// fmt.Print(666)
	c.handler.UnRegister()
	c.cancel()
}
