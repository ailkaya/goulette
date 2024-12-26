package core

import (
	"context"
	"time"

	"github.com/ailkaya/goport/client"
	"github.com/ailkaya/goport/server/common/config"
)

type Core struct {
	ctx context.Context
	// 过期时间
	expire time.Duration
	// 最大重试次数
	maxRetry int
	// 每个通道的容量
	bufferSize int64
	// 漏桶
	// leakyBucket chan bool
	// 存储消息的通道
	topic2ch map[string]chan []byte
	// topic2ch map[string]chan *carrier
	// 每个topic的前置处理器的控制器
	topic2controller map[string]*client.PreProcessorController
}

// bufferSize: 每个topic下的通道的容量, bucketSize: 桶的容量(总维护消息数)
func NewCore(bufferSize int64) *Core {
	ctx := context.Background()
	return &Core{
		ctx:        ctx,
		expire:     time.Duration(config.Conf.APP.MessageExpireTime),
		maxRetry:   config.Conf.APP.MessageSendRetryTimes,
		bufferSize: bufferSize,
		// leakyBucket: make(chan bool, bucketSize),
		topic2ch: make(map[string]chan []byte),
		// topic2ch:         make(map[string]chan *carrier),
		topic2controller: make(map[string]*client.PreProcessorController),
	}
}

func (c *Core) RegisterTopic(topic string) {
	if _, ok := c.topic2ch[topic]; !ok {
		c.topic2ch[topic] = make(chan []byte, c.bufferSize)
		// c.topic2ch[topic] = make(chan *carrier, c.bufferSize)
		c.topic2controller[topic] = client.NewPreProcessorController()
	}
}

func (c *Core) IsTopicExist(topic string) bool {
	_, ok := c.topic2ch[topic]
	return ok
}

func (c *Core) Push(topic string, msg []byte) {
	// fmt.Println("push")
	// c.leakyBucket <- true
	c.topic2ch[topic] <- msg
	// c.topic2ch[topic] <- getCarrier(c.aTimer.now(), msg)
}

func (c *Core) Pull(topic string) []byte {
	// fmt.Println("pull")
	// <-c.leakyBucket
	return <-c.topic2ch[topic]
	// carry := <-c.topic2ch[topic]
	// for c.aTimer.now().Sub(*carry.enter) > c.expire {
	// 	carry = <-c.topic2ch[topic]
	// }
	// return carry.data
}

func (c *Core) GetOutput(topic string) chan []byte {
	return c.topic2ch[topic]
}

func (c *Core) GetController(topic string) *client.PreProcessorController {
	return c.topic2controller[topic]
}

func (c *Core) RegisterPreProcessor(topic string, block chan bool) {
	c.topic2controller[topic].RegisterContainer(block)
}

func (c *Core) UnRegister(topic string) {
	c.topic2controller[topic].UnRegisterContainer()
}
