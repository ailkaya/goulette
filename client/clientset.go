package client

import (
	// "fmt"

	log "github.com/ailkaya/goport/common/logger"
)

var (
	logger               log.ILog
	DefaultInBufferSize  int = 1e6
	DefaultOutBufferSize int = 1e6
)

type ClientSet struct {
	inBufferSize  int
	outBufferSize int
	// broker -> client
	broker2client map[string]*Client
	// 每个topic所对应的输入输出通道
	topic2ch map[string]*Pipe
}

// 输入输出通道
type Pipe struct {
	In  chan interface{}
	Out chan interface{}
}

type SetOption struct {
	InBufferSize  int
	OutBufferSize int
	Option        *Option
}

func NewClientSet(addrs []string, setOption *SetOption) (*ClientSet, error) {
	logger = log.Log
	setOption = parseSetOption(setOption)
	cs := &ClientSet{
		inBufferSize:  setOption.InBufferSize,
		outBufferSize: setOption.OutBufferSize,
		broker2client: make(map[string]*Client),
		topic2ch:      make(map[string]*Pipe),
	}
	if err := cs.registerBroker(addrs, setOption.Option); err != nil {
		return nil, err
	}
	return cs, nil
}

// 向clientset注册broker
func (cs *ClientSet) registerBroker(addrs []string, option *Option) error {
	for _, addr := range addrs {
		// 判断是否已经注册过
		if _, ok := cs.broker2client[addr]; ok {
			logger.Errorf("broker %s already registered", addr)
			continue
		}
		client, err := NewClient(addr, option)
		if err != nil {
			logger.Errorf("create client error")
			return err
		}
		cs.broker2client[addr] = client
	}
	return nil
}

// 在多个broker中注册topic
func (cs *ClientSet) RegisterTopic(addrs []string, topic string) {
	if _, ok := cs.topic2ch[topic]; !ok {
		cs.topic2ch[topic] = &Pipe{
			In:  make(chan interface{}, cs.inBufferSize),
			Out: make(chan interface{}, cs.outBufferSize),
		}
	}

	for _, addr := range addrs {
		if client, ok := cs.broker2client[addr]; ok {
			client.RegisterTopic(topic, &Pipe{
				In:  cs.topic2ch[topic].In,
				Out: cs.topic2ch[topic].Out,
			})
		} else {
			logger.Panicf("broker %s not registered", addr)
		}
	}
}

// 向brokers推送消息
func (cs *ClientSet) Push(topic string, msg interface{}) {
	cs.topic2ch[topic].In <- msg
}

// 从brokers拉取消息
func (cs *ClientSet) Pull(topic string) interface{} {
	return <-cs.topic2ch[topic].Out
}

// 关闭连接
func (cs *ClientSet) Close() {
	for _, client := range cs.broker2client {
		client.Close()
	}
}

func parseSetOption(setOption *SetOption) *SetOption {
	if setOption == nil {
		setOption = &SetOption{
			InBufferSize:  DefaultInBufferSize,
			OutBufferSize: DefaultOutBufferSize,
		}
	} else {
		if setOption.InBufferSize == 0 {
			setOption.InBufferSize = DefaultInBufferSize
		}
		if setOption.OutBufferSize == 0 {
			setOption.OutBufferSize = DefaultOutBufferSize
		}
	}
	return setOption
}
