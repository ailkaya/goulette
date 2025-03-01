package sentry

import (
	"context"
	"fmt"
	"go.etcd.io/etcd/clientv3"
	"time"
)

// GetEtcdLeader 查询 etcd 集群中的 leader
func GetEtcdLeader(endpoints []string) (string, error) {
	// 创建 etcd 客户端
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create etcd client: %w", err)
	}
	defer cli.Close()

	// 获取成员信息
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := cli.MemberList(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get member list: %w", err)
	}

	// 查找 Leader
	for _, member := range resp.Members {
		if member.IsLeader {
			return member.ClientURLs[0], nil
		}
	}

	return "", fmt.Errorf("no leader found in the etcd cluster")
}
