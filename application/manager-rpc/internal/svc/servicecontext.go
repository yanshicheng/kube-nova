package svc

import (
	"log"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/billing"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/config"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/consumer"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	syncOperator "github.com/yanshicheng/kube-nova/application/manager-rpc/internal/rsync/operator"
	types3 "github.com/yanshicheng/kube-nova/application/manager-rpc/internal/rsync/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/alertservice"
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

	managerK8s := cluster.NewManager(managerservice.NewManagerService(managerRpc), redis.MustNewRedis(c.Cache))
	resourceSync := syncOperator.NewClusterResourceSync(syncOperator.ClusterResourceSyncConfig{
		K8sManager:                  managerK8s,
		ClusterModel:                model.NewOnecClusterModel(sqlConn, c.DBCache),
		ClusterNodeModel:            model.NewOnecClusterNodeModel(sqlConn, c.DBCache),
		ClusterResourceModel:        model.NewOnecClusterResourceModel(sqlConn, c.DBCache),
		ClusterNetwork:              model.NewOnecClusterNetworkModel(sqlConn, c.DBCache),
		ProjectModel:                model.NewOnecProjectModel(sqlConn, c.DBCache),
		ProjectClusterResourceModel: model.NewOnecProjectClusterModel(sqlConn, c.DBCache),
		ProjectWorkspaceModel:       model.NewOnecProjectWorkspaceModel(sqlConn, c.DBCache),
		ProjectApplication:          model.NewOnecProjectApplicationModel(sqlConn, c.DBCache),
		ProjectApplicationVersion:   model.NewOnecProjectVersionModel(sqlConn, c.DBCache),
		ProjectAuditModel:           model.NewOnecProjectAuditLogModel(sqlConn, c.DBCache),
	})
	// 初始化账单服务
	billingService := billing.NewService(billing.ServiceConfig{
		ClusterModel:              model.NewOnecClusterModel(sqlConn, c.DBCache),
		ProjectModel:              model.NewOnecProjectModel(sqlConn, c.DBCache),
		ProjectClusterModel:       model.NewOnecProjectClusterModel(sqlConn, c.DBCache),
		ProjectWorkspaceModel:     model.NewOnecProjectWorkspaceModel(sqlConn, c.DBCache),
		ProjectApplicationModel:   model.NewOnecProjectApplicationModel(sqlConn, c.DBCache),
		BillingPriceConfigModel:   model.NewOnecBillingPriceConfigModel(sqlConn, c.DBCache),
		BillingConfigBindingModel: model.NewOnecBillingConfigBindingModel(sqlConn, c.DBCache),
		BillingStatementModel:     model.NewOnecBillingStatementModel(sqlConn, c.DBCache),
	})

	// 在你的 logic 或 handler 中调用
	//err = billingService.GenerateByClusterAndProject(
	//	context.Background(),
	//	"94d2b889-f138-44b5-83d8-3bf28d3fcfb3",
	//	4,
	//	&billing.GenerateOption{
	//		StatementType: billing.StatementTypeDaily,
	//		CreatedBy:     "admin",
	//	},
	//)
	//if err != nil {
	//	log.Fatal("生成账单失败: %v", err)
	//}
	return &ServiceContext{
		Config: c,
		Cache:  redis.MustNewRedis(c.Cache),
		//ControllerRpc:         controllerservice.NewControllerService(controllerRpc),
		Storage:                       storageservice.NewStorageService(zrpc.MustNewClient(c.PortalRpc)),
		OnecClusterModel:              model.NewOnecClusterModel(sqlConn, c.DBCache),
		OnecClusterAppModel:           model.NewOnecClusterAppModel(sqlConn, c.DBCache),
		OnecClusterNodeModel:          model.NewOnecClusterNodeModel(sqlConn, c.DBCache),
		OnecClusterAuthModel:          model.NewOnecClusterAuthModel(sqlConn, c.DBCache),
		OnecClusterNetworkModel:       model.NewOnecClusterNetworkModel(sqlConn, c.DBCache),
		OnecClusterResourceModel:      model.NewOnecClusterResourceModel(sqlConn, c.DBCache),
		OnecProjectModel:              model.NewOnecProjectModel(sqlConn, c.DBCache),
		OnecProjectAdminModel:         model.NewOnecProjectAdminModel(sqlConn, c.DBCache),
		OnecProjectClusterModel:       model.NewOnecProjectClusterModel(sqlConn, c.DBCache),
		OnecProjectWorkspaceModel:     model.NewOnecProjectWorkspaceModel(sqlConn, c.DBCache),
		OnecProjectApplication:        model.NewOnecProjectApplicationModel(sqlConn, c.DBCache),
		OnecProjectVersion:            model.NewOnecProjectVersionModel(sqlConn, c.DBCache),
		OnecProjectAuditLog:           model.NewOnecProjectAuditLogModel(sqlConn, c.DBCache),
		OnecBillingPriceConfigModel:   model.NewOnecBillingPriceConfigModel(sqlConn, c.DBCache),
		OnecBillingStatementModel:     model.NewOnecBillingStatementModel(sqlConn, c.DBCache),
		OnecBillingConfigBindingModel: model.NewOnecBillingConfigBindingModel(sqlConn, c.DBCache),
		BillingService:                billingService,
		K8sManager:                    managerK8s,
		SyncOperator:                  resourceSync,
		AlertRuleGroupsModel:          model.NewAlertRuleGroupsModel(sqlConn, c.DBCache),
		AlertRuleFilesModel:           model.NewAlertRuleFilesModel(sqlConn, c.DBCache),
		AlertInstancesModel:           model.NewAlertInstancesModel(sqlConn, c.DBCache),
		AlertRulesModel:               model.NewAlertRulesModel(sqlConn, c.DBCache),
		AlertRpc:                      alertservice.NewAlertService(alertRpc),

		// BookModel: models.NewBooksModel(sqlx.NewMysql(c.Mysql.DataSource), c.DBCache),
	}
}
