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
		sts, ok := event.NewObject.(*appsv1.StatefulSet)
		if !ok {
			return errors.New("invalid statefulset object")
		}
		return h.handleStatefulSetUpdate(ctx, event.ClusterUUID, sts)

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

	// 计算初始状态
	status := CalculateStatefulSetStatus(sts)

	// 同步到数据库
	return h.syncWorkloadToDatabase(ctx, clusterUUID, sts.Namespace, sts.Name, "statefulset", sts.Labels, sts.Annotations, status)
}

// handleStatefulSetUpdate 处理 StatefulSet 更新事件
// 主要用于更新版本的 status 字段（根据副本数是否满足判断健康状态）
func (h *DefaultEventHandler) handleStatefulSetUpdate(ctx context.Context, clusterUUID string, sts *appsv1.StatefulSet) error {
	logger := logx.WithContext(ctx)

	// 正在删除中的跳过
	if sts.DeletionTimestamp != nil {
		logger.Debugf("[StatefulSet-UPDATE] 资源正在删除中，跳过状态更新: %s/%s", sts.Namespace, sts.Name)
		return nil
	}

	// 计算新状态
	newStatus := CalculateStatefulSetStatus(sts)

	logger.Debugf("[StatefulSet-UPDATE] ClusterUUID: %s, Namespace: %s, Name: %s, CalculatedStatus: %d",
		clusterUUID, sts.Namespace, sts.Name, newStatus)

	// 更新数据库中的状态
	return h.updateWorkloadStatus(ctx, clusterUUID, sts.Namespace, sts.Name, "statefulset", newStatus)
}

// handleStatefulSetDelete 处理 StatefulSet 删除事件
func (h *DefaultEventHandler) handleStatefulSetDelete(ctx context.Context, clusterUUID string, sts *appsv1.StatefulSet) error {
	logger := logx.WithContext(ctx)
	logger.Infof("[StatefulSet-DELETE] ClusterUUID: %s, Namespace: %s, Name: %s",
		clusterUUID, sts.Namespace, sts.Name)

	// 传入 annotations 用于检测 API 删除标记
	return h.deleteWorkloadFromDatabase(ctx, clusterUUID, sts.Namespace, sts.Name, "statefulset", sts.Annotations)
}
