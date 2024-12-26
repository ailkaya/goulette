package handle

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ailkaya/goport/client/kernel"
	"github.com/ailkaya/goport/server/core"
)

var (
	taskChannelSize = 1000
	ackLoopInterval = time.Millisecond * 1
)

// 负责暂存从client端接收到的请求信息，方便重发和定位错误
type SendHandle struct {
	ctx           context.Context
	ctxCancelFunc context.CancelFunc
	ackFisnished  chan bool
	host          *core.Core
	topic         string
	// 最新处理过的消息的id
	latestMid int32
	// ack任务通道, 传入latestMid, 调用ack函数
	ackTasks chan int32
	// 等待send ack的消息链表, 最大长度由client端bucket的大小决定
	waitAck chan *kernel.OnlyID
	// 避免并发问题的锁(ack时), 0代表此时没有协程正在进行ack操作
	// 防止ifHasSended与sendAck同时操作ack链表进行pop操作
	// 虽然ifHasSended的mid一定比sendAck的更大, 更后到达
	// 但进行ifHasSended时可能之前的ack还没有执行完毕, 从而导致并发问题
	waitAckMutex int64
}

func NewSendHandle(host *core.Core, topic string, bucketSize int64) *SendHandle {
	ctx, ctxCancelFunc := context.WithCancel(context.Background())
	s := &SendHandle{
		ctx:           ctx,
		ctxCancelFunc: ctxCancelFunc,
		ackFisnished:  make(chan bool),
		host:          host,
		topic:         topic,
		latestMid:     0,
		ackTasks:      make(chan int32, taskChannelSize),
		waitAck:       make(chan *kernel.OnlyID, bucketSize+10),
	}
	go s.ackLoop()
	return s
}

func (s *SendHandle) ackLoop() {
	running := true
	for running {
		select {
		case <-s.ctx.Done():
			running = false
		default:
			mid := <-s.ackTasks
			s.ack(mid)
			// 方便ifHasSended获取锁
			// time.Sleep(ackLoopInterval)
		}
	}
	s.ackFisnished <- true
}

func (s *SendHandle) Push(mid, retryMid int32, msg []byte) error {
	// 放入ack队列中等待ack
	s.waitAck <- kernel.GetOnlyID(mid)
	// 判断是否是重试请求以及已经接受过
	if retryMid != -1 && s.ifHasSended(retryMid) {
		return nil
	}
	// fmt.Print(9)
	s.host.Push(s.topic, msg)
	s.latestMid = kernel.Max(s.latestMid, mid)
	return nil
}

// 是否已经从client接收过
// 该函数只由Push调用
func (s *SendHandle) ifHasSended(retryMid int32) bool {
	// 一直尝试获取锁，直到获取成功
	for !atomic.CompareAndSwapInt64(&s.waitAckMutex, 0, 1) {
	}
	// 释放锁
	defer atomic.CompareAndSwapInt64(&s.waitAckMutex, 1, 0)
	if len(s.waitAck) == 0 {
		return false
	}
	// fmt.Print(8)

	var ackLog *kernel.OnlyID
	for {
		if ackLog == nil {
			ackLog = <-s.waitAck
		}
		mid := ackLog.GetMId()
		if mid < retryMid {
			// Pop得到的对象需要手动放回Pool中
			kernel.PutOnlyID(ackLog)
		} else if mid == retryMid {
			// 拿到未发送成功的消息返回
			kernel.PutOnlyID(ackLog)
			return true
		} else {
			// 到达mid > retryMid时说明retryMid的那次请求没有到达service, 说明没有被接受过
			return false
		}
		ackLog = nil
	}
}

func (s *SendHandle) Ack(latestMid int32) {
	// fmt.Println(latestMid)
	s.ackTasks <- latestMid
}

func (s *SendHandle) ack(latestMid int32) {
	for !atomic.CompareAndSwapInt64(&s.waitAckMutex, 0, 1) {
	}
	defer atomic.CompareAndSwapInt64(&s.waitAckMutex, 1, 0)
	// fmt.Println("ack", latestMid)
	var ackLog *kernel.OnlyID
	// 将成功发送的消息从ackdata中移除
	for {
		if ackLog == nil {
			ackLog = <-s.waitAck
		}
		mid := ackLog.GetMId()
		// fmt.Print(mid)
		// Ack的leatestMid一定存在于waitAck中
		if mid <= latestMid {
			// Pop得到的对象需要手动放回Pool中
			kernel.PutOnlyID(ackLog)
		} else {
			break
		}
		ackLog = nil
	}
}

func (s *SendHandle) Close() {
	for len(s.ackTasks) > 0 {
		time.Sleep(time.Second)
	}
	close(s.ackTasks)
	s.ctxCancelFunc()
	// 等待ackLoop结束
	<-s.ackFisnished
	// 归还资源
	for {
		select {
		case ackLog := <-s.waitAck:
			kernel.PutOnlyID(ackLog)
		default:
			return
		}
	}
}
