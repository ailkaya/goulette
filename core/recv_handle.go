package core

import (
	// "fmt"
	"context"
	"sync/atomic"
	"time"

	"github.com/ailkaya/goport/client/list"
	"github.com/ailkaya/goport/utils"
)

type RecvHandle struct {
	ctx           context.Context
	ctxCancelFunc context.CancelFunc
	ackFisnished  chan bool
	host          *Core
	topic         string
	// 最新处理过的消息的id
	latestMid int64
	// ack任务通道
	ackTasks chan int64
	// 等待send ack的消息链表
	waitAck *list.ListForMsg
	// 避免并发问题的锁(ack时), 0代表此时没有协程正在进行ack操作
	// 防止ifHasSended与sendAck同时操作ack链表进行pop操作
	// 虽然ifHasSended的mid一定比sendAck的更大, 更后到达
	// 但进行ifHasSended时可能之前的ack还没有执行完毕, 从而导致并发问题
	waitAckMutex int64
}

func NewRecvHandle(host *Core, topic string, bucketSize int64) *RecvHandle {
	ctx, ctxCancelFunc := context.WithCancel(context.Background())
	r := &RecvHandle{
		ctx:           ctx,
		ctxCancelFunc: ctxCancelFunc,
		ackFisnished:  make(chan bool),
		host:          host,
		topic:         topic,
		latestMid:     0,
		ackTasks:      make(chan int64, taskChannelSize),
		waitAck:       list.NewListForMsg(),
	}
	go r.ackLoop()
	return r
}

func (r *RecvHandle) ackLoop() {
	running := true
	for running {
		select {
		case <-r.ctx.Done():
			running = false
		default:
			mid := <-r.ackTasks
			r.ack(mid)
			// 方便ifHasSended获取锁
			time.Sleep(ackLoopInterval)
		}
	}
	r.ackFisnished <- true
}

func (r *RecvHandle) Pull(mid, retryid int64) ([]byte, error) {
	// 判断是否为重试请求
	if retryid != -1 {
		// 获取mid为retryid的消息数据
		retrymsg := r.getRecvRetryMsg(retryid)
		if retrymsg != nil {
			// 找到了未发送成功的消息
			// 重新放回ack队列中等待ack
			r.waitAck.Push(mid, retrymsg)
			return retrymsg, nil
		}
		// 继续执行说明mid=retryid的那次请求没有到达service, 从队列中拿一个新值返回
	}
	// 拿一个消息数据
	message := r.host.Pull(r.topic)
	// fmt.Println(5)
	// r.leakyBucket <- true
	r.waitAck.Push(mid, message)
	r.latestMid = utils.Max(r.latestMid, mid)
	return message, nil
}

// 获取未发送成功的消息数据
// 该函数只由Pull调用
func (r *RecvHandle) getRecvRetryMsg(retryMid int64) []byte {
	// 一直尝试获取锁，直到获取成功
	for !atomic.CompareAndSwapInt64(&r.waitAckMutex, 0, 1) {
	}
	// 释放锁
	defer atomic.CompareAndSwapInt64(&r.waitAckMutex, 1, 0)

	for {
		mid := r.waitAck.GetHeadValue()
		if mid < retryMid {
			// Pop得到的对象需要手动放回Pool中
			list.PutNodeOfMsg(r.waitAck.Pop())
		} else if mid == retryMid {
			// 拿到未发送成功的消息返回
			retrymsg := r.waitAck.Pop()
			list.PutNodeOfMsg(retrymsg)
			return retrymsg.Msg
		} else {
			// 到达mid > retryMid时说明retryMid的那次请求没有到达service, 从队列中拿一个新值返回
			// 直接返回nil，让Pull函数继续执行
			return nil
		}
	}
}

func (r *RecvHandle) Ack(latestMid int64) {
	r.ackTasks <- latestMid
}

func (r *RecvHandle) ack(latestMid int64) {
	for !atomic.CompareAndSwapInt64(&r.waitAckMutex, 0, 1) {
	}
	defer atomic.CompareAndSwapInt64(&r.waitAckMutex, 1, 0)
	// fmt.Println("ack", latestMid)
	// 将成功发送的消息从ackdata中移除
	for {
		mid := r.waitAck.GetHeadValue()
		// Ack的leatestMid一定存在于waitAck中
		if mid <= latestMid {
			// Pop得到的对象需要手动放回Pool中
			list.PutNodeOfMsg(r.waitAck.Pop())
		} else {
			break
		}
	}
}

func (r *RecvHandle) Close() {
	for len(r.ackTasks) > 0 {
		time.Sleep(time.Second)
	}
	close(r.ackTasks)
	r.ctxCancelFunc()
	// 等待ackLoop结束
	<-r.ackFisnished
	for !r.waitAck.Empty() {
		message := r.waitAck.Pop()
		// 将未ack的数据放回对应的topic chan中
		r.host.Push(r.topic, message.Msg)
		list.PutNodeOfMsg(message)
	}
}
