package kernel

import (
	"context"
	"fmt"

	pb "github.com/ailkaya/goport/broker"
	"google.golang.org/grpc"
)

type ReceiveArrive struct {
	ctx    context.Context
	cancel context.CancelFunc
	// 用来实现各协程安全退出
	safeEnd []chan bool

	kernelType string
	topic      string
	client     pb.BrokerServiceClient
	stream     grpc.BidiStreamingClient[pb.ConsumeRequestOption, pb.DataResponse]
	// ack间隔，每处理ackInterval条消息后ack一次
	ackInterval int32
	// ack时需要携带的目标id
	targetId int32
	// 桶漏
	bucket   chan bool
	sendLogs chan *OnlyID
	recvLogs chan *OnlyID
	// 当前的最新消息id
	mIdx        int32
	ackDemand   chan int32
	retryDemand chan *OnlyID
	// push func([]byte)
	push func([]byte)
}

func NewReceiveArrive(kernelType string, topic string, concurrency int32, ackInterval int32, client pb.BrokerServiceClient) (*ReceiveArrive, error) {
	stream, err := client.Consume(context.Background())
	if err != nil {
		return nil, err
	}
	// fmt.Println(concurrency)
	size := concurrency + 10
	ctx, cancel := context.WithCancel(context.Background())

	safeEnd := make([]chan bool, 4)
	for i := range safeEnd {
		safeEnd[i] = make(chan bool, 1)
	}

	return &ReceiveArrive{
		ctx:         ctx,
		cancel:      cancel,
		safeEnd:     safeEnd,
		kernelType:  kernelType,
		topic:       topic,
		client:      client,
		stream:      stream,
		ackInterval: ackInterval,
		bucket:      make(chan bool, concurrency),
		sendLogs:    make(chan *OnlyID, size),
		recvLogs:    make(chan *OnlyID, size),
		mIdx:        1,
		ackDemand:   make(chan int32, size),
		retryDemand: make(chan *OnlyID, size),
	}, nil
}

func (r *ReceiveArrive) Need(push func([]byte), pull func() []byte) {
	// 只取用push
	r.push = push
}

func (r *ReceiveArrive) Type() string {
	return r.kernelType
}

func (r *ReceiveArrive) Run() {
	go r.send()
	go r.receive()
	go r.compare()
	go r.ack()
}

func (r *ReceiveArrive) InitConfig() error {
	r.stream.Send(&pb.ConsumeRequestOption{
		InitTopic: r.topic,
	})

	ackch := make(chan error, 1)
	// 初始化超时处理
	if !wait(ackch, func() error {
		ret, err := r.stream.Recv()
		if err != nil {
			return err
		}
		// fmt.Println("receive init", ret.InitTargetId)
		r.targetId = ret.InitTargetId
		return nil
	}) {
		return fmt.Errorf("receive service config init failed")
	}
	return nil
}

func (r *ReceiveArrive) send() {
	var (
		request       = &pb.ConsumeRequestOption{}
		err     error = nil
	)
	for {
		r.bucket <- true
		// 为防止ackid > retry的mid, 导致service端需要重发的data提前被丢弃, 因此需要在retry为空时ack
		// retry不为空时, 可通过retry的mid在service端pop掉已经成功发送的data
		if len(r.retryDemand) == 0 {
			request.RetryId = -1
			// 是否需要ack, ack请求不check
		} else {
			// 是否有请求需要重试
			retry := <-r.retryDemand
			request.RetryId = retry.mId
			PutOnlyID(retry)
		}

		request.MId = r.mIdx
		if err = r.stream.Send(request); err != nil {
			if err.Error() == ErrorStreamClosed {
				r.safeEnd[0] <- true
				break
			}
			r.retryDemand <- GetOnlyID(request.MId)
			// continue
		} else {
			r.sendLogs <- GetOnlyID(request.MId)
		}
		r.mIdx = Inc(r.mIdx)
	}
}

func (r *ReceiveArrive) receive() {
	var (
		response       = &pb.DataResponse{}
		err      error = nil
	)
	for {
		response, err = r.stream.Recv()
		if err != nil {
			if err.Error() == ErrorStreamClosed {
				// r.safeEnd <- true
				r.safeEnd[1] <- true
				break
			}
			continue
		}
		// fmt.Println("receive mid: ", response.MId)
		a := GetOnlyID(response.MId)
		// a := &OnlyID{mId: response.MId}
		r.recvLogs <- a
		r.push(response.Msg)
	}
}

func (r *ReceiveArrive) compare() {
	// var err error = nil
	var (
		sendLog *OnlyID = nil
		recvLog *OnlyID = nil
		ok      bool
	)
	for {
		// 由于receive请求消息时，会提前发送请求，等待响应，所以这里无法直接通过判断sendLogs是否关闭来判断是否退出
		// 因而需要通过判断recvLogs是否关闭来判断是否退出，但recvLogs需要在receive退出之后才能关闭
		// 由此产生的顺序问题需要通过额外引入一个通道来解决
		if sendLog == nil {
			sendLog, ok = <-r.sendLogs
			if !ok {
				r.safeEnd[2] <- true
				break
			}
		}
		// fmt.Print(1)
		if recvLog == nil {
			recvLog, ok = <-r.recvLogs
			if !ok {
				r.safeEnd[2] <- true
				break
			}
		}
		// fmt.Print(2)
		sendMid, recvMid := sendLog.mId, recvLog.mId
		// fmt.Printf("send mid: %d, recv mid: %d\n", sendMid, recvMid)
		if sendMid == recvMid {
			// fmt.Print(3)
			<-r.bucket
			PutOnlyID(sendLog)
			PutOnlyID(recvLog)
			sendLog, recvLog = nil, nil
			// 每隔AckIntervalNumber个ack一次
			if sendMid%r.ackInterval == 0 {
				r.ackDemand <- sendMid
			}
			// fmt.Print(4)
			// continue
		} else if SmallerThan(sendMid, recvMid) {
			// 该拉取请求失败，准备重试
			// 由于不清楚是client->service的请求丢失还是service->client的响应丢失，因此需要重试
			r.retryDemand <- sendLog
			sendLog = nil
		} else {
			panic("send mid > recv mid")
		}
	}
}

func (r *ReceiveArrive) ack() {
	var (
		request = &pb.AcknowledgeRequest{
			AckTarget: r.targetId,
		}
		ok bool
	)
	for {
		request.AckId, ok = <-r.ackDemand
		if !ok {
			r.safeEnd[3] <- true
			return
		}
		r.client.Acknowledge(r.ctx, request)
	}
}

func (r *ReceiveArrive) Close() {
	// fmt.Println("receive arrive kernel close start")
	r.cancel()
	if len(r.bucket) > 0 {
		// 防止send在bucket满时阻塞导致无法关闭
		<-r.bucket
	}
	r.client.Close(context.Background(), &pb.CloseRequest{
		Target: r.targetId,
	})
	// 使退出send和receive
	r.stream.CloseSend()
	// 等待send, receive退出
	<-r.safeEnd[0]
	<-r.safeEnd[1]
	// 使退出compare
	close(r.sendLogs)
	close(r.recvLogs)
	<-r.safeEnd[2]
	// 使退出ack
	close(r.ackDemand)
	<-r.safeEnd[3]

	close(r.bucket)
	close(r.retryDemand)
	for i := range r.safeEnd {
		close(r.safeEnd[i])
	}
	fmt.Println("receive arrive kernel close")
}
