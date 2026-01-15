package svc

import (
	"log"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/billing"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/config"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/consumer"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	syncOperator "github.com/yanshicheng/kube-nova/application/manager-rpc/internal/rsync/operator"
	types3 "github.com/yanshicheng/kube-nova/application/manager-rpc/internal/rsync/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/incremental"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/alertservice"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/portalservice"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/storageservice"
	"github.com/yanshicheng/kube-nova/common/interceptors"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config config.Config
	Cache  *redis.Redis
	//ControllerRpc         controllerservice.ControllerService
	OnecClusterModel              model.OnecClusterModel
	OnecClusterAppModel           model.OnecClusterAppModel
	OnecClusterNodeModel          model.OnecClusterNodeModel
	OnecClusterAuthModel          model.OnecClusterAuthModel
	OnecClusterNetworkModel       model.OnecClusterNetworkModel
	OnecClusterResourceModel      model.OnecClusterResourceModel
	OnecProjectModel              model.OnecProjectModel
	OnecProjectAdminModel         model.OnecProjectAdminModel
	OnecProjectClusterModel       model.OnecProjectClusterModel
	OnecProjectWorkspaceModel     model.OnecProjectWorkspaceModel
	OnecProjectApplication        model.OnecProjectApplicationModel
	OnecProjectVersion            model.OnecProjectVersionModel
	OnecProjectAuditLog           model.OnecProjectAuditLogModel
	OnecBillingPriceConfigModel   model.OnecBillingPriceConfigModel
	OnecBillingStatementModel     model.OnecBillingStatementModel
	OnecBillingConfigBindingModel model.OnecBillingConfigBindingModel
	Storage                       storageservice.StorageService
	PortalRpc                     portalservice.PortalService
	BillingService                billing.Service
	K8sManager                    cluster.Manager
	SyncOperator                  types3.SyncService
	AlertRuleFilesModel           model.AlertRuleFilesModel
	AlertRuleGroupsModel          model.AlertRuleGroupsModel
	AlertInstancesModel           model.AlertInstancesModel
	AlertRulesModel               model.AlertRulesModel
	AlertRpc                      alertservice.AlertService
	// 消费者
	AlertConsumer consumer.Consumer

	// 增量同步管理器
	IncrementalSyncManager *incremental.Manager
}

func NewServiceContext(c config.Config) *ServiceContext {
	sqlConn := sqlx.NewMysql(c.Mysql.DataSource)
	rawDB, err := sqlConn.RawDB()
	if err != nil {
		log.Fatal(err) // 处理错误
	}
	// 配置连接池参数
	rawDB.SetMaxOpenConns(c.Mysql.MaxOpenConns)       // 最大打开连接数
	rawDB.SetMaxIdleConns(c.Mysql.MaxIdleConns)       // 最大空闲连接数
	rawDB.SetConnMaxLifetime(c.Mysql.ConnMaxLifetime) // 连接的最大生命周期
	//controllerRpc := zrpc.MustNewClient(c.ControllerRpc, zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()))
	listenAddr := c.ListenOn
	if strings.HasPrefix(listenAddr, "0.0.0.0:") {
		listenAddr = strings.Replace(listenAddr, "0.0.0.0:", "127.0.0.1:", 1)
	}
	managerRpcConf := zrpc.RpcClientConf{
		Endpoints: []string{listenAddr}, // 使用自己的监听地址
		NonBlock:  true,
		Timeout:   30000,
	}
	managerRpc := zrpc.MustNewClient(managerRpcConf, zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()))
	alertRpc := zrpc.MustNewClient(c.PortalRpc, zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()))

	// ==================== 初始化 Redis ====================
	rds := redis.MustNewRedis(c.Cache)

	// ==================== 初始化 Model ====================
	clusterModel := model.NewOnecClusterModel(sqlConn, c.DBCache)
	clusterNodeModel := model.NewOnecClusterNodeModel(sqlConn, c.DBCache)
	clusterResourceModel := model.NewOnecClusterResourceModel(sqlConn, c.DBCache)
	clusterNetworkModel := model.NewOnecClusterNetworkModel(sqlConn, c.DBCache)
	projectModel := model.NewOnecProjectModel(sqlConn, c.DBCache)
	projectClusterModel := model.NewOnecProjectClusterModel(sqlConn, c.DBCache)
	projectWorkspaceModel := model.NewOnecProjectWorkspaceModel(sqlConn, c.DBCache)
	projectApplicationModel := model.NewOnecProjectApplicationModel(sqlConn, c.DBCache)
	projectVersionModel := model.NewOnecProjectVersionModel(sqlConn, c.DBCache)
	projectAuditLogModel := model.NewOnecProjectAuditLogModel(sqlConn, c.DBCache)

	// ==================== 初始化 K8s Manager ====================
	k8sManager := cluster.NewManager(managerservice.NewManagerService(managerRpc), rds)

	// ==================== 初始化资源同步 ====================
	resourceSync := syncOperator.NewClusterResourceSync(syncOperator.ClusterResourceSyncConfig{
		K8sManager:                  k8sManager,
		ClusterModel:                clusterModel,
		ClusterNodeModel:            clusterNodeModel,
		ClusterResourceModel:        clusterResourceModel,
		ClusterNetwork:              clusterNetworkModel,
		ProjectModel:                projectModel,
		ProjectClusterResourceModel: projectClusterModel,
		ProjectWorkspaceModel:       projectWorkspaceModel,
		ProjectApplication:          projectApplicationModel,
		ProjectApplicationVersion:   projectVersionModel,
		ProjectAuditModel:           projectAuditLogModel,
	})

	// ==================== 初始化账单服务 ====================
	billingService := billing.NewService(billing.ServiceConfig{
		ClusterModel:              clusterModel,
		ProjectModel:              projectModel,
		ProjectClusterModel:       projectClusterModel,
		ProjectWorkspaceModel:     projectWorkspaceModel,
		ProjectApplicationModel:   projectApplicationModel,
		BillingPriceConfigModel:   model.NewOnecBillingPriceConfigModel(sqlConn, c.DBCache),
		BillingConfigBindingModel: model.NewOnecBillingConfigBindingModel(sqlConn, c.DBCache),
		BillingStatementModel:     model.NewOnecBillingStatementModel(sqlConn, c.DBCache),
	})

	// ==================== 创建增量同步管理器 ====================
	incrementalManager := incremental.NewManager(incremental.ManagerConfig{
		NodeID:          "",
		Redis:           rds,
		K8sManager:      k8sManager,
		ClusterModel:    clusterModel,
		WorkerCount:     10,               // 并发处理数
		EventBuffer:     2000,             // 事件缓冲区
		DedupeWindow:    5 * time.Second,  // 去重窗口
		LockTTL:         30 * time.Second, // 分布式锁 TTL
		ProcessTimeout:  25 * time.Second, // 单事件处理超时
		EnableAutoRenew: true,             // 启用锁自动续期
	})

	svcCtx := &ServiceContext{
		Config: c,
		Cache:  rds,
		//ControllerRpc:         controllerservice.NewControllerService(controllerRpc),
		Storage:                       storageservice.NewStorageService(zrpc.MustNewClient(c.PortalRpc)),
		PortalRpc:                     portalservice.NewPortalService(alertRpc),
		OnecClusterModel:              clusterModel,
		OnecClusterAppModel:           model.NewOnecClusterAppModel(sqlConn, c.DBCache),
		OnecClusterNodeModel:          clusterNodeModel,
		OnecClusterAuthModel:          model.NewOnecClusterAuthModel(sqlConn, c.DBCache),
		OnecClusterNetworkModel:       clusterNetworkModel,
		OnecClusterResourceModel:      clusterResourceModel,
		OnecProjectModel:              projectModel,
		OnecProjectAdminModel:         model.NewOnecProjectAdminModel(sqlConn, c.DBCache),
		OnecProjectClusterModel:       projectClusterModel,
		OnecProjectWorkspaceModel:     projectWorkspaceModel,
		OnecProjectApplication:        projectApplicationModel,
		OnecProjectVersion:            projectVersionModel,
		OnecProjectAuditLog:           projectAuditLogModel,
		OnecBillingPriceConfigModel:   model.NewOnecBillingPriceConfigModel(sqlConn, c.DBCache),
		OnecBillingStatementModel:     model.NewOnecBillingStatementModel(sqlConn, c.DBCache),
		OnecBillingConfigBindingModel: model.NewOnecBillingConfigBindingModel(sqlConn, c.DBCache),
		BillingService:                billingService,
		K8sManager:                    k8sManager,
		SyncOperator:                  resourceSync,
		AlertRuleGroupsModel:          model.NewAlertRuleGroupsModel(sqlConn, c.DBCache),
		AlertRuleFilesModel:           model.NewAlertRuleFilesModel(sqlConn, c.DBCache),
		AlertInstancesModel:           model.NewAlertInstancesModel(sqlConn, c.DBCache),
		AlertRulesModel:               model.NewAlertRulesModel(sqlConn, c.DBCache),
		AlertRpc:                      alertservice.NewAlertService(alertRpc),

		// 增量同步管理器
		IncrementalSyncManager: incrementalManager,
	}

	return svcCtx
}
