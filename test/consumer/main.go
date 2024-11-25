package main

import (
	"fmt"
	// "time"

	gp "github.com/ailkaya/goport/client"
)

func main() {
	client, err := gp.NewClient("test", []string{"127.0.0.1:8080"}, nil)
	if err != nil {
		panic(err)
	}
	client.OpenRecv([]string{"127.0.0.1:8080"}, nil)
	for {
		fmt.Println(client.Pull())
	}
	// client.Close()
}
