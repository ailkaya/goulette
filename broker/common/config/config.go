package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

var Conf *Config

type Config struct {
	APP APP `yaml:"app"`
}

type APP struct {
	Port       string `yaml:"port"`
	LogPath    string `yaml:"log-path"`
	BufferSize int64  `yaml:"buffer-size"`
	// BucketSize              int64  `yaml:"bucket-size"`
	SendBucketSizePerStream int64 `yaml:"send-bucket-size-per-stream"`
	RecvBucketSizePerStream int64 `yaml:"recv-bucket-size-per-stream"`
	MaxReceivePerCycle      int   `yaml:"max-receive-per-cycle"`
	// timer的更新时间间隔(ms)
	TimerUpdateInterval int `yaml:"timer-update-interval"`
	// 消息过期时间(ms)
	MessageExpireTime int `yaml:"message-expire-time"`
	// 消息发送重试次数
	MessageSendRetryTimes int `yaml:"message-send-retry-times"`
}

func InitConf() error {
	Conf = &Config{}
	err := getConf("config.yaml", Conf) // 尝试在当前目录查找
	if err != nil {
		log.Printf("[Setting] Config init failed: %v", err)
		return err
	}
	return nil
}

// 从yaml配置文件中获取配置
func getConf(yamlFilePath string, confModel *Config) error {
	file, err := os.ReadFile(yamlFilePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(file, confModel)
	if err != nil {
		return err
	}
	return nil
}
