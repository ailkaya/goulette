package core

// import "fmt"

type Core struct {
	// 每个通道的容量
	bufferSize int64
	// 漏桶
	leakyBucket chan bool
	// 存储消息的通道
	topic2ch map[string]chan []byte
}

// chanSize: 每个通道的容量, bucketSize: 桶的容量
func NewCore(bufferSize, bucketSize int64) *Core {
	return &Core{
		bufferSize:  bufferSize,
		leakyBucket: make(chan bool, bucketSize),
		topic2ch:    make(map[string]chan []byte),
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
	// fmt.Println("push")
	c.leakyBucket <- true
	c.topic2ch[topic] <- msg
}

func (c *Core) Pull(topic string) []byte {
	// fmt.Println("pull")
	<-c.leakyBucket
	return <-c.topic2ch[topic]
}
