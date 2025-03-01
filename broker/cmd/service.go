package main

import (
	"context"
	"fmt"
	"github.com/ailkaya/goulette/broker"
	"github.com/ailkaya/goulette/broker/common/config"
	"github.com/ailkaya/goulette/broker/common/logger"
	"github.com/ailkaya/goulette/broker/pb"
	sentryPb "github.com/ailkaya/goulette/sentry/pb"
	"github.com/ailkaya/goulette/singleton"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"go.etcd.io/etcd/clientv3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"log"
	"net"
	_ "net/http/pprof"
	"time"
)

const (
	LocalAddr     = "127.0.0.1:8081"
	EtcdAddress   = "127.0.0.1:2379"
	SentryAddress = "127.0.0.1:2380"
	BasePath      = singleton.BasePath + "/broker/"
)

func main() {
	// go func() {
	// 	log.Println(http.ListenAndServe(":6060", nil))
	// }()

	cli := singleton.GetEtcdCli()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lease := clientv3.NewLease(cli)
	grant, err := lease.Create(ctx, 10)
	if err != nil {
		log.Fatal(err)
	}
	_, err = lease.KeepAlive(ctx, clientv3.LeaseID(grant.ID))
	if err != nil {
		log.Fatal(err)
	}
	_, err = cli.Put(ctx, BasePath+LocalAddr, "lease in 10s", clientv3.WithLease(clientv3.LeaseID(grant.ID)))
	if err != nil {
		log.Fatal(err)
	}

	// 调用sentry的rpc接口注册
	conn, err := grpc.NewClient(SentryAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	gCli := sentryPb.NewSentryServiceClient(conn)
	// 将自己注册到哈希环上
	_, err = gCli.RegisterBroker(ctx, &sentryPb.RegisterBrokerParams{
		Addr:         LocalAddr,
		WeightFactor: 1,
	})
	if err != nil {
		log.Fatal(err)
	}

	// TODO: 协调lease与grpc service，使得其中一个出现问题后使另一个也停止
	StartGrpcServer()
}

func StartGrpcServer() {
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
	pb.RegisterBrokerServiceServer(grpcServer, broker.NewService())
	// 启动服务
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
	klog.Warningf("stop server")
	// 关闭服务
	grpcServer.GracefulStop()
}
