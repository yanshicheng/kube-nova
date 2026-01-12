package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/incremental"
)

const (

	// DefaultProjectID 默认项目 ID（未指定项目时使用）
	// 所有未指定项目的 namespace 都会绑定到这个默认项目
	DefaultProjectID uint64 = 3

	// SystemOperator 系统操作者标识
	SystemOperator = "system-incremental-sync"
)

// systemNamespaces 系统命名空间列表（这些命名空间不会被同步到平台）
var systemNamespaces = map[string]bool{
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
	"default":         true,
}

// HandleNamespaceEvent 处理 Namespace 事件
func (h *DefaultEventHandler) HandleNamespaceEvent(ctx context.Context, event *incremental.ResourceEvent) error {
	logger := logx.WithContext(ctx)

	switch event.Type {
	case incremental.EventAdd:
		return h.handleNamespaceAdd(ctx, event, logger)
	case incremental.EventUpdate:
		return h.handleNamespaceUpdate(ctx, event, logger)
	case incremental.EventDelete:
		return h.handleNamespaceDelete(ctx, event, logger)
	default:
		logger.Errorf("[NamespaceHandler] 未知事件类型: %s", event.Type)
		return nil
	}
}

// ==================== ADD 事件处理 ====================

// handleNamespaceAdd 处理 Namespace 创建事件
func (h *DefaultEventHandler) handleNamespaceAdd(ctx context.Context, event *incremental.ResourceEvent, logger logx.Logger) error {
	ns, ok := event.NewObject.(*corev1.Namespace)
	if !ok {
		return fmt.Errorf("无法转换为 Namespace 对象")
	}

	clusterUUID := event.ClusterUUID
	namespaceName := ns.Name

	logger.Infof("[NamespaceHandler] 处理 ADD 事件: Cluster=%s, Namespace=%s", clusterUUID, namespaceName)

	if h.isSystemNamespace(namespaceName) {
		logger.Debugf("[NamespaceHandler] 跳过系统命名空间: %s", namespaceName)
		return nil
	}

	if ns.Annotations != nil {
		if managedBy, ok := ns.Annotations[utils.AnnotationManagedBy]; ok && managedBy == utils.ManagedByPlatform {
			logger.Debugf("[NamespaceHandler] 平台创建的命名空间，跳过: %s", namespaceName)
			return nil
		}
	}

	existingWorkspaces, err := h.svcCtx.ProjectWorkspaceModel.FindAllByClusterUuidNamespaceIncludeDeleted(ctx, clusterUUID, namespaceName)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("查询工作空间失败: %v", err)
	}

	for _, ws := range existingWorkspaces {
		if ws.IsDeleted == 0 {
			logger.Debugf("[NamespaceHandler] 工作空间已存在，跳过: Cluster=%s, Namespace=%s, WorkspaceID=%d",
				clusterUUID, namespaceName, ws.Id)
			return nil
		}
	}

	projectCluster, err := h.ensureProjectClusterBinding(ctx, clusterUUID, ns, logger)
	if err != nil {
		return fmt.Errorf("确保项目集群绑定失败: %v", err)
	}

	for _, ws := range existingWorkspaces {
		if ws.IsDeleted == 1 && ws.ProjectClusterId == projectCluster.Id {
			logger.Infof("[NamespaceHandler] 恢复软删除的工作空间: ID=%d, Namespace=%s", ws.Id, namespaceName)
			if err := h.svcCtx.ProjectWorkspaceModel.RestoreAndUpdateStatus(ctx, ws.Id, 1, SystemOperator); err != nil {
				return fmt.Errorf("恢复工作空间失败: %v", err)
			}
			return nil
		}
	}

	workspace := h.buildWorkspaceFromNamespace(ns, projectCluster)
	_, err = h.svcCtx.ProjectWorkspaceModel.Insert(ctx, workspace)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "1062") {
			logger.Infof("[NamespaceHandler] 工作空间已被其他进程创建，跳过: Cluster=%s, Namespace=%s",
				clusterUUID, namespaceName)
			return nil
		}
		return fmt.Errorf("创建工作空间失败: %v", err)
	}

	logger.Infof("[NamespaceHandler] 创建工作空间成功: Cluster=%s, Namespace=%s, ProjectClusterID=%d",
		clusterUUID, namespaceName, projectCluster.Id)

	return nil
}

// handleNamespaceUpdate 处理 Namespace 更新事件
func (h *DefaultEventHandler) handleNamespaceUpdate(ctx context.Context, event *incremental.ResourceEvent, logger logx.Logger) error {
	ns, ok := event.NewObject.(*corev1.Namespace)
	if !ok {
		return fmt.Errorf("无法转换为 Namespace 对象")
	}

	clusterUUID := event.ClusterUUID
	namespaceName := ns.Name

	if h.isSystemNamespace(namespaceName) {
		return nil
	}

	if ns.DeletionTimestamp != nil {
		logger.Debugf("[NamespaceHandler] Namespace 正在删除中，跳过更新: %s", namespaceName)
		return nil
	}

	existingWorkspaces, err := h.svcCtx.ProjectWorkspaceModel.FindAllByClusterUuidNamespaceIncludeDeleted(ctx, clusterUUID, namespaceName)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("查询工作空间失败: %v", err)
	}

	if len(existingWorkspaces) == 0 {
		logger.Infof("[NamespaceHandler] UPDATE 事件但工作空间不存在，当作 ADD 处理: Cluster=%s, Namespace=%s",
			clusterUUID, namespaceName)
		return h.handleNamespaceAdd(ctx, event, logger)
	}

	for _, ws := range existingWorkspaces {
		if ws.IsDeleted == 0 {
			logger.Debugf("[NamespaceHandler] 工作空间存在，UPDATE 事件处理完成: ID=%d, Namespace=%s",
				ws.Id, namespaceName)
			return nil
		}
	}

	logger.Infof("[NamespaceHandler] 所有工作空间已删除，当作 ADD 处理: Cluster=%s, Namespace=%s",
		clusterUUID, namespaceName)
	return h.handleNamespaceAdd(ctx, event, logger)
}

// handleNamespaceDelete 处理 Namespace 删除事件
func (h *DefaultEventHandler) handleNamespaceDelete(ctx context.Context, event *incremental.ResourceEvent, logger logx.Logger) error {
	var namespaceName string
	if event.OldObject != nil {
		if ns, ok := event.OldObject.(*corev1.Namespace); ok {
			namespaceName = ns.Name
		}
	}
	if namespaceName == "" {
		namespaceName = event.Name
	}

	clusterUUID := event.ClusterUUID

	logger.Infof("[NamespaceHandler] 处理 DELETE 事件: Cluster=%s, Namespace=%s", clusterUUID, namespaceName)

	if h.isSystemNamespace(namespaceName) {
		return nil
	}
	existingWorkspaces, err := h.svcCtx.ProjectWorkspaceModel.FindAllByClusterUuidNamespaceIncludeDeleted(ctx, clusterUUID, namespaceName)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			logger.Debugf("[NamespaceHandler] 工作空间不存在，跳过删除: Cluster=%s, Namespace=%s",
				clusterUUID, namespaceName)
			return nil
		}
		return fmt.Errorf("查询工作空间失败: %v", err)
	}

	if len(existingWorkspaces) == 0 {
		logger.Debugf("[NamespaceHandler] 工作空间不存在，跳过删除: Cluster=%s, Namespace=%s",
			clusterUUID, namespaceName)
		return nil
	}

	deletedCount := 0
	for _, ws := range existingWorkspaces {
		if ws.IsDeleted == 0 {
			if err := h.svcCtx.ProjectWorkspaceModel.DeleteSoft(ctx, ws.Id); err != nil {
				logger.Errorf("[NamespaceHandler] 软删除工作空间失败: ID=%d, err=%v", ws.Id, err)
				continue
			}
			deletedCount++
			logger.Infof("[NamespaceHandler] 软删除工作空间成功: ID=%d, Namespace=%s", ws.Id, namespaceName)
		}
	}

	if deletedCount == 0 {
		logger.Debugf("[NamespaceHandler] 没有需要删除的工作空间: Cluster=%s, Namespace=%s",
			clusterUUID, namespaceName)
	}

	return nil
}

// ensureProjectClusterBinding 确保项目和项目集群绑定存在
func (h *DefaultEventHandler) ensureProjectClusterBinding(ctx context.Context, clusterUUID string, ns *corev1.Namespace, logger logx.Logger) (*model.OnecProjectCluster, error) {
	var projectID uint64

	// Step 1: 检查注解中是否指定了项目 UUID
	projectUUID := ""
	if ns.Annotations != nil {
		projectUUID = ns.Annotations[utils.AnnotationProjectUuid]
	}

	if projectUUID != "" {
		// 有注解，尝试查找或创建项目
		project, err := h.ensureProject(ctx, projectUUID, logger)
		if err != nil {
			return nil, fmt.Errorf("确保项目存在失败: %v", err)
		}
		projectID = project.Id
		logger.Debugf("[NamespaceHandler] 使用注解指定的项目: UUID=%s, ID=%d", projectUUID, projectID)
	} else {
		// 无注解，使用默认项目
		projectID = DefaultProjectID
		logger.Debugf("[NamespaceHandler] Namespace 无项目注解，使用默认项目 ID=%d", projectID)

		// 验证默认项目是否存在
		_, err := h.svcCtx.ProjectModel.FindOne(ctx, projectID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return nil, fmt.Errorf("默认项目(ID=%d)不存在，请先创建", projectID)
			}
			return nil, fmt.Errorf("查询默认项目失败: %v", err)
		}
	}

	projectCluster, err := h.ensureProjectCluster(ctx, clusterUUID, projectID, logger)
	if err != nil {
		return nil, fmt.Errorf("确保项目集群绑定失败: %v", err)
	}

	return projectCluster, nil
}

// ensureProject 确保项目存在
func (h *DefaultEventHandler) ensureProject(ctx context.Context, projectUUID string, logger logx.Logger) (*model.OnecProject, error) {
	project, err := h.svcCtx.ProjectModel.FindOneByUuidIncludeDeleted(ctx, projectUUID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			// 项目不存在，创建新项目
			logger.Infof("[NamespaceHandler] 项目不存在，创建新项目: UUID=%s", projectUUID)
			newProject, createErr := h.svcCtx.ProjectModel.CreateWithUuid(ctx, projectUUID, projectUUID, SystemOperator)
			if createErr != nil {
				return nil, fmt.Errorf("创建项目失败: %v", createErr)
			}
			return newProject, nil
		}
		return nil, fmt.Errorf("查询项目失败: %v", err)
	}

	if project.IsDeleted == 1 {
		logger.Infof("[NamespaceHandler] 恢复软删除的项目: ID=%d, UUID=%s", project.Id, projectUUID)
		if err := h.svcCtx.ProjectModel.RestoreSoftDeleted(ctx, project.Id, SystemOperator); err != nil {
			return nil, fmt.Errorf("恢复项目失败: %v", err)
		}
		// 重新查询以获取最新状态
		project, err = h.svcCtx.ProjectModel.FindOneByUuid(ctx, projectUUID)
		if err != nil {
			return nil, fmt.Errorf("查询恢复后的项目失败: %v", err)
		}
	}

	return project, nil
}

// ensureProjectCluster 确保项目集群绑定存在
func (h *DefaultEventHandler) ensureProjectCluster(ctx context.Context, clusterUUID string, projectID uint64, logger logx.Logger) (*model.OnecProjectCluster, error) {
	projectCluster, err := h.svcCtx.ProjectClusterModel.FindOneByClusterUuidProjectIdIncludeDeleted(ctx, clusterUUID, projectID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			logger.Infof("[NamespaceHandler] 项目集群绑定不存在，创建: ClusterUUID=%s, ProjectID=%d",
				clusterUUID, projectID)
			return h.createProjectCluster(ctx, clusterUUID, projectID)
		}
		return nil, fmt.Errorf("查询项目集群绑定失败: %v", err)
	}

	if projectCluster.IsDeleted == 1 {
		logger.Infof("[NamespaceHandler] 恢复软删除的项目集群绑定: ID=%d", projectCluster.Id)
		if err := h.svcCtx.ProjectClusterModel.RestoreSoftDeleted(ctx, projectCluster.Id, SystemOperator); err != nil {
			return nil, fmt.Errorf("恢复项目集群绑定失败: %v", err)
		}
		projectCluster, err = h.svcCtx.ProjectClusterModel.FindOneByClusterUuidProjectId(ctx, clusterUUID, projectID)
		if err != nil {
			return nil, fmt.Errorf("查询恢复后的项目集群绑定失败: %v", err)
		}
	}

	return projectCluster, nil
}

// createProjectCluster 创建项目集群绑定
func (h *DefaultEventHandler) createProjectCluster(ctx context.Context, clusterUUID string, projectID uint64) (*model.OnecProjectCluster, error) {
	projectCluster := &model.OnecProjectCluster{
		ClusterUuid:               clusterUUID,
		ProjectId:                 projectID,
		CpuLimit:                  "0",
		CpuOvercommitRatio:        1.0,
		CpuCapacity:               "0",
		CpuAllocated:              "0",
		MemLimit:                  "0Gi",
		MemOvercommitRatio:        1.0,
		MemCapacity:               "0Gi",
		MemAllocated:              "0Gi",
		StorageLimit:              "0Gi",
		StorageAllocated:          "0Gi",
		GpuLimit:                  "0",
		GpuOvercommitRatio:        1.0,
		GpuCapacity:               "0",
		GpuAllocated:              "0",
		PodsLimit:                 0,
		PodsAllocated:             0,
		ConfigmapLimit:            0,
		ConfigmapAllocated:        0,
		SecretLimit:               0,
		SecretAllocated:           0,
		PvcLimit:                  0,
		PvcAllocated:              0,
		EphemeralStorageLimit:     "0Gi",
		EphemeralStorageAllocated: "0Gi",
		ServiceLimit:              0,
		ServiceAllocated:          0,
		LoadbalancersLimit:        0,
		LoadbalancersAllocated:    0,
		NodeportsLimit:            0,
		NodeportsAllocated:        0,
		DeploymentsLimit:          0,
		DeploymentsAllocated:      0,
		JobsLimit:                 0,
		JobsAllocated:             0,
		CronjobsLimit:             0,
		CronjobsAllocated:         0,
		DaemonsetsLimit:           0,
		DaemonsetsAllocated:       0,
		StatefulsetsLimit:         0,
		StatefulsetsAllocated:     0,
		IngressesLimit:            0,
		IngressesAllocated:        0,
		CreatedBy:                 SystemOperator,
		UpdatedBy:                 SystemOperator,
		IsDeleted:                 0,
	}

	result, err := h.svcCtx.ProjectClusterModel.Insert(ctx, projectCluster)
	if err != nil {
		// 处理并发创建导致的重复键错误
		if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "1062") {
			// 重新查询返回已存在的记录
			return h.svcCtx.ProjectClusterModel.FindOneByClusterUuidProjectId(ctx, clusterUUID, projectID)
		}
		return nil, fmt.Errorf("插入项目集群绑定失败: %v", err)
	}

	insertID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("获取插入ID失败: %v", err)
	}
	projectCluster.Id = uint64(insertID)

	return projectCluster, nil
}

// buildWorkspaceFromNamespace 根据 Namespace 构建工作空间对象
func (h *DefaultEventHandler) buildWorkspaceFromNamespace(ns *corev1.Namespace, projectCluster *model.OnecProjectCluster) *model.OnecProjectWorkspace {
	// 从注解获取显示名称，如果没有则使用 namespace 名称
	name := ns.Name
	if ns.Annotations != nil {
		if displayName, ok := ns.Annotations[utils.AnnotationDisplayName]; ok && displayName != "" {
			name = displayName
		}
	}

	// 构建描述信息
	description := fmt.Sprintf("从 K8s Namespace 自动同步: %s", ns.Name)
	if ns.Annotations != nil {
		if desc, ok := ns.Annotations["description"]; ok && desc != "" {
			description = desc
		}
	}

	return &model.OnecProjectWorkspace{
		ProjectClusterId: projectCluster.Id,
		ClusterUuid:      projectCluster.ClusterUuid,
		Name:             name,
		Namespace:        ns.Name,
		Description:      description,

		CpuAllocated:              "0",
		MemAllocated:              "0Gi",
		StorageAllocated:          "0Gi",
		GpuAllocated:              "0",
		PodsAllocated:             0,
		ConfigmapAllocated:        0,
		SecretAllocated:           0,
		PvcAllocated:              0,
		EphemeralStorageAllocated: "0Gi",
		ServiceAllocated:          0,
		LoadbalancersAllocated:    0,
		NodeportsAllocated:        0,
		DeploymentsAllocated:      0,
		JobsAllocated:             0,
		CronjobsAllocated:         0,
		DaemonsetsAllocated:       0,
		StatefulsetsAllocated:     0,
		IngressesAllocated:        0,

		PodMaxCpu:                               "0",
		PodMaxMemory:                            "0Gi",
		PodMaxEphemeralStorage:                  "0Gi",
		PodMinCpu:                               "0",
		PodMinMemory:                            "0Mi",
		PodMinEphemeralStorage:                  "0Mi",
		ContainerMaxCpu:                         "0",
		ContainerMaxMemory:                      "0Gi",
		ContainerMaxEphemeralStorage:            "0Gi",
		ContainerMinCpu:                         "0",
		ContainerMinMemory:                      "0Mi",
		ContainerMinEphemeralStorage:            "0Mi",
		ContainerDefaultCpu:                     "100m",
		ContainerDefaultMemory:                  "128Mi",
		ContainerDefaultEphemeralStorage:        "0Gi",
		ContainerDefaultRequestCpu:              "50m",
		ContainerDefaultRequestMemory:           "64Mi",
		ContainerDefaultRequestEphemeralStorage: "0Mi",

		IsSystem:      0,
		Status:        1, // 1: 正常
		AppCreateTime: time.Now(),
		CreatedBy:     SystemOperator,
		UpdatedBy:     SystemOperator,
		IsDeleted:     0,
	}
}

// isSystemNamespace 判断是否是系统命名空间
func (h *DefaultEventHandler) isSystemNamespace(name string) bool {
	return systemNamespaces[name]
}
