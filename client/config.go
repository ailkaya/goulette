package client

import (
	"fmt"
)

type Config struct {
	Brokers []string
	// 各broekr的权重，用来负载均衡
	BrokerWeights map[string]float64
	// send/receive类型
	// 输入输出缓冲区大小
	InBufferSize  int
	OutBufferSize int
	// 每个broker的连接数
	MaxConnections int
	// 一个周期内的基准接收数，实际接收数还需考虑权重
	BaseReceive int32
	// 每个send/receive kernel的最大未确认消息维护数量
	// 设置为1时会client与server之间会变成同步通信
	SendConcurrency int32
	RecvConcurrency int32
	// 消息ack间隔
	AckInterval int32
}

var DefaultConfig = &Config{
	InBufferSize:    100,
	OutBufferSize:   100,
	MaxConnections:  1,
	BaseReceive:     10000,
	SendConcurrency: 100,
	RecvConcurrency: 100,
	AckInterval:     100,
}

func parseConfig(config *Config) (*Config, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("brokers cannot be empty")
	}
	if config.BrokerWeights == nil {
		config.BrokerWeights = make(map[string]float64)
		for i := 0; i < len(config.Brokers); i++ {
			// config.BrokerWeights = append(config.BrokerWeights, 1.0)
			config.BrokerWeights[config.Brokers[i]] = 1.0
		}
	}
	if len(config.BrokerWeights) != len(config.Brokers) {
		return nil, fmt.Errorf("broker weights length must be equal to brokers length")
	}
	if config.InBufferSize < 0 {
		return nil, fmt.Errorf("in buffer size must be greater than 0")
	} else if config.InBufferSize == 0 {
		config.InBufferSize = DefaultConfig.InBufferSize
	}
	if config.OutBufferSize < 0 {
		return nil, fmt.Errorf("out buffer size must be greater than 0")
	} else if config.OutBufferSize == 0 {
		config.OutBufferSize = DefaultConfig.OutBufferSize
	}
	if config.MaxConnections < 0 {
		return nil, fmt.Errorf("max connections must be greater than 0")
	} else if config.MaxConnections == 0 {
		config.MaxConnections = DefaultConfig.MaxConnections
	}
	if config.BaseReceive < 0 {
		return nil, fmt.Errorf("base receive must be greater than 0")
	} else if config.BaseReceive == 0 {
		config.BaseReceive = DefaultConfig.BaseReceive
	}
	if config.SendConcurrency < 0 {
		return nil, fmt.Errorf("send concurrency must be greater than 0")
	} else if config.SendConcurrency == 0 {
		config.SendConcurrency = DefaultConfig.SendConcurrency
	}
	if config.RecvConcurrency < 0 {
		return nil, fmt.Errorf("recv concurrency must be greater than 0")
	} else if config.RecvConcurrency == 0 {
		config.RecvConcurrency = DefaultConfig.RecvConcurrency
	}
	if config.AckInterval < 0 {
		return nil, fmt.Errorf("ack interval must be greater than 0")
	} else if config.AckInterval == 0 {
		config.AckInterval = DefaultConfig.AckInterval
	}
	return config, nil
}
