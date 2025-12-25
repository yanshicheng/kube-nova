package main

import (
	"flag"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/config"
	repositoryserviceServer "github.com/yanshicheng/kube-nova/application/console-rpc/internal/server/repositoryservice"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/interceptors"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/console.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterRepositoryServiceServer(grpcServer, repositoryserviceServer.NewRepositoryServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	// 自定义拦截器
	s.AddUnaryInterceptors(interceptors.ServerErrorInterceptor())
	s.AddUnaryInterceptors(interceptors.ServerMetadataInterceptor())
	fmt.Printf("Starting console-rpc server at %s...\n", c.ListenOn)
	s.Start()
}
