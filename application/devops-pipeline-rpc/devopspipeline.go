package main

import (
	"flag"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/config"
	executionservicelogic "github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/logic/executionservice"
	executionserviceServer "github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/server/executionservice"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"

	"github.com/yanshicheng/kube-nova/common/interceptors"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/devops-pipeline.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	ctx := svc.NewServiceContext(c)
	executionservicelogic.StartPipelineRunReconciler(ctx)
	executionservicelogic.StartTektonScheduleReconciler(ctx)
	executionservicelogic.StartTektonPrunerReconciler(ctx)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterExecutionServiceServer(grpcServer, executionserviceServer.NewExecutionServiceServer(ctx))

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
