package handler

import (
	"context"
	"errors"
	"fmt"

	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/incremental"
)

// HandleLimitRangeEvent 处理 LimitRange 事件
func (h *DefaultEventHandler) HandleLimitRangeEvent(ctx context.Context, event *incremental.ResourceEvent) error {
	var lr *corev1.LimitRange

	switch event.Type {
	case incremental.EventAdd, incremental.EventUpdate:
		var ok bool
		lr, ok = event.NewObject.(*corev1.LimitRange)
		if !ok {
			return errors.New("invalid limitrange object")
		}
	case incremental.EventDelete:
		var ok bool
		lr, ok = event.OldObject.(*corev1.LimitRange)
		if !ok {
			return errors.New("invalid limitrange object")
		}
	}

	// 只处理 ikubeops-${namespace} 命名的 LimitRange
	expectedName := IkubeopsResourcePrefix + lr.Namespace
	if lr.Name != expectedName {
		return nil
	}

	if event.Type == incremental.EventAdd {
		if lr.Annotations != nil {
			if managedBy, ok := lr.Annotations[utils.AnnotationManagedBy]; ok && managedBy == utils.ManagedByPlatform {
				logx.WithContext(ctx).Debugf("[LimitRange-ADD] 平台创建的资源，跳过: %s/%s", lr.Namespace, lr.Name)
				return nil
			}
		}
	}

	switch event.Type {
	case incremental.EventAdd:
		return h.handleLimitRangeAddOrUpdate(ctx, event.ClusterUUID, lr)
	case incremental.EventUpdate:
		return h.handleLimitRangeAddOrUpdate(ctx, event.ClusterUUID, lr)
	case incremental.EventDelete:
		return h.handleLimitRangeDelete(ctx, event.ClusterUUID, lr)
	}

	return nil
}

// handleLimitRangeAddOrUpdate 处理 LimitRange 创建或更新事件
func (h *DefaultEventHandler) handleLimitRangeAddOrUpdate(ctx context.Context, clusterUUID string, lr *corev1.LimitRange) error {
	logger := logx.WithContext(ctx)
	logger.Debugf("[LimitRange-SYNC] ClusterUUID: %s, Namespace: %s, Name: %s", clusterUUID, lr.Namespace, lr.Name)

	// 1. 通过 cluster_uuid + namespace 查询工作空间
	workspaces, err := h.svcCtx.ProjectWorkspaceModel.FindAllByClusterUuidNamespaceIncludeDeleted(ctx, clusterUUID, lr.Namespace)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("查询工作空间失败: %v", err)
	}

	// 2. 找到未删除的工作空间
	var workspace *model.OnecProjectWorkspace
	for _, ws := range workspaces {
		if ws.IsDeleted == 0 {
			workspace = ws
			break
		}
	}

	if workspace == nil {
		logger.Debugf("[LimitRange-SYNC] 未找到对应的工作空间，跳过: ClusterUUID=%s, Namespace=%s", clusterUUID, lr.Namespace)
		return nil
	}

	// 3. 【修复】检查是否有实际变更
	changes := h.checkLimitRangeChanges(workspace, lr)
	if len(changes) == 0 {
		logger.Debugf("[LimitRange-SYNC] 无实际变更，跳过: ClusterUUID=%s, Namespace=%s", clusterUUID, lr.Namespace)
		return nil
	}

	// 4. 从 LimitRange 中提取限制配置并更新工作空间
	h.updateWorkspaceFromLimitRange(workspace, lr)

	// 5. 更新工作空间
	if err := h.svcCtx.ProjectWorkspaceModel.Update(ctx, workspace); err != nil {
		return fmt.Errorf("更新工作空间失败: %v", err)
	}

	logger.Infof("[LimitRange-SYNC] 更新工作空间成功: ID=%d, Namespace=%s, Changes=%v", workspace.Id, workspace.Namespace, changes)
	return nil
}

// checkLimitRangeChanges 检查 LimitRange 是否有变更，返回变更列表
func (h *DefaultEventHandler) checkLimitRangeChanges(workspace *model.OnecProjectWorkspace, lr *corev1.LimitRange) []string {
	var changes []string

	for _, limit := range lr.Spec.Limits {
		switch limit.Type {
		case corev1.LimitTypePod:
			// Pod 级别最大限制
			if cpu, ok := limit.Max[corev1.ResourceCPU]; ok {
				if workspace.PodMaxCpu != cpu.String() {
					changes = append(changes, fmt.Sprintf("PodMaxCpu: %s -> %s", workspace.PodMaxCpu, cpu.String()))
				}
			}
			if mem, ok := limit.Max[corev1.ResourceMemory]; ok {
				if workspace.PodMaxMemory != mem.String() {
					changes = append(changes, fmt.Sprintf("PodMaxMemory: %s -> %s", workspace.PodMaxMemory, mem.String()))
				}
			}
			if ephStorage, ok := limit.Max[corev1.ResourceEphemeralStorage]; ok {
				if workspace.PodMaxEphemeralStorage != ephStorage.String() {
					changes = append(changes, fmt.Sprintf("PodMaxEphemeralStorage: %s -> %s", workspace.PodMaxEphemeralStorage, ephStorage.String()))
				}
			}

			// Pod 级别最小限制
			if cpu, ok := limit.Min[corev1.ResourceCPU]; ok {
				if workspace.PodMinCpu != cpu.String() {
					changes = append(changes, fmt.Sprintf("PodMinCpu: %s -> %s", workspace.PodMinCpu, cpu.String()))
				}
			}
			if mem, ok := limit.Min[corev1.ResourceMemory]; ok {
				if workspace.PodMinMemory != mem.String() {
					changes = append(changes, fmt.Sprintf("PodMinMemory: %s -> %s", workspace.PodMinMemory, mem.String()))
				}
			}
			if ephStorage, ok := limit.Min[corev1.ResourceEphemeralStorage]; ok {
				if workspace.PodMinEphemeralStorage != ephStorage.String() {
					changes = append(changes, fmt.Sprintf("PodMinEphemeralStorage: %s -> %s", workspace.PodMinEphemeralStorage, ephStorage.String()))
				}
			}

		case corev1.LimitTypeContainer:
			// Container 级别最大限制
			if cpu, ok := limit.Max[corev1.ResourceCPU]; ok {
				if workspace.ContainerMaxCpu != cpu.String() {
					changes = append(changes, fmt.Sprintf("ContainerMaxCpu: %s -> %s", workspace.ContainerMaxCpu, cpu.String()))
				}
			}
			if mem, ok := limit.Max[corev1.ResourceMemory]; ok {
				if workspace.ContainerMaxMemory != mem.String() {
					changes = append(changes, fmt.Sprintf("ContainerMaxMemory: %s -> %s", workspace.ContainerMaxMemory, mem.String()))
				}
			}
			if ephStorage, ok := limit.Max[corev1.ResourceEphemeralStorage]; ok {
				if workspace.ContainerMaxEphemeralStorage != ephStorage.String() {
					changes = append(changes, fmt.Sprintf("ContainerMaxEphemeralStorage: %s -> %s", workspace.ContainerMaxEphemeralStorage, ephStorage.String()))
				}
			}

			// Container 级别最小限制
			if cpu, ok := limit.Min[corev1.ResourceCPU]; ok {
				if workspace.ContainerMinCpu != cpu.String() {
					changes = append(changes, fmt.Sprintf("ContainerMinCpu: %s -> %s", workspace.ContainerMinCpu, cpu.String()))
				}
			}
			if mem, ok := limit.Min[corev1.ResourceMemory]; ok {
				if workspace.ContainerMinMemory != mem.String() {
					changes = append(changes, fmt.Sprintf("ContainerMinMemory: %s -> %s", workspace.ContainerMinMemory, mem.String()))
				}
			}
			if ephStorage, ok := limit.Min[corev1.ResourceEphemeralStorage]; ok {
				if workspace.ContainerMinEphemeralStorage != ephStorage.String() {
					changes = append(changes, fmt.Sprintf("ContainerMinEphemeralStorage: %s -> %s", workspace.ContainerMinEphemeralStorage, ephStorage.String()))
				}
			}

			// Container 默认限制 (limits)
			if cpu, ok := limit.Default[corev1.ResourceCPU]; ok {
				if workspace.ContainerDefaultCpu != cpu.String() {
					changes = append(changes, fmt.Sprintf("ContainerDefaultCpu: %s -> %s", workspace.ContainerDefaultCpu, cpu.String()))
				}
			}
			if mem, ok := limit.Default[corev1.ResourceMemory]; ok {
				if workspace.ContainerDefaultMemory != mem.String() {
					changes = append(changes, fmt.Sprintf("ContainerDefaultMemory: %s -> %s", workspace.ContainerDefaultMemory, mem.String()))
				}
			}
			if ephStorage, ok := limit.Default[corev1.ResourceEphemeralStorage]; ok {
				if workspace.ContainerDefaultEphemeralStorage != ephStorage.String() {
					changes = append(changes, fmt.Sprintf("ContainerDefaultEphemeralStorage: %s -> %s", workspace.ContainerDefaultEphemeralStorage, ephStorage.String()))
				}
			}

			// Container 默认请求 (requests)
			if cpu, ok := limit.DefaultRequest[corev1.ResourceCPU]; ok {
				if workspace.ContainerDefaultRequestCpu != cpu.String() {
					changes = append(changes, fmt.Sprintf("ContainerDefaultRequestCpu: %s -> %s", workspace.ContainerDefaultRequestCpu, cpu.String()))
				}
			}
			if mem, ok := limit.DefaultRequest[corev1.ResourceMemory]; ok {
				if workspace.ContainerDefaultRequestMemory != mem.String() {
					changes = append(changes, fmt.Sprintf("ContainerDefaultRequestMemory: %s -> %s", workspace.ContainerDefaultRequestMemory, mem.String()))
				}
			}
			if ephStorage, ok := limit.DefaultRequest[corev1.ResourceEphemeralStorage]; ok {
				if workspace.ContainerDefaultRequestEphemeralStorage != ephStorage.String() {
					changes = append(changes, fmt.Sprintf("ContainerDefaultRequestEphemeralStorage: %s -> %s", workspace.ContainerDefaultRequestEphemeralStorage, ephStorage.String()))
				}
			}
		}
	}

	return changes
}

// handleLimitRangeDelete 处理 LimitRange 删除事件
func (h *DefaultEventHandler) handleLimitRangeDelete(ctx context.Context, clusterUUID string, lr *corev1.LimitRange) error {
	logger := logx.WithContext(ctx)
	logger.Infof("[LimitRange-DELETE] ClusterUUID: %s, Namespace: %s, Name: %s", clusterUUID, lr.Namespace, lr.Name)

	// 查询对应的工作空间
	workspaces, err := h.svcCtx.ProjectWorkspaceModel.FindAllByClusterUuidNamespaceIncludeDeleted(ctx, clusterUUID, lr.Namespace)
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
		logger.Debugf("[LimitRange-DELETE] 未找到对应的工作空间，跳过: ClusterUUID=%s, Namespace=%s", clusterUUID, lr.Namespace)
		return nil
	}

	// 重置工作空间的 LimitRange 相关字段为默认值
	h.resetWorkspaceLimitRangeFields(workspace)

	// 更新工作空间
	if err := h.svcCtx.ProjectWorkspaceModel.Update(ctx, workspace); err != nil {
		return fmt.Errorf("更新工作空间失败: %v", err)
	}

	logger.Infof("[LimitRange-DELETE] 重置工作空间限制配置成功: ID=%d, Namespace=%s", workspace.Id, workspace.Namespace)
	return nil
}

// resetWorkspaceLimitRangeFields 重置工作空间的 LimitRange 相关字段为默认值
func (h *DefaultEventHandler) resetWorkspaceLimitRangeFields(workspace *model.OnecProjectWorkspace) {
	// 重置 Pod 级别限制
	workspace.PodMaxCpu = "0"
	workspace.PodMaxMemory = "0Gi"
	workspace.PodMaxEphemeralStorage = "0Gi"
	workspace.PodMinCpu = "0"
	workspace.PodMinMemory = "0Mi"
	workspace.PodMinEphemeralStorage = "0Mi"

	// 重置 Container 级别最大/最小限制
	workspace.ContainerMaxCpu = "0"
	workspace.ContainerMaxMemory = "0Gi"
	workspace.ContainerMaxEphemeralStorage = "0Gi"
	workspace.ContainerMinCpu = "0"
	workspace.ContainerMinMemory = "0Mi"
	workspace.ContainerMinEphemeralStorage = "0Mi"

	// 重置 Container 默认限制 (limits)
	workspace.ContainerDefaultCpu = "100m"
	workspace.ContainerDefaultMemory = "128Mi"
	workspace.ContainerDefaultEphemeralStorage = "0Gi"

	// 重置 Container 默认请求 (requests)
	workspace.ContainerDefaultRequestCpu = "50m"
	workspace.ContainerDefaultRequestMemory = "64Mi"
	workspace.ContainerDefaultRequestEphemeralStorage = "0Mi"

	// 更新操作者
	workspace.UpdatedBy = SystemOperator
}

// updateWorkspaceFromLimitRange 从 LimitRange 更新工作空间字段
func (h *DefaultEventHandler) updateWorkspaceFromLimitRange(workspace *model.OnecProjectWorkspace, lr *corev1.LimitRange) {
	for _, limit := range lr.Spec.Limits {
		switch limit.Type {
		case corev1.LimitTypePod:
			// Pod 级别最大限制
			if cpu, ok := limit.Max[corev1.ResourceCPU]; ok {
				workspace.PodMaxCpu = cpu.String()
			}
			if mem, ok := limit.Max[corev1.ResourceMemory]; ok {
				workspace.PodMaxMemory = mem.String()
			}
			if ephStorage, ok := limit.Max[corev1.ResourceEphemeralStorage]; ok {
				workspace.PodMaxEphemeralStorage = ephStorage.String()
			}

			// Pod 级别最小限制
			if cpu, ok := limit.Min[corev1.ResourceCPU]; ok {
				workspace.PodMinCpu = cpu.String()
			}
			if mem, ok := limit.Min[corev1.ResourceMemory]; ok {
				workspace.PodMinMemory = mem.String()
			}
			if ephStorage, ok := limit.Min[corev1.ResourceEphemeralStorage]; ok {
				workspace.PodMinEphemeralStorage = ephStorage.String()
			}

		case corev1.LimitTypeContainer:
			// Container 级别最大限制
			if cpu, ok := limit.Max[corev1.ResourceCPU]; ok {
				workspace.ContainerMaxCpu = cpu.String()
			}
			if mem, ok := limit.Max[corev1.ResourceMemory]; ok {
				workspace.ContainerMaxMemory = mem.String()
			}
			if ephStorage, ok := limit.Max[corev1.ResourceEphemeralStorage]; ok {
				workspace.ContainerMaxEphemeralStorage = ephStorage.String()
			}

			// Container 级别最小限制
			if cpu, ok := limit.Min[corev1.ResourceCPU]; ok {
				workspace.ContainerMinCpu = cpu.String()
			}
			if mem, ok := limit.Min[corev1.ResourceMemory]; ok {
				workspace.ContainerMinMemory = mem.String()
			}
			if ephStorage, ok := limit.Min[corev1.ResourceEphemeralStorage]; ok {
				workspace.ContainerMinEphemeralStorage = ephStorage.String()
			}

			// Container 默认限制 (limits)
			if cpu, ok := limit.Default[corev1.ResourceCPU]; ok {
				workspace.ContainerDefaultCpu = cpu.String()
			}
			if mem, ok := limit.Default[corev1.ResourceMemory]; ok {
				workspace.ContainerDefaultMemory = mem.String()
			}
			if ephStorage, ok := limit.Default[corev1.ResourceEphemeralStorage]; ok {
				workspace.ContainerDefaultEphemeralStorage = ephStorage.String()
			}

			// Container 默认请求 (requests)
			if cpu, ok := limit.DefaultRequest[corev1.ResourceCPU]; ok {
				workspace.ContainerDefaultRequestCpu = cpu.String()
			}
			if mem, ok := limit.DefaultRequest[corev1.ResourceMemory]; ok {
				workspace.ContainerDefaultRequestMemory = mem.String()
			}
			if ephStorage, ok := limit.DefaultRequest[corev1.ResourceEphemeralStorage]; ok {
				workspace.ContainerDefaultRequestEphemeralStorage = ephStorage.String()
			}
		}
	}

	workspace.UpdatedBy = SystemOperator
}
