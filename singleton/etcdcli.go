package singleton

import (
	"go.etcd.io/etcd/clientv3"
	"sync"
	"time"
)

const (
	Addr        = "localhost:2379"
	DialTimeout = 5 * time.Second
	BasePath    = "goulette/keepalive"
)

var (
	cli  *clientv3.Client
	once = sync.Once{}
)

func GetEtcdCli() *clientv3.Client {
	once.Do(func() {
		var err error
		cli, err = clientv3.New(clientv3.Config{
			Endpoints:   []string{Addr},
			DialTimeout: DialTimeout,
		})
		if err != nil {
			panic(err)
		}
	})
	return cli
}
