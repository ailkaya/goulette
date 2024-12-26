package kernel

import (
	"context"
	"fmt"

	pb "github.com/ailkaya/goport/broker"
	"google.golang.org/grpc"
)

// var (
// 	// errNormal               = fmt.Errorf("")
// 	AckIntervalNumber int32 = 100
// )

// 保证被接收
type SendArrive struct {
	ctx     context.Context
	cancel  context.CancelFunc
	safeEnd []chan bool

	kernelType string
	topic      string
	client     pb.BrokerServiceClient
	stream     grpc.BidiStreamingClient[pb.ProduceRequestOption, pb.Acknowledgment]
	// ack间隔，每处理ackInterval条消息后ack一次
	ackInterval int32
	// ack时需要携带的目标id
	targetId int32
	// 桶漏
	bucket   chan bool
	sendLogs chan *WithData
	recvLogs chan *OnlyID
	// 当前的最新消息id
	mIdx        int32
	ackDemand   chan int32
	retryDemand chan *WithData
	// push func([]byte)
	pull func() []byte
}

// concurrency代表并发能力，即最多同时保留的最大未确认请求数，设置为1时会client与server之间会变成同步通信
func NewSendArrive(kernelType string, topic string, concurrency int32, ackInterval int32, client pb.BrokerServiceClient) (*SendArrive, error) {
	stream, err := client.Produce(context.Background())
	if err != nil {
		return nil, err
	}
	size := concurrency + 10
	ctx, cancel := context.WithCancel(context.Background())

	safeEnd := make([]chan bool, 4)
	for i := range safeEnd {
		safeEnd[i] = make(chan bool, 1)
	}

	return &SendArrive{
		ctx:         ctx,
		cancel:      cancel,
		safeEnd:     safeEnd,
		kernelType:  kernelType,
		topic:       topic,
		client:      client,
		stream:      stream,
		ackInterval: ackInterval,
		bucket:      make(chan bool, concurrency),
		sendLogs:    make(chan *WithData, size),
		recvLogs:    make(chan *OnlyID, size),
		mIdx:        1,
		ackDemand:   make(chan int32, size),
		retryDemand: make(chan *WithData, size),
	}, nil
}

func (s *SendArrive) Need(push func([]byte), pull func() []byte) {
	// 只取用pull
	s.pull = pull
}

func (s *SendArrive) Type() string {
	return s.kernelType
}

func (s *SendArrive) Run() {
	// fmt.Println("send arrive run")
	go s.send()
	go s.receive()
	go s.compare()
	go s.ack()
}

func (s *SendArrive) InitConfig() error {
	s.stream.Send(&pb.ProduceRequestOption{
		InitTopic: s.topic,
	})

	ackch := make(chan error, 1)
	// 初始化超时处理
	if !wait(ackch, func() error {
		ret, err := s.stream.Recv()
		if err != nil {
			return err
		}
		// fmt.Println("send init", ret.InitTargetId)
		s.targetId = ret.InitTargetId
		return nil
	}) {
		return fmt.Errorf("send service config init failed")
	}
	return nil
}

func (s *SendArrive) send() {
	var (
		err     error = nil
		request       = &pb.ProduceRequestOption{}
	)
	for {
		request.Msg = nil

		// fmt.Print(1)
		s.bucket <- true
		// fmt.Print(2)
		if len(s.retryDemand) == 0 {
			// 正常请求
			request.RetryId = -1
			request.Msg = s.pull()
		} else {
			// retry请求
			retry := <-s.retryDemand
			request.RetryId, request.Msg = retry.mId, retry.mData
			PutWithData(retry)
		}

		// fmt.Println(request.RetryId, request.AckId, request.Msg)
		// fmt.Print(3)
		request.MId = s.mIdx
		if err = s.stream.Send(request); err != nil {
			if err.Error() == ErrorStreamClosed {
				// fmt.Println("send done")
				s.safeEnd[0] <- true
				break
			}
			s.retryDemand <- GetWithData(request.MId, request.Msg)
		} else {
			s.sendLogs <- GetWithData(request.MId, request.Msg)
		}
		// fmt.Print(4)

		s.mIdx = Inc(s.mIdx)
	}
}

func (s *SendArrive) receive() {
	var (
		err      error = nil
		response       = &pb.Acknowledgment{}
	)
	for {
		response, err = s.stream.Recv()
		if err != nil {
			if err.Error() == ErrorStreamClosed {
				// fmt.Println("recv done")
				s.safeEnd[1] <- true
				break
			}
			continue
		}
		a := GetOnlyID(response.MId)
		s.recvLogs <- a
	}
}

func (s *SendArrive) compare() {
	var (
		sendLog *WithData = nil
		recvLog *OnlyID   = nil
		ok      bool
	)
	for {
		// fmt.Print(1)
		if sendLog == nil {
			sendLog, ok = <-s.sendLogs
			if !ok {
				// fmt.Println("compare done")
				s.safeEnd[2] <- true
				break
			}
		}
		// fmt.Print(2)
		if recvLog == nil {
			recvLog, ok = <-s.recvLogs
			if !ok {
				// fmt.Println("compare done")
				s.safeEnd[2] <- true
				break
			}
		}
		sendMid, recvMid := sendLog.mId, recvLog.mId
		// fmt.Printf("send mid: %d, recv mid: %d\n", sendMid, recvMid)
		if sendMid == recvMid {
			// fmt.Print(3)
			<-s.bucket
			PutWithData(sendLog)
			PutOnlyID(recvLog)
			sendLog, recvLog = nil, nil
			// 每隔AckIntervalNumber个ack一次
			if sendMid%s.ackInterval == 0 {
				// fmt.Println("need ack", sendMid)
				s.ackDemand <- sendMid
			}
			// continue
		} else if SmallerThan(sendMid, recvMid) {
			// 该拉取请求失败，准备重试
			// 由于不清楚是client->service的请求丢失还是service->client的响应丢失，因此需要重试
			s.retryDemand <- sendLog
			sendLog = nil
		} else {
			panic("send mid > recv mid")
		}
		// fmt.Print(4)
	}
}

func (s *SendArrive) ack() {
	var (
		request = &pb.AcknowledgeRequest{
			AckTarget: s.targetId,
		}
		ok bool
	)
	for {
		request.AckId, ok = <-s.ackDemand
		if !ok {
			// fmt.Println("ack done")
			s.safeEnd[3] <- true
			break
		}
		s.client.Acknowledge(s.ctx, request)
	}
}

// 对于sendLogs, recvLogs, ackDemand, retryDemand, 下面是直接关闭
// 此时将不会等待通道内的数据被消耗完或超时
func (s *SendArrive) Close() {
	s.cancel()
	if len(s.bucket) > 0 {
		// 防止send在bucket满时阻塞导致无法关闭
		<-s.bucket
	}
	// 使退出send和receive
	s.stream.CloseSend()
	// 等待send, receive退出
	<-s.safeEnd[0]
	<-s.safeEnd[1]
	// 使退出compare
	close(s.sendLogs)
	close(s.recvLogs)
	<-s.safeEnd[2]
	// 使退出ack
	close(s.ackDemand)
	<-s.safeEnd[3]

	close(s.bucket)
	close(s.retryDemand)
	for i := range s.safeEnd {
		close(s.safeEnd[i])
	}
	fmt.Println("send arrive kernel close")
}
