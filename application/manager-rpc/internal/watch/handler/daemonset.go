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
		return nil

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
			logger.Debugf("[DaemonSet-ADD] 平台创建的资源，跳过: %s/%s", ds.Namespace, ds.Name)
			return nil
		}
	}

	logger.Infof("[DaemonSet-ADD] ClusterUUID: %s, Namespace: %s, Name: %s",
		clusterUUID, ds.Namespace, ds.Name)

	// 同步到数据库
	return h.syncWorkloadToDatabase(ctx, clusterUUID, ds.Namespace, ds.Name, "daemonset", ds.Labels, ds.Annotations)
}

// handleDaemonSetDelete 处理 DaemonSet 删除事件
func (h *DefaultEventHandler) handleDaemonSetDelete(ctx context.Context, clusterUUID string, ds *appsv1.DaemonSet) error {
	logger := logx.WithContext(ctx)
	logger.Infof("[DaemonSet-DELETE] ClusterUUID: %s, Namespace: %s, Name: %s",
		clusterUUID, ds.Namespace, ds.Name)

	return h.deleteWorkloadFromDatabase(ctx, clusterUUID, ds.Namespace, ds.Name, "daemonset")
}
