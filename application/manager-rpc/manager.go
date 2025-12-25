package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/config"
	managerserviceServer "github.com/yanshicheng/kube-nova/application/manager-rpc/internal/server/managerservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/interceptors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/consumer"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/manager.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterManagerServiceServer(grpcServer, managerserviceServer.NewManagerServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	// 自定义拦截器
	s.AddUnaryInterceptors(interceptors.ServerMetadataInterceptor())
	s.AddUnaryInterceptors(interceptors.ServerErrorInterceptor())

	// ==================== 启动告警消费者 ====================
	alertConsumer := consumer.NewAlertConsumer(&consumer.AlertConsumerDeps{
		Redis:                     ctx.Cache,
		AlertInstancesModel:       ctx.AlertInstancesModel,
		OnecClusterModel:          ctx.OnecClusterModel,
		OnecProjectModel:          ctx.OnecProjectModel,
		OnecProjectClusterModel:   ctx.OnecProjectClusterModel,
		OnecProjectWorkspaceModel: ctx.OnecProjectWorkspaceModel,
		AlertRpc:                  ctx.AlertRpc,
	})

	// 启动消费者
	if err := alertConsumer.Start(context.Background()); err != nil {
		panic(err)
	}

	// ==================== 优雅关闭 ====================
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)
		sig := <-sigCh

		fmt.Printf("\n🛑 收到信号: %v, 开始优雅关闭...\n", sig)

		// 1. 先停止消费者（不再接收新消息）
		fmt.Println("⏳ 正在停止告警消费者...")
		if err := alertConsumer.Stop(); err != nil {
			panic(err)
		}

		// 2. 再停止RPC服务
		fmt.Println("⏳ 正在停止RPC服务...")
		s.Stop()
	}()

	fmt.Printf("🚀 Starting manager-rpc server at %s...\n", c.ListenOn)
	s.Start()
}
