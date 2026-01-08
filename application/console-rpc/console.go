package main

import (
	"flag"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/config"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/crontab/jobs"
	repositoryserviceServer "github.com/yanshicheng/kube-nova/application/console-rpc/internal/server/repositoryservice"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/interceptors"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
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

	// 优雅退出时停止定时任务
	defer ctx.Stop()

	// ============================================================
	// 注册定时任务（在 ServiceContext 创建之后，避免循环导入）
	// ============================================================
	jobCount := jobs.SetupCronJobs(ctx, ctx.CronManager)
	logx.Infof("[Crontab] 定时任务注册完成, 成功注册 %d 个任务", jobCount)

	// 启动定时任务管理器
	if err := ctx.StartCronManager(); err != nil {
		logx.Errorf("[Crontab] 启动定时任务失败: %v", err)
	}

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
