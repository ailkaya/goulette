package main

import (
	"fmt"
	"log"
	"os"

	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"

	client "github.com/ailkaya/goport/client"
	"github.com/ailkaya/goport/server/common/logger"
	"gopkg.in/yaml.v2"
)

// 1kB的数据
// var data = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.0"

// 100B的数据
var data = "aB3dEfGfkdi3irnsjf8hIjKlMnOpQrStUvWxYz1234567890!@#$%^&*()_+[]{}|;:',.<>?`~ABCDEfghijklmnop234567890"

// 10B的数据
// var data = "1234567890"

var (
	err       error
	config    *Config
	msgClient *client.Client
	// Wg        = &sync.WaitGroup{}
)

type Config struct {
	Params *TestParams `yaml:"test_params"`
}

type TestParams struct {
	TestCount   int `yaml:"test_count"`
	NumMessages int `yaml:"num_messages"`
	// ProducerN    int64  `yaml:"producer_n"`
	ConsumerN int `yaml:"consumer_n"`

	Brokers         []string `yaml:"brokers"`
	MaxConnections  int      `yaml:"max_connections"`
	InBufferSize    int      `yaml:"in_buffer_size"`
	OutBufferSize   int      `yaml:"out_buffer_size"`
	BaseReceive     int32    `yaml:"base_receive"`
	SendConcurrency int32    `yaml:"send_concurrency"`
	RecvConcurrency int32    `yaml:"receive_concurrency"`
	AckInterval     int32    `yaml:"ack_interval"`
}

func readYaml() {
	config = &Config{}
	// 读取配置文件
	err := getConf("./config.yaml", config)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	Init()
	go func() {
		log.Println(http.ListenAndServe(":6060", nil))
	}()
	// fmt.Println(config.Params)
	time.Sleep(3 * time.Second)
	sumElapsedReceive := time.Duration(0)

	for j := 0; j < config.Params.TestCount; j++ {
		// cnt = 0
		wg := &sync.WaitGroup{}

		startReceive := time.Now()
		// 开始发送
		// fmt.Println("start send")
		// go func() {
		for j := 0; j < config.Params.NumMessages; j++ {
			send([]byte(data))
		}
		// }()

		// startReceive := time.Now()
		// 启动消费者
		// fmt.Println("start receive")
		for i := 0; i < config.Params.ConsumerN; i++ {
			wg.Add(1)
			go func() {
				for j := 0; j < config.Params.NumMessages/config.Params.ConsumerN; j++ {
					receive()
				}
				wg.Done()
			}()
		}
		wg.Wait()
		// time.Sleep(1 * time.Second)
		interval := time.Since(startReceive)
		// fmt.Printf("round %d: Received %d messages in %s\n", j+1, config.Params.NumMessages, interval)
		elapsedReceive := interval
		sumElapsedReceive += elapsedReceive
	}

	elapsedReceive := sumElapsedReceive / time.Duration(config.Params.TestCount)
	// fmt.Printf("Env: %d tests, %d producers, %d consumers, %d messages\n", testCount, numProducers, numConsumers, numMessages)
	fmt.Printf("Received %d messages in %s(mean)\n", config.Params.NumMessages, elapsedReceive)
	fmt.Printf("Receive Throughput: %.2f messages/sec(mean)\n\n", float64(config.Params.NumMessages)/elapsedReceive.Seconds())
	msgClient.Close()
	time.Sleep(15 * time.Second)
	// fmt.Println("close")
}

func Init() {
	logger.InitLogger("log.out")
	readYaml()
	msgClient, err = client.NewScheduler("test", &client.Config{
		Brokers:        config.Params.Brokers,
		InBufferSize:   config.Params.InBufferSize,
		OutBufferSize:  config.Params.OutBufferSize,
		MaxConnections: config.Params.MaxConnections,
		BaseReceive:    config.Params.BaseReceive,
		// 桶漏算法中的桶容量
		SendConcurrency: config.Params.SendConcurrency,
		RecvConcurrency: config.Params.RecvConcurrency,
		// ack间隔
		AckInterval: config.Params.AckInterval,
	})
	if err != nil {
		log.Fatal(err)
	}
	msgClient.AuthSend()
	msgClient.AuthReceive()
}

// 模拟的 send 函数
func send(v []byte) {
	msgClient.Send(v)
}

// 模拟的 receive 函数
func receive() {
	msgClient.Receive()
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
