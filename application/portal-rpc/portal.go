package main

import (
	"flag"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/config"
	portalserviceServer "github.com/yanshicheng/kube-nova/application/portal-rpc/internal/server/portalservice"
	sitemessagesServer "github.com/yanshicheng/kube-nova/application/portal-rpc/internal/server/sitemessagesservice"
	storageserviceServer "github.com/yanshicheng/kube-nova/application/portal-rpc/internal/server/storageservice"
	sysauthserviceServer "github.com/yanshicheng/kube-nova/application/portal-rpc/internal/server/sysauthservice"
	"github.com/yanshicheng/kube-nova/common/interceptors"

	alertsServer "github.com/yanshicheng/kube-nova/application/portal-rpc/internal/server/alertservice"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/portal.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterStorageServiceServer(grpcServer, storageserviceServer.NewStorageServiceServer(ctx))
		pb.RegisterSysAuthServiceServer(grpcServer, sysauthserviceServer.NewSysAuthServiceServer(ctx))
		pb.RegisterPortalServiceServer(grpcServer, portalserviceServer.NewPortalServiceServer(ctx))
		pb.RegisterAlertServiceServer(grpcServer, alertsServer.NewAlertServiceServer(ctx))
		pb.RegisterSiteMessagesServiceServer(grpcServer, sitemessagesServer.NewSiteMessagesServiceServer(ctx))
		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	// 自定义拦截器
	s.AddUnaryInterceptors(interceptors.ServerErrorInterceptor())
	s.AddUnaryInterceptors(interceptors.ServerMetadataInterceptor())
	fmt.Printf("Starting portal-rpc server at %s...\n", c.ListenOn)
	s.Start()
}
