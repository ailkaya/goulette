package main

import (
	"flag"
	"fmt"

	"log"
	// "net/http"
	_ "net/http/pprof"
	"time"

	// "sync"
	// "sync/atomic"
	client "github.com/ailkaya/goport/client"
	"github.com/ailkaya/goport/common/logger"
)

// var data = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.0"
// var data = "1234567890"

var (
	// 每次测试的轮数
	testCount = 1
	// 每轮发送的消息数量
	numMessages = 1
	// 生产者数量
	ProducerN int64 = 1
	// 消费者数量
	ConsumerN int64 = 1
)

// var ch chan interface{}
var (
	// cnt       int64 = 0
	err       error
	msgClient *client.Client
	// n         int64 = 5
)

// 模拟的 send 函数
func send(v []byte) {
	// fmt.Println(v)
	// ch <- v
	// if atomic.AddInt64(&cnt, 1) == int64(numMessages) {
	// 	wg.Done()
	// }
	msgClient.Push(v)
}

// 模拟的 receive 函数
func receive() {
	// fmt.Println(<-ch)
	// <-ch
	// if atomic.AddInt64(&cnt, 1) == int64(numMessages) {
	// 	wg.Done()
	// }
	// fmt.Println(msgClient.Pull())
	msgClient.Pull()
}

func parseArgs() {
	// 解析命令行参数
	rounds := flag.Int("r", 1, "number of rounds to run")
	numMsgs := flag.Int("m", 1, "number of messages to send")
	producerN := flag.Int64("p", 1, "number of producers")
	consumerN := flag.Int64("c", 1, "number of consumers")
	flag.Parse()

	// 更新全局变量
	// testCount, numMessages = *rounds, *numMsgs
	testCount, numMessages, ProducerN, ConsumerN = *rounds, *numMsgs, *producerN, *consumerN
	// fmt.Printf("Env: %d tests, %d producers, %d consumers, %d messages\n", testCount, minNumProducers, minNumConsumers, numMessages)
}

func Init() {
	logger.InitLogger("log.out")
	parseArgs()
	msgClient, err = client.NewClient("test", []string{"127.0.0.1:8080"}, &client.ClientConfig{
		InBufferSize:  50000,
		OutBufferSize: 50000,
	})
	if err != nil {
		log.Fatal(err)
	}
	msgClient.OpenSend([]string{"127.0.0.1:8080"}, &client.StreamGroupConfig{
		InitialNofSC: ProducerN,
		MinNofSC:     ProducerN,
		MaxNofSC:     ProducerN,
		RewardLimit:  0.1,
		PenaltyLimit: 0.1,
	})
	msgClient.OpenRecv([]string{"127.0.0.1:8080"}, &client.StreamGroupConfig{
		InitialNofSC: ConsumerN,
		MinNofSC:     ConsumerN,
		MaxNofSC:     ConsumerN,
		RewardLimit:  0.1,
		PenaltyLimit: 0.1,
	})
	// msgClient.OpenRecvWithSureProcessed([]string{"127.0.0.1:8080"}, &client.StreamGroupConfig{
	// 	InitialNofSC: ConsumerN,
	// 	MinNofSC:     ConsumerN,
	// 	MaxNofSC:     ConsumerN,
	// 	RewardLimit:  0.1,
	// 	PenaltyLimit: 0.1,
	// }, func(b []byte) error {
	// 	fmt.Println(b)
	// 	return nil
	// })
}

func main() {
	Init()
	// fmt.Println(0)
	// go func() {
	// 	log.Println(http.ListenAndServe(":6060", nil))
	// }()
	time.Sleep(2 * time.Second)
	// c := convergev2.NewConvergev2(10000)
	sumElapsedReceive := time.Duration(0)
	// numProducers := minNumProducers * scale
	// numConsumers := minNumConsumers * scale
	for j := 0; j < testCount; j++ {
		// cnt = 0
		// wg := &sync.WaitGroup{}
		// wg.Add(1)

		startReceive := time.Now()
		// 启动生产者
		for i := 0; i < numMessages; i++ {
			// go func(v int) {
			// for j := 0; j < numMessages/n; j++ {
			// send(v)

			// }
			// }(i)
			// fmt.Println("send", i)
			go send([]byte(string(i)))
		}

		for i := 0; i < numMessages; i++ {
			receive()
		}
		// time.Sleep(1 * time.Second)
		// startReceive := time.Now()
		// 启动消费者
		// fmt.Println(numMessages)
		// for i := 0; i < n; i++ {
		// 	wg.Add(1)
		// 	go func() {
		// 		for j := 0; j < numMessages/n; j++ {
		// 			receive()
		// 		}
		// 		wg.Done()
		// 	}()
		// }
		// wg.Wait()
		// time.Sleep(1 * time.Second)
		elapsedReceive := time.Since(startReceive)
		sumElapsedReceive += elapsedReceive
	}
	// time.Sleep(20 * time.Second)
	elapsedReceive := sumElapsedReceive / time.Duration(testCount)
	// fmt.Printf("Env: %d tests, %d producers, %d consumers, %d messages\n", testCount, numProducers, numConsumers, numMessages)
	fmt.Printf("Received %d messages in %s(mean)\n", numMessages, elapsedReceive)
	fmt.Printf("Receive Throughput: %.2f messages/sec(mean)\n\n", float64(numMessages)/elapsedReceive.Seconds())
	msgClient.Close()
}
