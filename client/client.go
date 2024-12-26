package client

import "fmt"

type Client struct {
	scheduler *Scheduler
	in        chan []byte
	out       chan []byte
}

func NewClient(scheduler *Scheduler) *Client {
	return &Client{
		scheduler: scheduler,
		in:        scheduler.getIn(),
		out:       scheduler.getOut(),
	}
}

func (c *Client) AuthSend() {
	c.scheduler.AuthSend()
}

func (c *Client) AuthReceive() {
	c.scheduler.AuthReceive()
}

func (c *Client) Send(data []byte) {
	// if c.in == nil {
	// 	return fmt.Errorf("send not authenticated")
	// }
	c.in <- data
}

func (c *Client) Receive() []byte {
	// if c.out == nil {
	// 	return nil, fmt.Errorf("receive not authenticated")
	// }
	return <-c.out
}

func (c *Client) Close() {
	c.scheduler.Close()
	fmt.Println("client closed")
}
