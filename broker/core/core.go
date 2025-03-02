package core

import (
	"bufio"
	"context"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

const (
	ConsumedFilePath     = "log/consumed.cor"
	TempConsumedFilePath = "log/consumedTemp.cor"
	FlushInterval        = time.Second
)

type Core struct {
	ctx context.Context
	// consumed写磁盘间隔
	interval time.Duration
	// 每个通道的容量
	bufferSize int64
	// 存储消息的通道
	topic2ch map[string]chan *box
	// 最大已消费id
	consumed *sync.Map
	wt       *writer
}

type box struct {
	id   uint64
	data []byte
}

// bufferSize: 每个topic下的通道的容量, bucketSize: 桶的容量(总维护消息数)
func NewCore(bufferSize int64) *Core {
	ctx := context.Background()
	wt, err := newWriter(false)
	if err != nil {
		panic(err)
	}
	if err = os.MkdirAll(path.Dir(ConsumedFilePath), 0777); err != nil {
		panic(err)
	}
	c := &Core{
		ctx:        ctx,
		interval:   FlushInterval,
		bufferSize: bufferSize,
		topic2ch:   make(map[string]chan *box),
		consumed:   &sync.Map{},
		wt:         wt,
	}
	go c.flush()
	return c
}

func (c *Core) flush() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for range ticker.C {
		// 创建一个临时文件
		tempFile, err := os.Create(TempConsumedFilePath)
		if err != nil {
			panic(err)
		}
		buf := bufio.NewWriter(tempFile)
		c.consumed.Range(func(k, v interface{}) bool {
			buf.WriteString(k.(string))
			buf.WriteString("")
			buf.WriteString(strconv.FormatUint(v.(uint64), 10))
			buf.WriteString("\n")
			return true
		})
		// 覆盖旧文件
		os.Rename(TempConsumedFilePath, ConsumedFilePath)
		tempFile.Close()
	}
}

func (c *Core) RegisterTopic(topic string) {
	if _, ok := c.topic2ch[topic]; !ok {
		c.topic2ch[topic] = make(chan *box, c.bufferSize)
		c.consumed.Store(topic, 0)
	}
}

func (c *Core) IsTopicExist(topic string) bool {
	_, ok := c.topic2ch[topic]
	return ok
}

func (c *Core) Push(topic string, msg []byte) {
	id, err := c.wt.write(&line{
		topic: topic,
		data:  msg,
	})
	if err != nil {
		// TODO: 错误处理
	}
	c.topic2ch[topic] <- &box{id, msg}
}

func (c *Core) Pull(topic string) []byte {
	return (<-c.topic2ch[topic]).data
}

func (c *Core) UpdateConsumed(topic string, consumed uint64) {
	c.consumed.Store(topic, consumed)
}
