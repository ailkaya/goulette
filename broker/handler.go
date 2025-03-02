package broker

import (
	"context"
	"github.com/ailkaya/goulette/broker/common/logger"
	"github.com/ailkaya/goulette/broker/common/utils"
	"github.com/ailkaya/goulette/broker/core"
	"github.com/ailkaya/goulette/broker/pb"
	"google.golang.org/grpc"
	"time"
)

const (
	BucketSize = 1024
	// 初始令牌生成速率设置为50个/s
	TokenGenerateInterval = 20 * time.Microsecond
	MaxWaiting            = 100
)

type produceHandler struct {
	ctx    context.Context
	cancel context.CancelFunc
	topic  string
	core   *core.Core
	stream grpc.BidiStreamingServer[pb.ProduceRequest, pb.ProduceResponse]
	// 令牌桶
	tokenCh chan struct{}
	// 用于更新令牌生成间隔
	modifyTokenGenerateIntervalCh chan time.Duration
	latestMID                     uint64
	log                           logger.ILog
}

func newProduceHandler(core *core.Core, topic string, stream grpc.BidiStreamingServer[pb.ProduceRequest, pb.ProduceResponse]) *produceHandler {
	ctx, cancel := context.WithCancel(context.Background())
	tokenCh := make(chan struct{}, BucketSize)
	modifyTokenGenerateIntervalCh := make(chan time.Duration, 10)
	utils.TokenBucket(ctx, tokenCh, TokenGenerateInterval, modifyTokenGenerateIntervalCh)
	return &produceHandler{
		ctx:                           ctx,
		cancel:                        cancel,
		topic:                         topic,
		core:                          core,
		stream:                        stream,
		tokenCh:                       tokenCh,
		modifyTokenGenerateIntervalCh: modifyTokenGenerateIntervalCh,
		latestMID:                     0,
		log:                           logger.Log,
	}
}

// topic初始化完成后调用
func (ph *produceHandler) start() {
	go ph.send()
	go ph.receive()
}

func (ph *produceHandler) send() {
	for range ph.tokenCh {
		select {
		case <-ph.ctx.Done():
			return
		default:
		}
		err := ph.stream.Send(&pb.ProduceResponse{
			MID:       ph.latestMID,
			AckID:     -1,
			ErrorCode: -1,
		})
		ph.latestMID++
		if err != nil {
			ph.log.Errorf("broker producer handler send error: %s", err.Error())
			return
		}
	}
}

func (ph *produceHandler) receive() {
	for {
		select {
		case <-ph.ctx.Done():
			return
		default:
		}
		resp, err := ph.stream.Recv()
		if err != nil {
			ph.log.Errorf("broker producer handler receive error: %s", err.Error())
			break
		}
		ph.core.Push(ph.topic, resp.Msg)

		// 发送ack
		err = ph.stream.Send(&pb.ProduceResponse{
			AckID:     resp.MID,
			ErrorCode: -1,
		})
		if err != nil {
			ph.log.Errorf("broker producer handler receive error: %s", err.Error())
			break
		}
	}
}

func (ph *produceHandler) close() {
	ph.cancel()
	close(ph.tokenCh)
	close(ph.modifyTokenGenerateIntervalCh)
}

type ConsumerHandler struct {
	ctx         context.Context
	cancel      context.CancelFunc
	topic       string
	core        *core.Core
	waiting     chan uint64
	stream      grpc.BidiStreamingServer[pb.ConsumeRequest, pb.ConsumeResponse]
	latestAcked uint64
	log         logger.ILog
}

func newConsumerHandler(core *core.Core, topic string, stream grpc.BidiStreamingServer[pb.ConsumeRequest, pb.ConsumeResponse]) *ConsumerHandler {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConsumerHandler{
		ctx:         ctx,
		cancel:      cancel,
		topic:       topic,
		core:        core,
		waiting:     make(chan uint64, MaxWaiting),
		stream:      stream,
		latestAcked: 0,
		log:         logger.Log,
	}
}

func (ch *ConsumerHandler) start() {
	go ch.receive()
	go ch.send()
}

func (ch *ConsumerHandler) receive() {
	for {
		select {
		case <-ch.ctx.Done():
			return
		default:
		}
		resp, err := ch.stream.Recv()
		if err != nil {
			ch.log.Errorf("broker consumer handler receive error: %s", err.Error())
			return
		}

		if resp.AckID != -1 {
			ch.latestAcked = resp.AckID
			ch.core.UpdateConsumed(ch.topic, ch.latestAcked)
		} else {
			ch.waiting <- resp.MID
		}
	}
}

func (ch *ConsumerHandler) send() {
	for mID := range ch.waiting {
		select {
		case <-ch.ctx.Done():
			return
		default:
		}
		// TODO: 在尚未ack之前暂时保存已发送但未ack的消息，ack后再彻底删除
		err := ch.stream.Send(&pb.ConsumeResponse{
			MID:       mID,
			Msg:       ch.core.Pull(ch.topic),
			ErrorCode: -1,
		})
		if err != nil {
			ch.log.Errorf("broker consumer handler send error: %s", err.Error())
			return
		}
	}
}

func (ch *ConsumerHandler) close() {
	ch.cancel()
	close(ch.waiting)
}
