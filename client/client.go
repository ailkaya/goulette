package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	// "time"

	pb "github.com/ailkaya/goport/broker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	topicRequestPool sync.Pool = sync.Pool{
		New: func() interface{} {
			return &pb.TopicRequest{}
		},
	}
	defaultOption = &Option{
		SenderNum:         1,
		ReceiverNum:       1,
		SendBufferSize:    1024,
		ReceiveBufferSize: 1024,
	}
)

// 代表每个broker的client
type Client struct {
	ctx       context.Context
	ctxCancel context.CancelFunc
	client    pb.BrokerServiceClient

	// 配置信息
	option *Option
	// 消息发送缓冲区
	sendBuffer map[string]chan interface{}
	// 消息接收缓冲区
	receiveBuffer map[string]chan interface{}

	sendStream []grpc.ClientStreamingClient[pb.SendMessageRequest, pb.Empty]
	recvStream []grpc.ServerStreamingClient[pb.RetrieveMessageResponse]
}

// 配置信息
type Option struct {
	// 数据发送协程数量
	SenderNum int
	// 数据接收协程数量
	ReceiverNum int
	// 数据发送缓冲区大小
	SendBufferSize int
	// 数据接收缓冲区大小
	ReceiveBufferSize int
}

// 消息格式
type Format struct {
	Data interface{}
}

// 新建broker-client
func NewClient(addr string, option *Option) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		ctx:           ctx,
		ctxCancel:     cancel,
		sendBuffer:    make(map[string]chan interface{}),
		receiveBuffer: make(map[string]chan interface{}),
		sendStream:    make([]grpc.ClientStreamingClient[pb.SendMessageRequest, pb.Empty], option.SenderNum),
		recvStream:    make([]grpc.ServerStreamingClient[pb.RetrieveMessageResponse], option.ReceiverNum),
	}
	c.option = parseOption(option)
	if err := c.connect(addr); err != nil {
		logger.Errorf("connect to broker error: %s", err)
		return nil, err
	}
	return c, nil
}

// 连接到broker
func (c *Client) connect(addr string) error {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Errorf("connect to broker error: %s", err)
		return err
	}
	c.client = pb.NewBrokerServiceClient(conn)
	return nil
}

// 向broker注册topic
// target带包含消息来源及去向的channel, 不指定则使用默认channel
func (c *Client) RegisterTopic(topic string, target *Pipe) error {
	if _, ok := c.sendBuffer[topic]; ok {
		logger.Errorf("topic %s already registered", topic)
		return fmt.Errorf("topic %s already registered", topic)
	}
	c.sendBuffer[topic] = make(chan interface{}, c.option.SendBufferSize)
	c.receiveBuffer[topic] = make(chan interface{}, c.option.ReceiveBufferSize)

	topicReq := topicRequestPool.Get().(*pb.TopicRequest)
	defer topicRequestPool.Put(topicReq)
	topicReq.Topic = topic
	// ctx, _ := context.WithTimeout(c.ctx, 5*time.Second)
	_, err := c.client.RegisterTopic(c.ctx, topicReq)
	if err != nil {
		logger.Errorf("register topic error: %s", err)
		return err
	}

	// 选择消息来源及去向
	target = c.parsePipe(topic, target)
	go c.send(topic, target.In)
	go c.receive(topic, target.Out)
	return nil
}

// 决定消息来源及去向的channel
func (c *Client) parsePipe(topic string, pipe *Pipe) *Pipe {
	if pipe == nil {
		pipe = &Pipe{
			In:  c.sendBuffer[topic],
			Out: c.receiveBuffer[topic],
		}
	} else {
		if pipe.In == nil {
			pipe.In = c.sendBuffer[topic]
		}
		if pipe.Out == nil {
			pipe.Out = c.receiveBuffer[topic]
		}
	}
	return pipe
}

// 发送消息
func (c *Client) Push(topic string, data interface{}) error {
	if ch, ok := c.sendBuffer[topic]; ok {
		ch <- data
		return nil
	}
	logger.Errorf("topic %s not registered", topic)
	return fmt.Errorf("topic %s not registered", topic)
}

// 接收消息
func (c *Client) Pull(topic string) (interface{}, error) {
	if ch, ok := c.receiveBuffer[topic]; !ok {
		return <-ch, nil
	}
	logger.Errorf("topic %s not registered", topic)
	return nil, fmt.Errorf("topic %s not registered", topic)
}

// 向broker发送消息
func (c *Client) send(topic string, from chan interface{}) {
	var err error
	// ch := c.sendBuffer[topic]

	for i := 0; i < c.option.SenderNum; i++ {
		c.sendStream[i], err = c.client.SendMessage(c.ctx)
		if err != nil {
			logger.Errorf("send message error: %s", err)
			return
		}

		sendReq, format := pb.SendMessageRequest{}, Format{}
		go func(req *pb.SendMessageRequest, format *Format, k int) {
			for {
				format.Data = <-from
				jsonBytes, _ := json.Marshal(format)
				if err != nil {
					logger.Errorf("encode message error: %s", err)
					continue
				}
				req.Topic, req.Msg = topic, jsonBytes
				// fmt.Println("send message: ", format.Data)
				if err = c.sendStream[k].Send(req); err != nil {
					logger.Errorf("send message error: %s", err)
					return
				}
			}
		}(&sendReq, &format, i)
	}
}

// 从broker接收消息
func (c *Client) receive(topic string, to chan interface{}) {
	var err error
	// ch := c.respBuffer[topic]
	for i := 0; i < c.option.ReceiverNum; i++ {
		c.recvStream[i], err = c.client.RetrieveMessage(c.ctx, &pb.TopicRequest{Topic: topic})
		if err != nil {
			logger.Errorf("receive message error: %s", err)
			return
		}

		recvResp, format := pb.RetrieveMessageResponse{}, Format{}
		go func(resp *pb.RetrieveMessageResponse, format *Format, k int) {
			for {
				var err error
				resp, err = c.recvStream[k].Recv()
				// fmt.Println("receive message: ", resp)
				if resp == nil {
					fmt.Println("receive stream nil")
					continue
				}
				if err == io.EOF {
					fmt.Println("receive stream end")
					return
				}
				if err != nil {
					logger.Errorf("receive message error: %s", err)
					return
				}
				json.Unmarshal(resp.Msg, &format)
				to <- format.Data
			}
		}(&recvResp, &format, i)
	}
}

// 关闭连接，关闭所有流
func (c *Client) Close() {
	for _, stream := range c.sendStream {
		stream.CloseAndRecv()
	}
	for _, stream := range c.recvStream {
		stream.CloseSend()
	}
	c.ctxCancel()
}

// 检查option值
func parseOption(option *Option) *Option {
	if option == nil {
		option = defaultOption
	} else {
		if option.SenderNum < 0 || option.ReceiverNum < 0 || option.SendBufferSize < 0 || option.ReceiveBufferSize < 0 {
			logger.Errorf("Invalid option: argvs must be positive. Auto set to default")
		}
		if option.SenderNum <= 0 {
			option.SenderNum = defaultOption.SenderNum
		}
		if option.ReceiverNum <= 0 {
			option.ReceiverNum = defaultOption.ReceiverNum
		}
		if option.SendBufferSize <= 0 {
			option.SendBufferSize = defaultOption.SendBufferSize
		}
		if option.ReceiveBufferSize <= 0 {
			option.ReceiveBufferSize = defaultOption.ReceiveBufferSize
		}
	}
	return option
}
