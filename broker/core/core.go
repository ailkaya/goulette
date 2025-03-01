package core

import (
	"context"
	"time"

	"github.com/ailkaya/goulette/broker/common/config"
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
	// 存储消息的通道
	topic2ch map[string]chan []byte
}

// bufferSize: 每个topic下的通道的容量, bucketSize: 桶的容量(总维护消息数)
func NewCore(bufferSize int64) *Core {
	ctx := context.Background()
	return &Core{
		ctx:        ctx,
		expire:     time.Duration(config.Conf.APP.MessageExpireTime),
		maxRetry:   config.Conf.APP.MessageSendRetryTimes,
		bufferSize: bufferSize,
		topic2ch:   make(map[string]chan []byte),
	}
}

func (c *Core) RegisterTopic(topic string) {
	if _, ok := c.topic2ch[topic]; !ok {
		c.topic2ch[topic] = make(chan []byte, c.bufferSize)
	}
}

func (c *Core) IsTopicExist(topic string) bool {
	_, ok := c.topic2ch[topic]
	return ok
}

func (c *Core) Push(topic string, msg []byte) {
	c.topic2ch[topic] <- msg
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
