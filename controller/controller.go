package controller

import "fmt"

type Controller struct {
	bufferSize int
	msgCh      map[string]chan []byte
}

func NewController(bufferSize int) *Controller {
	return &Controller{
		bufferSize: bufferSize,
		msgCh:      make(map[string]chan []byte),
	}
}

func (c *Controller) RegisterTopic(topic string) {
	if _, ok := c.msgCh[topic]; !ok { // 如果不存在，则创建
		c.msgCh[topic] = make(chan []byte, c.bufferSize)
		// fmt.Println("topic exist")
	}
	// fmt.Println("topic not exist")
}

func (c *Controller) SendMessage(topic string, msg []byte) error {
	// fmt.Println(c.bufferSize)
	msgCh, ok := c.msgCh[topic]
	if !ok {
		return fmt.Errorf("topic %s not found", topic)
	}
	msgCh <- msg
	// fmt.Println('2')
	return nil
}

func (c *Controller) RetrieveMessage(topic string) ([]byte, error) {
	msgCh, ok := c.msgCh[topic]
	if !ok {
		return nil, fmt.Errorf("topic %s not found", topic)
	}
	return <-msgCh, nil
}
