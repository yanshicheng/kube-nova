package watch

import (
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/handler"
)

// SetupIncrementalSync 设置增量同步
func SetupIncrementalSync(svcCtx *svc.ServiceContext) bool {
	if svcCtx.IncrementalSyncManager == nil {
		logx.Errorf("[IncrementalSync] 管理器未初始化，跳过设置")
		return false
	}

	eventHandler := handler.NewDefaultEventHandler(&handler.SvcCtx{
		ClusterModel:         svcCtx.OnecClusterModel,
		ClusterResourceModel: svcCtx.OnecClusterResourceModel,

		ProjectModel:          svcCtx.OnecProjectModel,
		ProjectClusterModel:   svcCtx.OnecProjectClusterModel,
		ProjectWorkspaceModel: svcCtx.OnecProjectWorkspaceModel,
		ProjectApplication:    svcCtx.OnecProjectApplication,
		ProjectVersion:        svcCtx.OnecProjectVersion,
		ProjectAuditLog:       svcCtx.OnecProjectAuditLog,
	})

	svcCtx.IncrementalSyncManager.SetHandler(eventHandler)

	logx.Info("[IncrementalSync] Handler 设置完成")
	return true
}

// StartIncrementalSync 启动增量同步
func StartIncrementalSync(svcCtx *svc.ServiceContext) error {
	if svcCtx.IncrementalSyncManager == nil {
		logx.Errorf("[IncrementalSync] 管理器未初始化，跳过启动")
		return nil
	}

	return svcCtx.IncrementalSyncManager.Start()
}

// StopIncrementalSync 停止增量同步
func StopIncrementalSync(svcCtx *svc.ServiceContext) error {
	if svcCtx.IncrementalSyncManager == nil {
		return nil
	}

	return svcCtx.IncrementalSyncManager.Stop()
}
