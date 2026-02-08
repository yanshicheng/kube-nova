package svc

import (
	"log"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/authz"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/config"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/message"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/notification"
	notification2 "github.com/yanshicheng/kube-nova/application/portal-rpc/internal/notification"
	"github.com/yanshicheng/kube-nova/pkg/storage"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config  config.Config    `json:"Config"`
	Cache   *redis.Redis     `json:"Cache,omitempty"`
	Storage storage.Uploader `json:"Storage,omitempty"`

	// 数据模型
	SysUser                      model.SysUserModel                 `json:"SysUser,omitempty"`
	SysUserRole                  model.SysUserRoleModel             `json:"SysUserRole,omitempty"`
	SysUserDept                  model.SysUserDeptModel             `json:"SysUserDept,omitempty"`
	SysRole                      model.SysRoleModel                 `json:"SysRole,omitempty"`
	SysApi                       model.SysApiModel                  `json:"SysApi,omitempty"`
	SysRoleApi                   model.SysRoleApiModel              `json:"SysRoleApi,omitempty"`
	SysToken                     model.SysTokenModel                `json:"SysToken,omitempty"`
	SysMenu                      model.SysMenuModel                 `json:"SysMenu,omitempty"`
	SysMenuModel                 model.SysMenuModel                 `json:"SysMenuModel,omitempty"`
	SysRoleMenu                  model.SysRoleMenuModel             `json:"SysRoleMenu,omitempty"`
	SysLoginLog                  model.SysLoginLogModel             `json:"SysLoginLog,omitempty"`
	SysDept                      model.SysDeptModel                 `json:"SysDept,omitempty"`
	SysPlatformModel             model.SysPlatformModel             `json:"SysPlatformModel,omitempty"`
	SysUserPlatformModel         model.SysUserPlatformModel         `json:"SysUserPlatformModel,omitempty"`
	SiteMessagesModel            model.SiteMessagesModel            `json:"SiteMessagesModel,omitempty"`
	AlertChannelsModel           model.AlertChannelsModel           `json:"Model.AlertChannelsModel,omitempty"`
	AlertGroupsModel             model.AlertGroupsModel             `json:"Model.AlertGroupsModel,omitempty"`
	AlertNotificationsModel      model.AlertNotificationsModel      `json:"Model.AlertNotificationsModel,omitempty"`
	AlertGroupLevelChannelsModel model.AlertGroupLevelChannelsModel `json:"Model.AlertGroupLevelChannelsModel,omitempty"`
	AlertGroupMembersModel       model.AlertGroupMembersModel       `json:"Model.AlertGroupMembersModel,omitempty"`
	AlertGroupAppsModel          model.AlertGroupAppsModel

	// Casbin RBAC 管理器（K8s 分布式部署）
	AuthzManager *authz.CasbinRBACManager `json:"AuthzManager,omitempty"`

	// 告警通知管理器
	AlertManager notification2.Manager `json:"AlertManager,omitempty"`

	// 消息推送器
	MessagePusher *message.MessagePusher `json:"MessagePusher,omitempty"`

	// 告警聚合器服务（支持 K8s Leader Election 和 Redis 降级）
	AggregatorService *notification2.AggregatorService `json:"AggregatorService,omitempty"`
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

	// 初始化存储
	uploader, err := storage.NewUploader(storage.UploaderOptions{
		AccessKey:    c.StorageConf.AccessKey,
		AccessSecret: c.StorageConf.AccessSecret,
		CAFile:       c.StorageConf.CAFile,
		CAKey:        c.StorageConf.CAKey,
		Endpoints:    c.StorageConf.Endpoints,
		Provider:     c.StorageConf.Provider,
		UseTLS:       c.StorageConf.UseTLS,
		BucketName:   c.StorageConf.BucketName,
	})
	if err != nil {
		log.Fatalf("初始化上传器失败: %v", err)
	}

	// 初始化 Redis
	rdb := redis.MustNewRedis(c.Cache)

	// 初始化数据模型
	sysUser := model.NewSysUserModel(sqlConn, c.DBCache)
	sysRole := model.NewSysRoleModel(sqlConn, c.DBCache)
	sysRoleMenu := model.NewSysRoleMenuModel(sqlConn, c.DBCache)
	sysRoleApi := model.NewSysRoleApiModel(sqlConn, c.DBCache)
	sysUserRole := model.NewSysUserRoleModel(sqlConn, c.DBCache)
	sysUserDept := model.NewSysUserDeptModel(sqlConn, c.DBCache)
	sysApi := model.NewSysApiModel(sqlConn, c.DBCache)
	sysMenu := model.NewSysMenuModel(sqlConn, c.DBCache)
	sysLoginLog := model.NewSysLoginLogModel(sqlConn, c.DBCache)
	sysToken := model.NewSysTokenModel(sqlConn, c.DBCache)
	sysDept := model.NewSysDeptModel(sqlConn, c.DBCache)
	sysPlatform := model.NewSysPlatformModel(sqlConn, c.DBCache)
	sysUserPlatform := model.NewSysUserPlatformModel(sqlConn, c.DBCache)
	siteMessagesModel := model.NewSiteMessagesModel(sqlConn, c.DBCache)
	alertChannelsModel := model.NewAlertChannelsModel(sqlConn, c.DBCache)
	alertGroupsModel := model.NewAlertGroupsModel(sqlConn, c.DBCache)
	alertNotificationsModel := model.NewAlertNotificationsModel(sqlConn, c.DBCache)
	alertGroupLevelChannelsModel := model.NewAlertGroupLevelChannelsModel(sqlConn, c.DBCache)
	alertGroupMembersModel := model.NewAlertGroupMembersModel(sqlConn, c.DBCache)
	alertGroupAppsModel := model.NewAlertGroupAppsModel(sqlConn, c.DBCache)

	rbacManager, err := authz.NewCasbinRBACManager(
		c.Cache,
		sysRole,
		sysApi,
		sysRoleApi,
	)
	if err != nil {
		log.Fatalf("初始化 RBAC 管理器失败: %v", err)
	}

	// 初始化消息推送器
	messagePusher := message.NewMessagePusher(rdb)

	// 创建告警聚合器配置
	// 将配置文件中的聚合器配置转换为通知系统的配置格式
	aggregatorConfig := notification.AggregatorConfig{
		Enabled:            c.Aggregator.Enabled,
		MaxBufferSize:      c.Aggregator.MaxBufferSize,
		GlobalBufferWindow: c.Aggregator.GlobalBufferWindow,
		SeverityWindows: notification.SeverityWindowConfig{
			Critical: c.Aggregator.SeverityWindows.Critical,
			Warning:  c.Aggregator.SeverityWindows.Warning,
			Info:     c.Aggregator.SeverityWindows.Info,
			Default:  c.Aggregator.SeverityWindows.Default,
		},
	}

	// 如果配置文件中没有设置，使用默认值
	if !c.Aggregator.Enabled {
		aggregatorConfig = notification.DefaultAggregatorConfig()
	}

	// 初始化告警管理器（传入聚合器配置）
	alertManager := notification.NewManager(notification.ManagerConfig{
		PortalName:                   c.PortalName,
		PortalUrl:                    c.PortalUrl,
		SysUserModel:                 sysUser,
		AlertChannelsModel:           alertChannelsModel,
		AlertGroupsModel:             alertGroupsModel,
		AlertGroupMembersModel:       alertGroupMembersModel,
		AlertGroupLevelChannelsModel: alertGroupLevelChannelsModel,
		AlertNotificationsModel:      alertNotificationsModel,
		AlertGroupAppsModel:          alertGroupAppsModel,
		SiteMessagesModel:            siteMessagesModel,
		Redis:                        rdb,
		AggregatorConfig:             &aggregatorConfig, // 传入聚合器配置
	})

	// 创建告警聚合器服务（支持 K8s Leader Election 和 Redis 降级）
	// 注意：聚合器配置已经在 alertManager 中设置，这里只需要管理 Leader Election
	aggregatorService := notification.NewAggregatorService(notification.AggregatorServiceConfig{
		Redis:   rdb,
		Manager: alertManager,
		Config:  aggregatorConfig,
		LeaderConfig: notification.LeaderElectionConfig{
			Enabled:        c.LeaderElection.Enabled,
			LeaseName:      c.LeaderElection.LeaseName,
			LeaseNamespace: c.LeaderElection.LeaseNamespace,
			LeaseDuration:  c.LeaderElection.LeaseDuration,
			RenewDeadline:  c.LeaderElection.RenewDeadline,
			RetryPeriod:    c.LeaderElection.RetryPeriod,
		},
	})

	// 启动聚合器服务（会自动选择 K8s Leader Election 或 Redis 模式）
	if err := aggregatorService.Start(); err != nil {
		log.Fatalf("启动告警聚合器服务失败: %v", err)
	}

	return &ServiceContext{
		Config:                       c,
		Cache:                        rdb,
		Storage:                      uploader,
		SysUser:                      sysUser,
		SysRole:                      sysRole,
		SysRoleMenu:                  sysRoleMenu,
		SysRoleApi:                   sysRoleApi,
		SysUserRole:                  sysUserRole,
		SysUserDept:                  sysUserDept,
		SysApi:                       sysApi,
		SysMenu:                      sysMenu,
		SysMenuModel:                 sysMenu,
		SysLoginLog:                  sysLoginLog,
		SysToken:                     sysToken,
		SysDept:                      sysDept,
		SysPlatformModel:             sysPlatform,
		SysUserPlatformModel:         sysUserPlatform,
		AuthzManager:                 rbacManager,
		SiteMessagesModel:            siteMessagesModel,
		AlertChannelsModel:           alertChannelsModel,
		AlertGroupsModel:             alertGroupsModel,
		AlertNotificationsModel:      alertNotificationsModel,
		AlertGroupLevelChannelsModel: alertGroupLevelChannelsModel,
		AlertGroupMembersModel:       alertGroupMembersModel,
		AlertGroupAppsModel:          alertGroupAppsModel,
		AlertManager:                 alertManager,
		MessagePusher:                messagePusher,
		AggregatorService:            aggregatorService,
	}
}
