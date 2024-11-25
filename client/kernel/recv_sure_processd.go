package kernel

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"

	pb "github.com/ailkaya/goport/broker"
	"github.com/ailkaya/goport/client/list"
	"github.com/ailkaya/goport/utils"
	"google.golang.org/grpc"
)

var (
	dataProcessRetry = 3
)

// 保证消息到达client后被成功处理
type RecvWithSureProcessed struct {
	sync.Mutex
	ctx   context.Context
	topic string
	// 下一个发送的消息的id
	midx int64
	// 桶漏
	bucket chan bool
	// 数据处理函数
	process func([]byte) error
	// 请求重试
	retry chan int64
	// service端消息确认
	ack chan int64
	// 双向流
	stream grpc.BidiStreamingClient[pb.RecvMessageRequestOption, pb.Data]
	// 判断是否继续
	ifContinue func(error) bool
}

func NewRecvWithSureProcessed(topic string, process func([]byte) error) *RecvWithSureProcessed {
	return &RecvWithSureProcessed{
		ctx:     context.Background(),
		topic:   topic,
		midx:    0,
		bucket:  make(chan bool, RecvLeackyBucketSize),
		ack:     make(chan int64, AckChannelSize),
		retry:   make(chan int64, RetryChannelSize),
		process: process,
	}
}

func (k *RecvWithSureProcessed) GetKernelFunc() KernelFunc {
	return func(client pb.BrokerServiceClient, ifContinue func(error) bool) {
		stream, err := client.RecvMessage(context.Background())
		if err != nil {
			ifContinue(io.EOF)
			return
		}
		k.stream = stream
		if err = k.serviceConfInit(stream); err != nil {
			ifContinue(io.EOF)
			return
		}
		k.ifContinue = ifContinue
		sendList, recvList := list.NewListForMid(), list.NewListForMid()
		go k.send(sendList)
		go k.recv(recvList)
		go k.compare(sendList, recvList)
	}
}

func (k *RecvWithSureProcessed) serviceConfInit(stream grpc.BidiStreamingClient[pb.RecvMessageRequestOption, pb.Data]) error {
	stream.Send(&pb.RecvMessageRequestOption{
		Topic: k.topic,
	})

	ackch := make(chan error, 1)
	if !wait(ackch, func() error {
		_, err := stream.Recv()
		return err
	}) {
		return fmt.Errorf("receive service config init failed")
	}
	return nil
}

func (k *RecvWithSureProcessed) send(l *list.ListForMid) {
	var (
		req       = &pb.RecvMessageRequestOption{}
		err error = nil
	)
	for {
		if !k.ifContinue(err) {
			return
		}
		k.bucket <- true

		// 为防止ackid > retry的mid, 导致service端需要重发的data提前被丢弃, 因此需要在retry为空时ack
		// retry不为空时, 可通过retry的mid在service端pop掉已经成功发送的data
		if len(k.retry) == 0 {
			req.Retryid = -1
			// 是否需要ack, ack请求不check
			select {
			case ackid := <-k.ack:
				req.Ackid = ackid
			default:
				req.Ackid = -1
			}
		} else {
			req.Ackid = -1
			// 是否有请求需要重试
			select {
			case mid := <-k.retry:
				req.Retryid = mid
			default:
				req.Retryid = -1
			}
		}

		req.Mid = k.midx
		if err = k.stream.Send(req); err != nil {
			continue
		} else if req.Ackid == -1 {
			// 排除ack请求
			l.Push(req.Mid)
		}
		k.midx = utils.Inc(k.midx)
	}
}

func (k *RecvWithSureProcessed) recv(l *list.ListForMid) {
	var (
		resp       = &pb.Data{}
		err  error = nil
	)
	for {
		if !k.ifContinue(err) {
			return
		}
		resp, err = k.stream.Recv()
		if err != nil {
			continue
		}
		// fmt.Println("recv:", resp.Msg)
		// 等待数据处理完成
		i := 0
		for ; i < dataProcessRetry; i++ {
			if err = k.process(resp.Msg); err == nil {
				l.Push(resp.Mid)
				break
			}
		}
		if i == dataProcessRetry {
			// 处理失败, 等待从service端重新拉取处理
			log.Printf("process data failed after retry %d times, err: %v", dataProcessRetry, err)
		}
	}
}

func (k *RecvWithSureProcessed) compare(sendList *list.ListForMid, recvList *list.ListForMid) {
	var err error = nil
	for {
		if !k.ifContinue(err) {
			break
		}
		sendMid, recvMid := sendList.GetHeadValue(), recvList.GetHeadValue()
		<-k.bucket
		if sendMid == recvMid {
			list.PutNodeOfMid(sendList.Pop())
			list.PutNodeOfMid(recvList.Pop())
			// 每隔AckIntervalNumber个ack一次
			if sendMid%AckIntervalNumber == 0 {
				k.ack <- sendMid
			}
			err = nil
			continue
		} else if utils.SmallerThan(sendMid, recvMid) {
			node := sendList.Pop()
			// 该拉取请求失败，准备重试
			k.retry <- node.Mid
			list.PutNodeOfMid(node)
		} else {
			panic("send mid > recv mid")
		}
		// fmt.Print(1)
		err = errNormal
	}
}

func (k *RecvWithSureProcessed) Close() {
	close(k.bucket)
	if k.stream != nil {
		k.Lock()
		k.stream.CloseSend()
		k.Unlock()
	}
}
