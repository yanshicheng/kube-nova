package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/incremental"
)

const (
	// IkubeopsResourcePrefix ikubeops 资源前缀
	IkubeopsResourcePrefix = "ikubeops-"
)

// HandleResourceQuotaEvent 处理 ResourceQuota 事件
func (h *DefaultEventHandler) HandleResourceQuotaEvent(ctx context.Context, event *incremental.ResourceEvent) error {
	var rq *corev1.ResourceQuota

	switch event.Type {
	case incremental.EventAdd, incremental.EventUpdate:
		var ok bool
		rq, ok = event.NewObject.(*corev1.ResourceQuota)
		if !ok {
			return errors.New("invalid resourcequota object")
		}
	case incremental.EventDelete:
		var ok bool
		rq, ok = event.OldObject.(*corev1.ResourceQuota)
		if !ok {
			return errors.New("invalid resourcequota object")
		}
	}

	expectedName := IkubeopsResourcePrefix + rq.Namespace
	if rq.Name != expectedName {
		return nil
	}

	if event.Type == incremental.EventAdd {
		if rq.Annotations != nil {
			if managedBy, ok := rq.Annotations[utils.AnnotationManagedBy]; ok && managedBy == utils.ManagedByPlatform {
				logx.WithContext(ctx).Debugf("[ResourceQuota-ADD] 平台创建的资源，跳过: %s/%s", rq.Namespace, rq.Name)
				return nil
			}
		}
	}

	switch event.Type {
	case incremental.EventAdd:
		return h.handleResourceQuotaAddOrUpdate(ctx, event.ClusterUUID, rq)
	case incremental.EventUpdate:
		return h.handleResourceQuotaAddOrUpdate(ctx, event.ClusterUUID, rq)
	case incremental.EventDelete:
		return h.handleResourceQuotaDelete(ctx, event.ClusterUUID, rq)
	}

	return nil
}

// handleResourceQuotaAddOrUpdate 处理 ResourceQuota 创建或更新事件
func (h *DefaultEventHandler) handleResourceQuotaAddOrUpdate(ctx context.Context, clusterUUID string, rq *corev1.ResourceQuota) error {
	logger := logx.WithContext(ctx)
	logger.Infof("[ResourceQuota-SYNC] ClusterUUID: %s, Namespace: %s, Name: %s", clusterUUID, rq.Namespace, rq.Name)

	workspaces, err := h.svcCtx.ProjectWorkspaceModel.FindAllByClusterUuidNamespaceIncludeDeleted(ctx, clusterUUID, rq.Namespace)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("查询工作空间失败: %v", err)
	}

	var workspace *model.OnecProjectWorkspace
	for _, ws := range workspaces {
		if ws.IsDeleted == 0 {
			workspace = ws
			break
		}
	}

	if workspace == nil {
		logger.Debugf("[ResourceQuota-SYNC] 未找到对应的工作空间，跳过: ClusterUUID=%s, Namespace=%s", clusterUUID, rq.Namespace)
		return nil
	}

	h.updateWorkspaceFromResourceQuota(workspace, rq)

	if err := h.svcCtx.ProjectWorkspaceModel.Update(ctx, workspace); err != nil {
		return fmt.Errorf("更新工作空间失败: %v", err)
	}

	// workspace.ProjectClusterId 是 project_cluster 表的 ID，不是 project 表的 ID
	if err := h.svcCtx.ProjectModel.SyncProjectClusterResourceAllocation(ctx, workspace.ProjectClusterId); err != nil {
		logger.Errorf("[ResourceQuota-SYNC] 同步项目集群资源失败: %v", err)
	}

	logger.Infof("[ResourceQuota-SYNC] 更新工作空间成功: ID=%d, Namespace=%s", workspace.Id, workspace.Namespace)
	return nil
}

// handleResourceQuotaDelete 处理 ResourceQuota 删除事件
func (h *DefaultEventHandler) handleResourceQuotaDelete(ctx context.Context, clusterUUID string, rq *corev1.ResourceQuota) error {
	logger := logx.WithContext(ctx)
	logger.Infof("[ResourceQuota-DELETE] ClusterUUID: %s, Namespace: %s, Name: %s", clusterUUID, rq.Namespace, rq.Name)

	// 查询对应的工作空间
	workspaces, err := h.svcCtx.ProjectWorkspaceModel.FindAllByClusterUuidNamespaceIncludeDeleted(ctx, clusterUUID, rq.Namespace)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("查询工作空间失败: %v", err)
	}

	// 找到未删除的工作空间
	var workspace *model.OnecProjectWorkspace
	for _, ws := range workspaces {
		if ws.IsDeleted == 0 {
			workspace = ws
			break
		}
	}

	if workspace == nil {
		logger.Debugf("[ResourceQuota-DELETE] 未找到对应的工作空间，跳过: ClusterUUID=%s, Namespace=%s", clusterUUID, rq.Namespace)
		return nil
	}

	// 重置工作空间的 ResourceQuota 相关字段为默认值
	h.resetWorkspaceResourceQuotaFields(workspace)

	// 更新工作空间
	if err := h.svcCtx.ProjectWorkspaceModel.Update(ctx, workspace); err != nil {
		return fmt.Errorf("更新工作空间失败: %v", err)
	}

	// 同步项目集群资源分配
	if err := h.svcCtx.ProjectModel.SyncProjectClusterResourceAllocation(ctx, workspace.ProjectClusterId); err != nil {
		logger.Errorf("[ResourceQuota-DELETE] 同步项目集群资源失败: %v", err)
	}

	logger.Infof("[ResourceQuota-DELETE] 重置工作空间配额成功: ID=%d, Namespace=%s", workspace.Id, workspace.Namespace)
	return nil
}

// resetWorkspaceResourceQuotaFields 重置工作空间的 ResourceQuota 相关字段为默认值
func (h *DefaultEventHandler) resetWorkspaceResourceQuotaFields(workspace *model.OnecProjectWorkspace) {
	// 重置计算资源
	workspace.CpuAllocated = "0"
	workspace.MemAllocated = "0Gi"
	workspace.StorageAllocated = "0Gi"
	workspace.GpuAllocated = "0"
	workspace.EphemeralStorageAllocated = "0Gi"

	// 重置对象数量限制
	workspace.PodsAllocated = 0
	workspace.ConfigmapAllocated = 0
	workspace.SecretAllocated = 0
	workspace.PvcAllocated = 0
	workspace.ServiceAllocated = 0
	workspace.LoadbalancersAllocated = 0
	workspace.NodeportsAllocated = 0

	// 重置工作负载数量限制
	workspace.DeploymentsAllocated = 0
	workspace.JobsAllocated = 0
	workspace.CronjobsAllocated = 0
	workspace.DaemonsetsAllocated = 0
	workspace.StatefulsetsAllocated = 0
	workspace.IngressesAllocated = 0

	// 更新操作者
	workspace.UpdatedBy = SystemOperator
}

// updateWorkspaceFromResourceQuota 从 ResourceQuota 更新工作空间字段
func (h *DefaultEventHandler) updateWorkspaceFromResourceQuota(workspace *model.OnecProjectWorkspace, rq *corev1.ResourceQuota) {
	hard := rq.Spec.Hard

	// CPU
	if cpu, ok := hard[corev1.ResourceCPU]; ok {
		workspace.CpuAllocated = cpu.String()
	}

	// Memory
	if mem, ok := hard[corev1.ResourceMemory]; ok {
		workspace.MemAllocated = mem.String()
	}

	// Storage (requests.storage)
	if storage, ok := hard[corev1.ResourceRequestsStorage]; ok {
		workspace.StorageAllocated = storage.String()
	}

	// GPU (nvidia.com/gpu)
	if gpu, ok := hard["nvidia.com/gpu"]; ok {
		workspace.GpuAllocated = gpu.String()
	} else if gpu, ok := hard["requests.nvidia.com/gpu"]; ok {
		workspace.GpuAllocated = gpu.String()
	}

	// Ephemeral Storage
	if ephStorage, ok := hard[corev1.ResourceEphemeralStorage]; ok {
		workspace.EphemeralStorageAllocated = ephStorage.String()
	} else if ephStorage, ok := hard[corev1.ResourceRequestsEphemeralStorage]; ok {
		workspace.EphemeralStorageAllocated = ephStorage.String()
	}

	// Pods
	if pods, ok := hard[corev1.ResourcePods]; ok {
		workspace.PodsAllocated = pods.Value()
	}

	// ConfigMaps
	if configmaps, ok := hard[corev1.ResourceConfigMaps]; ok {
		workspace.ConfigmapAllocated = configmaps.Value()
	}

	// Secrets
	if secrets, ok := hard[corev1.ResourceSecrets]; ok {
		workspace.SecretAllocated = secrets.Value()
	}

	// PVCs
	if pvcs, ok := hard[corev1.ResourcePersistentVolumeClaims]; ok {
		workspace.PvcAllocated = pvcs.Value()
	}

	// Services
	if services, ok := hard[corev1.ResourceServices]; ok {
		workspace.ServiceAllocated = services.Value()
	}

	// LoadBalancers
	if lbs, ok := hard[corev1.ResourceServicesLoadBalancers]; ok {
		workspace.LoadbalancersAllocated = lbs.Value()
	}

	// NodePorts
	if nodeports, ok := hard[corev1.ResourceServicesNodePorts]; ok {
		workspace.NodeportsAllocated = nodeports.Value()
	}

	for resourceName, quantity := range hard {
		name := string(resourceName)
		switch {
		case strings.HasSuffix(name, "deployments.apps"):
			workspace.DeploymentsAllocated = quantity.Value()
		case strings.HasSuffix(name, "jobs.batch"):
			workspace.JobsAllocated = quantity.Value()
		case strings.HasSuffix(name, "cronjobs.batch"):
			workspace.CronjobsAllocated = quantity.Value()
		case strings.HasSuffix(name, "daemonsets.apps"):
			workspace.DaemonsetsAllocated = quantity.Value()
		case strings.HasSuffix(name, "statefulsets.apps"):
			workspace.StatefulsetsAllocated = quantity.Value()
		case strings.HasSuffix(name, "ingresses.networking.k8s.io"):
			workspace.IngressesAllocated = quantity.Value()
		}
	}

	// 更新操作者
	workspace.UpdatedBy = SystemOperator
}
