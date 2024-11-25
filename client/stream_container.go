package client

import (
	"context"
	"io"
	"sync"

	kn "github.com/ailkaya/goport/client/kernel"
)

var (
	containerPool = sync.Pool{
		New: func() interface{} {
			return &StreamContainer{}
		},
	}
	// ErrClose = fmt.Errorf("close")
)

func GetStreamContainer() *StreamContainer {
	return containerPool.Get().(*StreamContainer)
}

func PutStreamContainer(sc *StreamContainer) {
	containerPool.Put(sc)
}

type StreamContainer struct {
	ctx       context.Context
	ctxCancel context.CancelFunc
	// kernel流关闭函数
	kernelClose func()
	// 处理过的信息数 process message
	numOfPM int64
	// 发生了错误的信息数 error message
	numOfEM int64
}

func (sc *StreamContainer) InitStreamContainer(sg *StreamGroup, kernel kn.IKernel) {
	sc.ctx, sc.ctxCancel = context.WithCancel(context.Background())
	sc.kernelClose = kernel.Close
	sc.numOfPM, sc.numOfEM = 0, 0
	go kernel.GetKernelFunc()(sg.GetBrokerClient(), sc.Check)
}

func (sc *StreamContainer) GetNofPM() float64 {
	return float64(sc.numOfPM)
}

func (sc *StreamContainer) GetNofEM() float64 {
	return float64(sc.numOfEM)
}

func (sc *StreamContainer) Check(err error) bool {
	if err != nil {
		sc.numOfEM++
		if err == io.EOF {
			sc.Close()
		}
	}
	sc.numOfPM++
	select {
	case <-sc.ctx.Done():
		sc.kernelClose()
		return false
	default:
		return true
	}
}

func (sc *StreamContainer) Close() {
	// fmt.Println("container closed")
	sc.ctxCancel()
	// sc.Check(ErrClose)
}

func (sc *StreamContainer) IsClosed() bool {
	select {
	case <-sc.ctx.Done():
		return true
	default:
		return false
	}
}
