package kernel

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	pb "github.com/ailkaya/goport/broker"
	"github.com/ailkaya/goport/client/list"
	"github.com/ailkaya/goport/utils"
	"google.golang.org/grpc"
)

type KernelFunc func(client pb.BrokerServiceClient, ifContinue func(error) bool)

// container核心逻辑
type IKernel interface {
	GetKernelFunc() KernelFunc
	Close()
}

var (
	errNormal              = fmt.Errorf("")
	ServiceConfInitTimeout = 5 * time.Second
	SendLeackyBucketSize   = 100
	RetryMessagePool       = sync.Pool{
		New: func() interface{} {
			return &RetryMessage{}
		},
	}
)

type RetryMessage struct {
	Mid int64
	Msg []byte
}

func getRetryMessage(mid int64, msg []byte) *RetryMessage {
	retryMessage := RetryMessagePool.Get().(*RetryMessage)
	retryMessage.Mid, retryMessage.Msg = mid, msg
	return retryMessage
}

func putRetryMessage(m *RetryMessage) {
	m.Msg = nil
	RetryMessagePool.Put(m)
}

// 保证被service端接收
type SendMessageKernel struct {
	sync.Mutex
	ctx   context.Context
	topic string
	midx  int64
	// 桶漏
	bucket chan bool
	// 消息重发队列
	retry chan *RetryMessage
	// service端消息确认
	ack chan int64
	// 消息来源
	from       chan []byte
	stream     grpc.BidiStreamingClient[pb.SendMessageRequestOption, pb.Acknowledgment]
	ifContinue func(error) bool
}

func NewSendMessageKernel(topic string, from chan []byte) *SendMessageKernel {
	return &SendMessageKernel{
		ctx:    context.Background(),
		topic:  topic,
		midx:   0,
		bucket: make(chan bool, SendLeackyBucketSize),
		ack:    make(chan int64, AckChannelSize),
		retry:  make(chan *RetryMessage, RetryChannelSize),
		from:   from,
	}
}

func (k *SendMessageKernel) GetKernelFunc() KernelFunc {
	return func(client pb.BrokerServiceClient, ifContinue func(error) bool) {
		stream, err := client.SendMessage(context.Background())
		if err != nil {
			ifContinue(io.EOF)
			return
		}
		k.stream = stream
		if err = k.serviceConfInit(); err != nil {
			ifContinue(io.EOF)
			return
		}
		k.ifContinue = ifContinue
		sendList, recvList := list.NewListForMsg(), list.NewListForMid()
		go k.send(sendList)
		go k.recv(recvList)
		go k.compare(sendList, recvList)
	}
}

// 初始化服务器端接收方配置
func (k *SendMessageKernel) serviceConfInit() error {
	k.stream.Send(&pb.SendMessageRequestOption{
		Topic: k.topic,
	})

	ackch := make(chan error, 1)
	if !wait(ackch, func() error {
		_, err := k.stream.Recv()
		return err
	}) {
		return fmt.Errorf("send service config init failed")
	}
	return nil
}

func (k *SendMessageKernel) send(l *list.ListForMsg) {
	var (
		err error = nil
		req       = &pb.SendMessageRequestOption{}
	)
	for {
		if !k.ifContinue(err) {
			break
		}
		k.bucket <- true

		if len(k.retry) == 0 {
			req.Retryid = -1
			// 是否需要ack, ack请求不需要check
			select {
			case ackid := <-k.ack:
				req.Ackid = ackid
			default:
				// 正常发送
				req.Ackid, req.Retryid, req.Msg = -1, -1, <-k.from
			}

		} else {
			req.Ackid = -1
			// 是否有请求需要重试
			select {
			case msg := <-k.retry:
				req.Retryid, req.Msg = msg.Mid, msg.Msg
				putRetryMessage(msg)
			default:
				// 正常发送
				req.Retryid, req.Msg = -1, <-k.from
			}
		}

		req.Mid = k.midx
		if err = k.stream.Send(req); err != nil {
			k.retry <- getRetryMessage(req.Mid, req.Msg)
			// continue
		} else if req.Ackid == -1 {
			// 排除ack请求
			l.Push(req.Mid, req.Msg)
		}
		k.midx = utils.Inc(k.midx)
	}
}

func (k *SendMessageKernel) recv(l *list.ListForMid) {
	var (
		err  error = nil
		resp       = &pb.Acknowledgment{}
	)
	for {
		if !k.ifContinue(err) {
			break
		}
		resp, err = k.stream.Recv()
		if err != nil {
			continue
		}
		l.Push(resp.Mid)
	}
}

func (k *SendMessageKernel) compare(sendList *list.ListForMsg, recvList *list.ListForMid) {
	var err error = nil
	for {
		if !k.ifContinue(err) {
			break
		}
		sendMid, recvMid := sendList.GetHeadValue(), recvList.GetHeadValue()
		// fmt.Printf("send compare: %d-%d\n", sendMid, recvMid)
		// 防止mod后recvMid比sendMid小
		<-k.bucket
		if sendMid == recvMid {
			list.PutNodeOfMsg(sendList.Pop())
			list.PutNodeOfMid(recvList.Pop())
			// 每隔AckIntervalNumber个ack一次
			if sendMid%AckIntervalNumber == 0 {
				k.ack <- sendMid
			}
			err = nil
			continue
		} else if utils.SmallerThan(sendMid, recvMid) {
			node := sendList.Pop()
			k.retry <- getRetryMessage(node.Mid, node.Msg)
			// k.from <- node.Msg
			list.PutNodeOfMsg(node)
		} else {
			panic("send mid > recv mid")
		}
		// fmt.Print(1)
		err = errNormal
	}
}

func (k *SendMessageKernel) Close() {
	close(k.bucket)
	close(k.from)
	if k.stream != nil {
		k.Lock()
		k.stream.CloseSend()
		k.Unlock()
	}
}

func wait(ackch chan error, execFunc func() error) bool {
	go func() {
		ackch <- execFunc()
	}()
	select {
	case <-time.After(ServiceConfInitTimeout):
		return false
	case err := <-ackch:
		if err != nil {
			return false
		}
		return true
	}
}
