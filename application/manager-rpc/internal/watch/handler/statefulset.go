package handler

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/incremental"
)

// HandleStatefulSetEvent 处理 StatefulSet 事件
func (h *DefaultEventHandler) HandleStatefulSetEvent(ctx context.Context, event *incremental.ResourceEvent) error {
	switch event.Type {
	case incremental.EventAdd:
		sts, ok := event.NewObject.(*appsv1.StatefulSet)
		if !ok {
			return errors.New("invalid statefulset object")
		}
		return h.handleStatefulSetAdd(ctx, event.ClusterUUID, sts)

	case incremental.EventUpdate:
		// 按需求只处理新增和删除，UPDATE 事件跳过
		return nil

	case incremental.EventDelete:
		sts, ok := event.OldObject.(*appsv1.StatefulSet)
		if !ok {
			return errors.New("invalid statefulset object")
		}
		return h.handleStatefulSetDelete(ctx, event.ClusterUUID, sts)
	}

	return nil
}

// handleStatefulSetAdd 处理 StatefulSet 创建事件
func (h *DefaultEventHandler) handleStatefulSetAdd(ctx context.Context, clusterUUID string, sts *appsv1.StatefulSet) error {
	logger := logx.WithContext(ctx)

	// 如果是平台创建的资源，跳过
	if sts.Annotations != nil {
		if managedBy, ok := sts.Annotations[utils.AnnotationManagedBy]; ok && managedBy == utils.ManagedByPlatform {
			logger.Debugf("[StatefulSet-ADD] 平台创建的资源，跳过: %s/%s", sts.Namespace, sts.Name)
			return nil
		}
	}

	logger.Infof("[StatefulSet-ADD] ClusterUUID: %s, Namespace: %s, Name: %s",
		clusterUUID, sts.Namespace, sts.Name)

	// 同步到数据库
	return h.syncWorkloadToDatabase(ctx, clusterUUID, sts.Namespace, sts.Name, "statefulset", sts.Labels, sts.Annotations)
}

// handleStatefulSetDelete 处理 StatefulSet 删除事件
func (h *DefaultEventHandler) handleStatefulSetDelete(ctx context.Context, clusterUUID string, sts *appsv1.StatefulSet) error {
	logger := logx.WithContext(ctx)
	logger.Infof("[StatefulSet-DELETE] ClusterUUID: %s, Namespace: %s, Name: %s",
		clusterUUID, sts.Namespace, sts.Name)

	return h.deleteWorkloadFromDatabase(ctx, clusterUUID, sts.Namespace, sts.Name, "statefulset")
}
