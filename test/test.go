package main

import (
	"flag"
	"fmt"

	// "log"
	// "net/http"
	_ "net/http/pprof"
	"time"

	// "sync"
	// "sync/atomic"
	client "github.com/ailkaya/goport/client"
	"github.com/ailkaya/goport/common/logger"
	// "google.golang.org/grpc"
	// "google.golang.org/grpc/insecure"
)

// var data = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.0"

var (
	// 每次测试的轮数
	testCount = 1
	// 每轮发送的消息数量
	numMessages = 1
	// 生产者数量
	minNumProducers = 1
	// 消费者数量
	minNumConsumers = 1
	// 扩展因子(递增生产者和消费者数量)
	scaleFactor = 1
)

// var ch chan interface{}
var clientset *client.ClientSet
var cnt int64 = 0
var err error

// 模拟的 send 函数
func send(v interface{}) {
	// fmt.Println(v)
	// ch <- v
	// if atomic.AddInt64(&cnt, 1) == int64(numMessages) {
	// 	wg.Done()
	// }
	clientset.Push("test", v)
}

// 模拟的 receive 函数
func receive() {
	// fmt.Println(<-ch)
	// <-ch
	// if atomic.AddInt64(&cnt, 1) == int64(numMessages) {
	// 	wg.Done()
	// }
	clientset.Pull("test")
	// if err != nil {
	// 	panic(err)
	// }
}

func parseArgs() {
	// 解析命令行参数
	rounds := flag.Int("r", 1, "number of rounds to run")
	numMsgs := flag.Int("m", 1, "number of messages to send")
	scale := flag.Int("s", 1, "scale factor")
	flag.Parse()

	// 更新全局变量
	testCount, numMessages, scaleFactor = *rounds, *numMsgs, *scale
	// fmt.Printf("Env: %d tests, %d producers, %d consumers, %d messages\n", testCount, minNumProducers, minNumConsumers, numMessages)
}

func Init() {
	logger.InitLogger("log.out")
	clientset, err = client.NewClientSet([]string{"127.0.0.1:8080"}, &client.SetOption{
		Option: &client.Option{
			SenderNum:         4,
			ReceiverNum:       3,
			SendBufferSize:    10000,
			ReceiveBufferSize: 1000,
		},
	})
	if err != nil {
		panic(err)
	}
	clientset.RegisterTopic([]string{"127.0.0.1:8080"}, "test")
	parseArgs()
	// ch = make(chan interface{}, 10000000)
}

func main() {
	Init()
	// go func() {
	// 	log.Println(http.ListenAndServe(":6060", nil))
	// }()
	// time.Sleep(8 * time.Second)
	// c := convergev2.NewConvergev2(10000)
	for scale := 1; scale <= scaleFactor; scale++ {
		sumElapsedReceive := time.Duration(0)
		// numProducers := minNumProducers * scale
		// numConsumers := minNumConsumers * scale
		for j := 0; j < testCount; j++ {
			cnt = 0
			// wg := &sync.WaitGroup{}
			// wg.Add(1)

			startReceive := time.Now()
			// 启动生产者
			for i := 0; i < numMessages; i++ {
				go func(v int) {
					// for j := 0; j < numMessages/n; j++ {
					send(v)

					// }
				}(i)
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

		elapsedReceive := sumElapsedReceive / time.Duration(testCount)
		// fmt.Printf("Env: %d tests, %d producers, %d consumers, %d messages\n", testCount, numProducers, numConsumers, numMessages)
		fmt.Printf("Received %d messages in %s(mean)\n", numMessages, elapsedReceive)
		fmt.Printf("Receive Throughput: %.2f messages/sec(mean)\n\n", float64(numMessages)/elapsedReceive.Seconds())
	}
	clientset.Close()
}
