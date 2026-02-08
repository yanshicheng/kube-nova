package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/incremental"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	"github.com/zeromicro/go-zero/core/logx"
)

// SvcCtx 事件处理器服务上下文
type SvcCtx struct {
	ClusterModel         model.OnecClusterModel
	ClusterResourceModel model.OnecClusterResourceModel

	ProjectModel          model.OnecProjectModel
	ProjectClusterModel   model.OnecProjectClusterModel
	ProjectWorkspaceModel model.OnecProjectWorkspaceModel
	ProjectApplication    model.OnecProjectApplicationModel
	ProjectVersion        model.OnecProjectVersionModel
	ProjectAuditLog       model.OnecProjectAuditLogModel

	K8sManager cluster.Manager
}

// DefaultEventHandler 默认事件处理实现
type DefaultEventHandler struct {
	svcCtx *SvcCtx
}

// NewDefaultEventHandler 创建默认事件处理器
func NewDefaultEventHandler(svcCtx *SvcCtx) *DefaultEventHandler {
	return &DefaultEventHandler{
		svcCtx: svcCtx,
	}
}

// GetSvcCtx 获取服务上下文（供子处理器使用）
func (h *DefaultEventHandler) GetSvcCtx() *SvcCtx {
	return h.svcCtx
}

// ClusterModel 获取集群模型
func (h *DefaultEventHandler) ClusterModel() model.OnecClusterModel {
	return h.svcCtx.ClusterModel
}

// ClusterResourceModel 获取集群资源模型
func (h *DefaultEventHandler) ClusterResourceModel() model.OnecClusterResourceModel {
	return h.svcCtx.ClusterResourceModel
}

// ProjectModel 获取项目模型
func (h *DefaultEventHandler) ProjectModel() model.OnecProjectModel {
	return h.svcCtx.ProjectModel
}

// ProjectClusterModel 获取项目集群模型
func (h *DefaultEventHandler) ProjectClusterModel() model.OnecProjectClusterModel {
	return h.svcCtx.ProjectClusterModel
}

// WorkspaceModel 获取工作空间模型
func (h *DefaultEventHandler) WorkspaceModel() model.OnecProjectWorkspaceModel {
	return h.svcCtx.ProjectWorkspaceModel
}

// ApplicationModel 获取应用模型
func (h *DefaultEventHandler) ApplicationModel() model.OnecProjectApplicationModel {
	return h.svcCtx.ProjectApplication
}

// VersionModel 获取版本模型
func (h *DefaultEventHandler) VersionModel() model.OnecProjectVersionModel {
	return h.svcCtx.ProjectVersion
}

// AuditLogModel 获取审计日志模型
func (h *DefaultEventHandler) AuditLogModel() model.OnecProjectAuditLogModel {
	return h.svcCtx.ProjectAuditLog
}

// K8sManager 获取 K8s 管理器
func (h *DefaultEventHandler) K8sManager() cluster.Manager {
	return h.svcCtx.K8sManager
}

// AuditLogInfo 审计日志信息结构体
type AuditLogInfo struct {
	ClusterName     string
	ClusterUuid     string
	ProjectId       uint64
	ProjectName     string
	WorkspaceId     uint64
	WorkspaceName   string
	ApplicationId   uint64
	ApplicationName string
	Title           string
	ActionDetail    string
	Status          int64 // 1: 成功, 0: 失败
}

// createAuditLog 创建审计日志
func (h *DefaultEventHandler) createAuditLog(ctx context.Context, info *AuditLogInfo) {
	logger := logx.WithContext(ctx)

	auditLog := &model.OnecProjectAuditLog{
		ClusterName:     info.ClusterName,
		ClusterUuid:     info.ClusterUuid,
		ProjectId:       info.ProjectId,
		ProjectName:     info.ProjectName,
		WorkspaceId:     info.WorkspaceId,
		WorkspaceName:   info.WorkspaceName,
		ApplicationId:   info.ApplicationId,
		ApplicationName: info.ApplicationName,
		Title:           info.Title,
		ActionDetail:    info.ActionDetail,
		Status:          info.Status,
		OperatorId:      0,
		OperatorName:    SystemOperator,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		IsDeleted:       0,
	}

	_, err := h.svcCtx.ProjectAuditLog.Insert(ctx, auditLog)
	if err != nil {
		logger.Errorf("[AuditLog] 创建审计日志失败: %v, info: %+v", err, info)
	}
}

// getClusterName 获取集群名称
func (h *DefaultEventHandler) getClusterName(ctx context.Context, clusterUuid string) string {
	clusterInfo, err := h.svcCtx.ClusterModel.FindOneByUuid(ctx, clusterUuid)
	if err != nil {
		return clusterUuid
	}
	return clusterInfo.Name
}

// getProjectInfo 获取项目信息
func (h *DefaultEventHandler) getProjectInfo(ctx context.Context, projectClusterId uint64) (uint64, string) {
	projectCluster, err := h.svcCtx.ProjectClusterModel.FindOne(ctx, projectClusterId)
	if err != nil {
		return 0, ""
	}
	project, err := h.svcCtx.ProjectModel.FindOne(ctx, projectCluster.ProjectId)
	if err != nil {
		return projectCluster.ProjectId, ""
	}
	return project.Id, project.Name
}

// cascadeDeleteWorkspace 级联硬删除工作空间及其下属资源
// 删除顺序：Version -> Application -> Workspace
func (h *DefaultEventHandler) cascadeDeleteWorkspace(ctx context.Context, workspace *model.OnecProjectWorkspace) error {
	logger := logx.WithContext(ctx)
	workspaceId := workspace.Id

	// Step 1: 查询该工作空间下的所有应用
	apps, err := h.svcCtx.ProjectApplication.FindAllByWorkspaceId(ctx, workspaceId)
	if err != nil && err != model.ErrNotFound {
		return fmt.Errorf("查询应用列表失败: %v", err)
	}

	// Step 2: 遍历每个应用，先删除其下的所有版本，再删除应用
	for _, app := range apps {
		// 查询该应用下的所有版本
		versions, err := h.svcCtx.ProjectVersion.FindAllByApplicationId(ctx, app.Id)
		if err != nil && err != model.ErrNotFound {
			logger.Errorf("[CascadeDelete] 查询版本列表失败: AppID=%d, err=%v", app.Id, err)
			continue
		}

		// 硬删除所有版本
		for _, version := range versions {
			if err := h.svcCtx.ProjectVersion.Delete(ctx, version.Id); err != nil {
				logger.Errorf("[CascadeDelete] 硬删除版本失败: VersionID=%d, err=%v", version.Id, err)
				continue
			}
			logger.Infof("[CascadeDelete] 硬删除版本成功: VersionID=%d, ResourceName=%s", version.Id, version.ResourceName)
		}

		// 硬删除应用
		if err := h.svcCtx.ProjectApplication.Delete(ctx, app.Id); err != nil {
			logger.Errorf("[CascadeDelete] 硬删除应用失败: AppID=%d, err=%v", app.Id, err)
			continue
		}
		logger.Infof("[CascadeDelete] 硬删除应用成功: AppID=%d, AppName=%s", app.Id, app.NameEn)
	}

	// Step 3: 硬删除工作空间
	if err := h.svcCtx.ProjectWorkspaceModel.Delete(ctx, workspaceId); err != nil {
		return fmt.Errorf("硬删除工作空间失败: %v", err)
	}
	logger.Infof("[CascadeDelete] 硬删除工作空间成功: WorkspaceID=%d, Namespace=%s", workspaceId, workspace.Namespace)

	return nil
}

// 确保 DefaultEventHandler 实现了 incremental.EventHandler 接口
var _ incremental.EventHandler = (*DefaultEventHandler)(nil)
