package watch

import (
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/handler"
)

// SetupIncrementalSync 设置增量同步
func SetupIncrementalSync(svcCtx *svc.ServiceContext) bool {
	// 创建事件处理器
	eventHandler := handler.NewDefaultEventHandler(&handler.SvcCtx{
		ClusterModel:         svcCtx.OnecClusterModel,
		ClusterResourceModel: svcCtx.OnecClusterResourceModel,

		ProjectModel:          svcCtx.OnecProjectModel,
		ProjectClusterModel:   svcCtx.OnecProjectClusterModel,
		ProjectWorkspaceModel: svcCtx.OnecProjectWorkspaceModel,
		ProjectApplication:    svcCtx.OnecProjectApplication,
		ProjectVersion:        svcCtx.OnecProjectVersion,
		ProjectAuditLog:       svcCtx.OnecProjectAuditLog,

		K8sManager: svcCtx.K8sManager,
	})

	// 根据配置选择模式
	if svcCtx.Config.LeaderElection.Enabled {
		// Leader Election 模式
		if svcCtx.LeaderSyncManager == nil {
			logx.Errorf("[IncrementalSync] LeaderSyncManager 未初始化")
			return false
		}
		svcCtx.LeaderSyncManager.SetHandler(eventHandler)
		logx.Info("[IncrementalSync] Handler 设置完成 (Leader Election 模式)")
	} else {
		// Redis 分布式锁模式
		if svcCtx.IncrementalSyncManager == nil {
			logx.Errorf("[IncrementalSync] IncrementalSyncManager 未初始化")
			return false
		}
		svcCtx.IncrementalSyncManager.SetHandler(eventHandler)
		logx.Info("[IncrementalSync] Handler 设置完成 (Redis 分布式锁模式)")
	}

	return true
}

// StartIncrementalSync 启动增量同步
func StartIncrementalSync(svcCtx *svc.ServiceContext) error {
	if svcCtx.Config.LeaderElection.Enabled {
		if svcCtx.LeaderSyncManager == nil {
			logx.Errorf("[IncrementalSync] LeaderSyncManager 未初始化，跳过启动")
			return nil
		}
		return svcCtx.LeaderSyncManager.Start()
	}

	if svcCtx.IncrementalSyncManager == nil {
		logx.Errorf("[IncrementalSync] IncrementalSyncManager 未初始化，跳过启动")
		return nil
	}
	return svcCtx.IncrementalSyncManager.Start()
}

// StopIncrementalSync 停止增量同步
func StopIncrementalSync(svcCtx *svc.ServiceContext) error {
	if svcCtx.Config.LeaderElection.Enabled {
		if svcCtx.LeaderSyncManager == nil {
			return nil
		}
		return svcCtx.LeaderSyncManager.Stop()
	}

	if svcCtx.IncrementalSyncManager == nil {
		return nil
	}
	return svcCtx.IncrementalSyncManager.Stop()
}
