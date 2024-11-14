package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/ailkaya/goport/broker"
	"github.com/ailkaya/goport/common/config"
	"github.com/ailkaya/goport/common/logger"
	"github.com/ailkaya/goport/service"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe(":6060", nil))
	}()
	// 初始化配置
	config.InitConf()
	// 初始化日志
	logger.InitLogger(config.Conf.APP.LogPath)
	// 监听本地1234端口
	addr := fmt.Sprintf("127.0.0.1:%v", config.Conf.APP.Port)
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
	klog.Infof("start server, listen: %s\n", config.Conf.APP.Port)
	// 注册服务
	broker.RegisterBrokerServiceServer(grpcServer, service.NewService())
	// 启动服务
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
	klog.Warningf("stop server")
	// 关闭服务
	grpcServer.GracefulStop()
}
