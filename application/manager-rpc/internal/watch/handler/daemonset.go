package handler

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/incremental"
)

// HandleDaemonSetEvent 处理 DaemonSet 事件
func (h *DefaultEventHandler) HandleDaemonSetEvent(ctx context.Context, event *incremental.ResourceEvent) error {
	switch event.Type {
	case incremental.EventAdd:
		ds, ok := event.NewObject.(*appsv1.DaemonSet)
		if !ok {
			return errors.New("invalid daemonset object")
		}
		return h.handleDaemonSetAdd(ctx, event.ClusterUUID, ds)

	case incremental.EventUpdate:
		ds, ok := event.NewObject.(*appsv1.DaemonSet)
		if !ok {
			return errors.New("invalid daemonset object")
		}
		return h.handleDaemonSetUpdate(ctx, event.ClusterUUID, ds)

	case incremental.EventDelete:
		ds, ok := event.OldObject.(*appsv1.DaemonSet)
		if !ok {
			return errors.New("invalid daemonset object")
		}
		return h.handleDaemonSetDelete(ctx, event.ClusterUUID, ds)
	}

	return nil
}

// handleDaemonSetAdd 处理 DaemonSet 创建事件
func (h *DefaultEventHandler) handleDaemonSetAdd(ctx context.Context, clusterUUID string, ds *appsv1.DaemonSet) error {
	logger := logx.WithContext(ctx)

	// 如果是平台创建的资源，跳过
	if ds.Annotations != nil {
		if managedBy, ok := ds.Annotations[utils.AnnotationManagedBy]; ok && managedBy == utils.ManagedByPlatform {
			logger.Infof("[DaemonSet-ADD] 平台创建的资源，跳过同步: %s/%s (注解: %s=%s)",
				ds.Namespace, ds.Name, utils.AnnotationManagedBy, managedBy)
			return nil
		}
	}

	logger.Infof("[DaemonSet-ADD] ClusterUUID: %s, Namespace: %s, Name: %s",
		clusterUUID, ds.Namespace, ds.Name)

	// 计算初始状态
	status := CalculateDaemonSetStatus(ds)

	// 同步到数据库
	return h.syncWorkloadToDatabase(ctx, clusterUUID, ds.Namespace, ds.Name, "daemonset", ds.Labels, ds.Annotations, status)
}

// handleDaemonSetUpdate 处理 DaemonSet 更新事件
// 主要用于更新版本的 status 字段（根据就绪数量是否满足期望判断健康状态）
func (h *DefaultEventHandler) handleDaemonSetUpdate(ctx context.Context, clusterUUID string, ds *appsv1.DaemonSet) error {
	logger := logx.WithContext(ctx)

	// 正在删除中的跳过
	if ds.DeletionTimestamp != nil {
		logger.Debugf("[DaemonSet-UPDATE] 资源正在删除中，跳过状态更新: %s/%s", ds.Namespace, ds.Name)
		return nil
	}

	// 计算新状态
	newStatus := CalculateDaemonSetStatus(ds)

	logger.Debugf("[DaemonSet-UPDATE] ClusterUUID: %s, Namespace: %s, Name: %s, CalculatedStatus: %d",
		clusterUUID, ds.Namespace, ds.Name, newStatus)

	// 更新数据库中的状态
	return h.updateWorkloadStatus(ctx, clusterUUID, ds.Namespace, ds.Name, "daemonset", newStatus)
}

// handleDaemonSetDelete 处理 DaemonSet 删除事件
func (h *DefaultEventHandler) handleDaemonSetDelete(ctx context.Context, clusterUUID string, ds *appsv1.DaemonSet) error {
	logger := logx.WithContext(ctx)
	logger.Infof("[DaemonSet-DELETE] ClusterUUID: %s, Namespace: %s, Name: %s",
		clusterUUID, ds.Namespace, ds.Name)

	return h.deleteWorkloadFromDatabase(ctx, clusterUUID, ds.Namespace, ds.Name, "daemonset", ds.Annotations)
}
