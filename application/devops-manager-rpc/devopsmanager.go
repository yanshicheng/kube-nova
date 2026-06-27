package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/config"
	channelserviceServer "github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/server/channelservice"
	pipelineconfigserviceServer "github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/server/pipelineconfigservice"
	projectserviceServer "github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/server/projectservice"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/tektonsync"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/yanshicheng/kube-nova/common/interceptors"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/devops-manager.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	ctx := svc.NewServiceContext(c)
	if c.Bootstrap.TektonStepSyncEnabled {
		syncTektonStepsOnStartup(ctx)
	} else {
		logx.Infof("Tekton 步骤启动同步已关闭")
	}

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterProjectServiceServer(grpcServer, projectserviceServer.NewProjectServiceServer(ctx))
		pb.RegisterChannelServiceServer(grpcServer, channelserviceServer.NewChannelServiceServer(ctx))
		pb.RegisterPipelineConfigServiceServer(grpcServer, pipelineconfigserviceServer.NewPipelineConfigServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()
	// 自定义拦截器
	s.AddUnaryInterceptors(interceptors.ServerErrorInterceptor())
	s.AddUnaryInterceptors(interceptors.ServerMetadataInterceptor())
	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}

func syncTektonStepsOnStartup(svcCtx *svc.ServiceContext) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := tektonsync.NewSyncer(svcCtx).SyncAllStepsToAllChannels(ctx, "system"); err != nil {
			logx.Errorf("Tekton 步骤启动同步失败: %v", err)
		}
	}()
}
