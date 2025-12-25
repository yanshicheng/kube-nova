package main

import (
	"flag"
	"fmt"

	{{.imports}}

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/yanshicheng/kube-nova/common/interceptors"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/{{.serviceName}}.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c,conf.UseEnv())
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
{{range .serviceNames}}       {{.Pkg}}.Register{{.GRPCService}}Server(grpcServer, {{.ServerPkg}}.New{{.Service}}Server(ctx))
{{end}}
		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()
	// 自定义拦截器
	s.AddUnaryInterceptors(interceptors.ServerErrorInterceptor())
	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
