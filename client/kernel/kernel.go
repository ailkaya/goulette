package kernel

import (
	"context"
	pb "github.com/ailkaya/goport/broker"
	"google.golang.org/grpc"
)

type IRequest interface {
	GenerateSendRequest()
	StreamRecv()
	IncMIdx()
	UpdateSend()
	UpdateRecv()
	GetSendMId() int32
	GetRecvMId() int32
	PutSendLogBack()
	PutRecvLogBack()
	SetSendLogNil()
	SetRecvLogNil()
	PutSendToRetry()
	Close()
}

type Kernel struct {
	ctx    context.Context
	cancel context.CancelFunc
	// 函数安全退出
	safeEnd chan bool

	kernelType string
	topic      string
	stream     grpc.BidiStreamingClient[pb.ProduceRequestOption, pb.Acknowledgment]

	// 当前的最新消息id
	mIdx int32
	// 桶漏
	bucket    chan bool
	sendLogs  chan *WithData
	recvLogs  chan *OnlyID
	ackDemand chan int32
	// retryDemand chan *WithData
	// 推送和拉取消息
	push func([]byte)
	pull func() []byte
}
