package core

import (
	// "fmt"
	"context"
	"sync/atomic"
	"time"

	"github.com/ailkaya/goport/client/list"
	"github.com/ailkaya/goport/utils"
)

var (
	taskChannelSize = 1000
	ackLoopInterval = time.Millisecond * 1
)

type SendHandle struct {
	ctx           context.Context
	ctxCancelFunc context.CancelFunc
	ackFisnished  chan bool
	host          *Core
	topic         string
	// 最新处理过的消息的id
	latestMid int64
	// ack任务通道, 传入latestMid, 调用ack函数
	ackTasks chan int64
	// 等待send ack的消息链表
	waitAck *list.ListForMid
	// 避免并发问题的锁(ack时), 0代表此时没有协程正在进行ack操作
	// 防止ifHasSended与sendAck同时操作ack链表进行pop操作
	// 虽然ifHasSended的mid一定比sendAck的更大, 更后到达
	// 但进行ifHasSended时可能之前的ack还没有执行完毕, 从而导致并发问题
	waitAckMutex int64
}

func NewSendHandle(host *Core, topic string, bucketSize int64) *SendHandle {
	ctx, ctxCancelFunc := context.WithCancel(context.Background())
	s := &SendHandle{
		ctx:           ctx,
		ctxCancelFunc: ctxCancelFunc,
		ackFisnished:  make(chan bool),
		host:          host,
		topic:         topic,
		latestMid:     0,
		ackTasks:      make(chan int64, taskChannelSize),
		waitAck:       list.NewListForMid(),
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
			time.Sleep(ackLoopInterval)
		}
	}
	s.ackFisnished <- true
}

func (s *SendHandle) Push(mid, retryMid int64, msg []byte) error {
	// 判断是否是重试请求以及已经接受过
	if retryMid != -1 && s.ifHasSended(retryMid) {
		// 重新放回ack队列中等待ack
		s.waitAck.Push(mid)
		return nil
	}
	// select {
	// case s.leakyBucket <- true:
	s.host.Push(s.topic, msg)
	s.waitAck.Push(mid)
	s.latestMid = utils.Max(s.latestMid, mid)
	return nil
	// default:
	// return fmt.Errorf("leaky bucket is full")
	// }
}

// 是否已经从client接收过
// 该函数只由Push调用
func (s *SendHandle) ifHasSended(retryMid int64) bool {
	// 一直尝试获取锁，直到获取成功
	for !atomic.CompareAndSwapInt64(&s.waitAckMutex, 0, 1) {
	}
	// 释放锁
	defer atomic.CompareAndSwapInt64(&s.waitAckMutex, 1, 0)

	for {
		mid := s.waitAck.GetHeadValue()
		if mid < retryMid {
			// Pop得到的对象需要手动放回Pool中
			list.PutNodeOfMid(s.waitAck.Pop())
		} else if mid == retryMid {
			// 拿到未发送成功的消息返回
			retrymsg := s.waitAck.Pop()
			list.PutNodeOfMid(retrymsg)
			return true
		} else {
			// 到达mid > retryMid时说明retryMid的那次请求没有到达service, 说明没有被接受过
			return false
		}
	}
}

func (s *SendHandle) Ack(latestMid int64) {
	s.ackTasks <- latestMid
}

func (s *SendHandle) ack(latestMid int64) {
	for !atomic.CompareAndSwapInt64(&s.waitAckMutex, 0, 1) {
	}
	defer atomic.CompareAndSwapInt64(&s.waitAckMutex, 1, 0)
	// fmt.Println("ack", latestMid)
	// 将成功发送的消息从ackdata中移除
	for {
		// fmt.Println("ack loop")
		mid := s.waitAck.GetHeadValue()
		// Ack的leatestMid一定存在于waitAck中
		if mid <= latestMid {
			// Pop得到的对象需要手动放回Pool中
			list.PutNodeOfMid(s.waitAck.Pop())
		} else {
			break
		}
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
	for !s.waitAck.Empty() {
		list.PutNodeOfMid(s.waitAck.Pop())
	}
}
