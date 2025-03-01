package goulette

import (
	"context"
	brokerPb "github.com/ailkaya/goulette/broker/pb"
	"github.com/ailkaya/goulette/sentry"
	sentryPb "github.com/ailkaya/goulette/sentry/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

const (
	EndPoint        = "127.0.0.1:2379"
	ReFreshInterval = time.Second * 20
	BufSize         = 64
	MaxWaiting      = 10
)

type Producer struct {
	//ctx    context.Context
	//cancel context.CancelFunc
	trs      []*pTrans
	topic    string
	buf      chan []byte
	pullReqs chan uint64
}

type pTrans struct {
	stream grpc.BidiStreamingClient[brokerPb.ProduceRequest, brokerPb.ProduceResponse]
	ch     chan uint64
}

func NewProducer(topic string) *Producer {
	//ctx, cancel := context.WithCancel(context.Background())
	producer := &Producer{
		trs:   make([]*pTrans, 0, 4),
		topic: topic,
		buf:   make(chan []byte, BufSize),
	}

	producer.connect()
	// TODO: broker信息更新时，保留旧的broker连接并将其读取优先级设置的更高，同时兼顾新连接
	//go producer.refresh()
	return producer
}

// 定期重新拉取相关信息
func (p *Producer) refresh() {
	ticker := time.NewTicker(ReFreshInterval)
	for _ = range ticker.C {
		err := p.connect()
		// TODO: 完善错误处理
		if err != nil {
			break
		}
	}
}

func (p *Producer) connect() error {
	leaderUrl, err := sentry.GetEtcdLeader([]string{EndPoint})
	if err != nil {
		return err
	}
	leaderConn, err := grpc.NewClient(leaderUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	leaderClient := sentryPb.NewSentryServiceClient(leaderConn)
	resp, err := leaderClient.GetBrokersForProducer(context.Background(), &sentryPb.OnlyTopic{
		Topic: p.topic,
	})
	if err != nil {
		return err
	}

	for _, addr := range RemoveDuplicates(resp.Brokers) {
		conn, err := grpc.NewClient(addr)
		if err != nil {
			return err
		}
		stream, err := brokerPb.NewBrokerServiceClient(conn).Produce(context.Background())
		if err != nil {
			return err
		}

		reqCh := make(chan uint64, MaxWaiting)
		p.trs = append(p.trs, &pTrans{stream: stream, ch: reqCh})

		go p.send(reqCh, stream)
		// TODO: 完善receive
		//go p.receive(stream)
	}
	return nil
}

func (p *Producer) Produce(msg []byte) {
	p.buf <- msg
}

func (p *Producer) send(req chan uint64, stream grpc.BidiStreamingClient[brokerPb.ProduceRequest, brokerPb.ProduceResponse]) {
	for mID := range req {
		err := stream.Send(&brokerPb.ProduceRequest{
			MID: mID,
			Msg: <-p.buf,
		})
		// TODO: 错误处理
		if err != nil {
			break
		}
	}
}

func (p *Producer) receive(stream grpc.BidiStreamingClient[brokerPb.ProduceRequest, brokerPb.ProduceResponse]) {
	// TODO: 接收ack
}

func (p *Producer) Close() {
	for _, tr := range p.trs {
		tr.stream.CloseSend()
		close(tr.ch)
	}
}

type Consumer struct {
	trs      []*cTrans
	topic    string
	buf      chan []byte
	bucket   chan struct{}
	latestID uint64
}

type cTrans struct {
	ctx    context.Context
	cancel context.CancelFunc
	stream grpc.BidiStreamingClient[brokerPb.ConsumeRequest, brokerPb.ConsumeResponse]
}

func NewConsumer(topic string) *Consumer {
	consumer := &Consumer{
		trs:    make([]*cTrans, 0, 4),
		topic:  topic,
		buf:    make(chan []byte, BufSize),
		bucket: make(chan struct{}, MaxWaiting),
	}
	return consumer
}

func (c *Consumer) refresh() {}

func (c *Consumer) connect() error {
	leaderUrl, err := sentry.GetEtcdLeader([]string{EndPoint})
	if err != nil {
		return err
	}
	leaderConn, err := grpc.NewClient(leaderUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	leaderClient := sentryPb.NewSentryServiceClient(leaderConn)
	resp, err := leaderClient.GetBrokersForProducer(context.Background(), &sentryPb.OnlyTopic{
		Topic: c.topic,
	})
	if err != nil {
		return err
	}

	for _, addr := range RemoveDuplicates(resp.Brokers) {
		conn, err := grpc.NewClient(addr)
		if err != nil {
			return err
		}
		stream, err := brokerPb.NewBrokerServiceClient(conn).Consume(context.Background())
		if err != nil {
			return err
		}

		//reqCh := make(chan int64, MaxWaiting)
		ctx, cancel := context.WithCancel(context.Background())
		c.trs = append(c.trs, &cTrans{
			ctx:    ctx,
			cancel: cancel,
			stream: stream,
		})

		go c.send(ctx, stream)
		go c.receive(ctx, stream)
	}
	return nil
}

func (c *Consumer) send(ctx context.Context, stream grpc.BidiStreamingClient[brokerPb.ConsumeRequest, brokerPb.ConsumeResponse]) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		<-c.bucket
		err := stream.Send(&brokerPb.ConsumeRequest{
			MID:   c.latestID,
			AckID: -1,
		})
		c.latestID++
		if err != nil {
			break
		}
	}
}

func (c *Consumer) receive(ctx context.Context, stream grpc.BidiStreamingClient[brokerPb.ConsumeRequest, brokerPb.ConsumeResponse]) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		resp, err := stream.Recv()
		// TODO: 错误处理
		if err != nil {
			break
		}
		c.buf <- resp.Msg

		err = stream.Send(&brokerPb.ConsumeRequest{
			AckID: resp.MID,
		})
		if err != nil {
			break
		}
	}
}

func (c *Consumer) Consume() []byte {
	return <-c.buf
}

func (c *Consumer) Close() {
	for _, tr := range c.trs {
		tr.cancel()
		tr.stream.CloseSend()
	}
	close(c.buf)
}

func RemoveDuplicates(input []string) []string {
	// 使用 map 存储唯一字符串
	unique := make(map[string]struct{})
	result := []string{}

	for _, str := range input {
		if _, exists := unique[str]; !exists {
			unique[str] = struct{}{}     // 将字符串添加到 map 中
			result = append(result, str) // 同时添加到结果切片
		}
	}

	return result
}
