package main

import (
	"context"
	"fmt"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"go.etcd.io/etcd/server/v3/embed"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"goulette/sentry"
	"goulette/sentry/pb"
	"k8s.io/klog/v2"
	"log"
	"net"
	"time"
)

const (
	EtcdPort = 2379
	GrpcPort = 2380
)

func main() {
	StartEtcdService()
	StartGrpcService()
}

// TODO: 完善配置以及使用配置文件保存并读取相关配置，配置集群模式
func StartEtcdService() {
	cfg := embed.NewConfig()
	cfg.Dir = "default.etcd"
	e, err := embed.StartEtcd(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer e.Close()
	select {
	case <-e.Server.ReadyNotify():
		log.Printf("Server is ready!")
	case <-time.After(60 * time.Second):
		e.Server.Stop() // trigger a shutdown
		log.Printf("Server took too long to start!")
	}
}

func StartGrpcService() {
	addr := fmt.Sprintf("127.0.0.1:%v", GrpcPort)
	lis, _ := net.Listen("tcp", addr)
	// 定义grpc恢复选项
	opts := []grpc_recovery.Option{
		grpc_recovery.WithRecoveryHandlerContext(func(ctx context.Context, rec interface{}) (err error) {
			// 记录恢复信息
			klog.Warningf("Recovered in f %v", rec)
			// 返回内部错误
			return status.Errorf(codes.Internal, "Recovered from panic")
		}),
	}
	// 创建grpc服务器
	grpcServer := grpc.NewServer(
		// 设置保活策略
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second, // 最小保活时间
			PermitWithoutStream: true,            // Allow pings even when there are no active streams
		}),
		// 设置保活参数
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    2 * time.Hour,
			Timeout: 20 * time.Second,
		}),
		// 设置恢复拦截器
		grpc.ChainUnaryInterceptor(
			grpc_recovery.UnaryServerInterceptor(opts...),
		),
		// 设置恢复流拦截器
		grpc.StreamInterceptor(grpc_recovery.StreamServerInterceptor(opts...)),
	)
	klog.Infof("start server, listen: %v\n", GrpcPort)
	// 注册服务
	pb.RegisterSentryServiceServer(grpcServer, sentry.NewService(sentry.NewConsistentHash()))
	// 启动服务
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
	klog.Warningf("stop server")
	// 关闭服务
	grpcServer.GracefulStop()
}
