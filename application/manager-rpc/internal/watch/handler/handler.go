package handler

import (
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/incremental"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
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

// 确保 DefaultEventHandler 实现了 incremental.EventHandler 接口
var _ incremental.EventHandler = (*DefaultEventHandler)(nil)
