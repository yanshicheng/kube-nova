// main.go
package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/config"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/consumer"
	managerserviceServer "github.com/yanshicheng/kube-nova/application/manager-rpc/internal/server/managerservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/interceptors"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/manager.yaml", "the config file")

func main() {
	flag.Parse()

	// ==================== 加载配置 ====================
	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	// ==================== 初始化服务上下文 ====================
	// 这会初始化所有依赖：数据库连接、Redis、K8s Manager、增量同步管理器等
	svcCtx := svc.NewServiceContext(c)

	// ==================== 创建 RPC 服务 ====================
	rpcServer := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		// 注册 ManagerService
		pb.RegisterManagerServiceServer(
			grpcServer,
			managerserviceServer.NewManagerServiceServer(svcCtx),
		)

		// 开发/测试模式下启用 gRPC 反射，方便调试
		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	// 添加自定义拦截器
	rpcServer.AddUnaryInterceptors(interceptors.ServerMetadataInterceptor())
	rpcServer.AddUnaryInterceptors(interceptors.ServerErrorInterceptor())

	// ==================== 创建告警消费者服务 ====================
	alertConsumer := consumer.NewAlertConsumer(&consumer.AlertConsumerDeps{
		Redis:                     svcCtx.Cache,
		AlertInstancesModel:       svcCtx.AlertInstancesModel,
		OnecClusterModel:          svcCtx.OnecClusterModel,
		OnecProjectModel:          svcCtx.OnecProjectModel,
		OnecProjectClusterModel:   svcCtx.OnecProjectClusterModel,
		OnecProjectWorkspaceModel: svcCtx.OnecProjectWorkspaceModel,
		AlertRpc:                  svcCtx.AlertRpc,
	})
	alertService := consumer.NewAlertConsumerService(alertConsumer)

	// ==================== 创建增量同步服务 ====================
	// 这个服务会在启动时等待 RPC 服务就绪，然后再初始化集群监听器
	incrementalService := watch.NewIncrementalSyncService(watch.IncrementalSyncServiceConfig{
		SvcCtx:         svcCtx,
		RPCAddr:        c.ListenOn,             // RPC 监听地址
		StartupTimeout: 60 * time.Second,       // 等待 RPC 就绪的超时时间
		CheckInterval:  500 * time.Millisecond, // 健康检查间隔
	})

	// ==================== 使用 ServiceGroup 管理所有服务 ====================
	// ServiceGroup 会：
	// 1. 并发启动所有服务
	// 2. 监听系统信号 (SIGTERM, SIGINT 等)
	// 3. 收到信号时优雅关闭所有服务
	group := service.NewServiceGroup()
	defer group.Stop() // 确保程序退出时停止所有服务

	// 添加服务（添加顺序决定了日志输出顺序，但启动是并发的）
	group.Add(rpcServer)          // RPC 服务
	group.Add(alertService)       // 告警消费者服务
	group.Add(incrementalService) // 增量同步服务（会等待 RPC 就绪）

	fmt.Printf("Starting manager-rpc server at %s...\n", c.ListenOn)
	logx.Info("[Main] 所有服务启动中...")

	// 启动所有服务并阻塞
	// ServiceGroup.Start() 会：
	// 1. 为每个服务创建一个 goroutine 并调用其 Start() 方法
	// 2. 监听系统信号
	// 3. 收到信号时调用每个服务的 Stop() 方法
	// 4. 等待所有服务停止后返回
	group.Start()

	logx.Info("[Main] 所有服务已停止，程序退出")
}
