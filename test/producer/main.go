package main

import (
	"time"

	gp "github.com/ailkaya/goport/client"
)

func main() {
	client, err := gp.NewClient("test", []string{"127.0.0.1:8080"}, nil)
	if err != nil {
		panic(err)
	}
	client.OpenSend([]string{"127.0.0.1:8080"}, nil)
	for i := 0; i < 10; i++ {
		client.Push([]byte("hello world"))
	}
	time.Sleep(time.Second * 5)
	// client.Close()
}
