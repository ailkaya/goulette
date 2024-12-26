package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ailkaya/goport/client"
)

func TestProcessor() {
	preProcessorController := client.NewPreProcessorController()
	N, M, T := 10, 1000, 1000000
	in := make(chan []byte, T)
	processorSlice := make([]client.IPreProcessor, N)
	cnt := make([]int, N)
	for i := 0; i < N; i++ {
		processorSlice[i] = client.NewPreProcessor(M).Cite(preProcessorController)
		preProcessorController.RegisterContainer(processorSlice[i].Register(in))
	}

	for i := 0; i < T; i++ {
		in <- []byte(string(i))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < N; i++ {
		// fmt.Println(processorSlice[i])
		go func(j int) {
			for {
				select {
				case <-ctx.Done():
					fmt.Println("done")
					return
				default:
					// fmt.Println(j)
					// fmt.Println(j, processorSlice[j].Process())
					processorSlice[j].Process()
					cnt[j]++
				}
			}
		}(i)
	}

	preProcessorController.Start()
	time.Sleep(time.Second * 10)
	for i := 0; i < N; i++ {
		fmt.Println(cnt[i])
	}
	close(in)
}
