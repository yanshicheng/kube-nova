package svc

import (
	"log"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/config"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/crontab"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	repository2 "github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/cluster"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/common/interceptors"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config                      config.Config
	Cache                       *redis.Redis
	HarborManager               *cluster.HarborManager
	ContainerRegistryModel      repository.ContainerRegistryModel
	RepositoryClusterModel      repository2.RegistryClusterModel
	RegistryProjectBindingModel repository2.RegistryProjectBindingModel
	ManagerRpc                  managerservice.ManagerService

	// 定时任务管理器
	CronManager *crontab.Manager
}

func NewServiceContext(c config.Config) *ServiceContext {
	sqlConn := sqlx.NewMysql(c.Mysql.DataSource)
	rawDB, err := sqlConn.RawDB()
	if err != nil {
		log.Fatal(err)
	}
	// 配置连接池参数
	rawDB.SetMaxOpenConns(c.Mysql.MaxOpenConns)
	rawDB.SetMaxIdleConns(c.Mysql.MaxIdleConns)
	rawDB.SetConnMaxLifetime(c.Mysql.ConnMaxLifetime)

	// 创建 Redis 客户端
	rds := redis.MustNewRedis(c.Cache)
	managerRpc := zrpc.MustNewClient(c.ManagerRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	// 创建 ServiceContext
	svcCtx := &ServiceContext{
		Config:                      c,
		Cache:                       rds,
		HarborManager:               cluster.NewHarborManager(repository.NewContainerRegistryModel(sqlConn, c.DBCache)),
		ContainerRegistryModel:      repository.NewContainerRegistryModel(sqlx.NewMysql(c.Mysql.DataSource), c.DBCache),
		RepositoryClusterModel:      repository2.NewRegistryClusterModel(sqlx.NewMysql(c.Mysql.DataSource), c.DBCache),
		RegistryProjectBindingModel: repository2.NewRegistryProjectBindingModel(sqlx.NewMysql(c.Mysql.DataSource), c.DBCache),
		ManagerRpc:                  managerservice.NewManagerService(managerRpc)}

	// 初始化定时任务管理器（只创建 Manager，不注册 jobs）
	svcCtx.CronManager = initCronManager(rds)

	return svcCtx
}

// initCronManager 初始化定时任务管理器
// 注意：这里只创建 Manager，不注册具体的 jobs（避免循环导入）
// jobs 的注册在 main.go 中通过 SetupCronJobs 完成
func initCronManager(rds *redis.Redis) *crontab.Manager {
	// 创建定时任务管理器
	manager := crontab.NewManager(crontab.ManagerConfig{
		NodeID:                "",    // 为空则自动生成 (hostname-pid)
		Redis:                 rds,   // Redis 客户端，用于分布式锁
		EnableDistributedLock: true,  // K8s 多 Pod 部署必须开启
		RecoverPanic:          true,  // 自动恢复 panic
		LockTTLMultiplier:     1.5,   // 锁过期时间倍数
		EnableAutoRenew:       false, // 是否启用锁自动续期
	})

	return manager
}

// StartCronManager 启动定时任务管理器
// 在 main.go 中注册完 jobs 后调用
func (s *ServiceContext) StartCronManager() error {
	if s.CronManager == nil {
		return nil
	}

	if err := s.CronManager.Start(); err != nil {
		logx.Errorf("[Crontab] 定时任务管理器启动失败: %v", err)
		return err
	}

	logx.Infof("[Crontab] 定时任务管理器启动成功, NodeID=%s", s.CronManager.GetNodeID())
	return nil
}

// Stop 停止 ServiceContext 中的资源
// 在 main.go 中调用: defer svcCtx.Stop()
func (s *ServiceContext) Stop() {
	if s.CronManager != nil {
		if err := s.CronManager.Stop(); err != nil {
			logx.Errorf("[Crontab] 定时任务管理器停止失败: %v", err)
		} else {
			logx.Info("[Crontab] 定时任务管理器已停止")
		}
	}
}
