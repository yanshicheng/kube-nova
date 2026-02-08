package handler

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	batchv1 "k8s.io/api/batch/v1"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/incremental"
)

// HandleCronJobEvent 处理 CronJob 事件
//
// 事件类型处理：
// - ADD: 同步到数据库（检查平台管理标记）
// - UPDATE: 更新版本状态（CronJob 状态始终为正常）
// - DELETE: 硬删除版本记录
func (h *DefaultEventHandler) HandleCronJobEvent(ctx context.Context, event *incremental.ResourceEvent) error {
	switch event.Type {
	case incremental.EventAdd:
		cj, ok := event.NewObject.(*batchv1.CronJob)
		if !ok {
			return errors.New("invalid cronjob object")
		}
		return h.handleCronJobAdd(ctx, event.ClusterUUID, cj)

	case incremental.EventUpdate:
		cj, ok := event.NewObject.(*batchv1.CronJob)
		if !ok {
			return errors.New("invalid cronjob object")
		}
		return h.handleCronJobUpdate(ctx, event.ClusterUUID, cj)

	case incremental.EventDelete:
		cj, ok := event.OldObject.(*batchv1.CronJob)
		if !ok {
			return errors.New("invalid cronjob object")
		}
		return h.handleCronJobDelete(ctx, event.ClusterUUID, cj)
	}

	return nil
}

// handleCronJobAdd 处理 CronJob 创建事件
func (h *DefaultEventHandler) handleCronJobAdd(ctx context.Context, clusterUUID string, cj *batchv1.CronJob) error {
	logger := logx.WithContext(ctx)

	// 如果是平台创建的资源，跳过
	if cj.Annotations != nil {
		if managedBy, ok := cj.Annotations[utils.AnnotationManagedBy]; ok && managedBy == utils.ManagedByPlatform {
			logger.Debugf("[CronJob-ADD] 平台创建的资源，跳过: %s/%s", cj.Namespace, cj.Name)
			return nil
		}
	}

	logger.Infof("[CronJob-ADD] ClusterUUID: %s, Namespace: %s, Name: %s",
		clusterUUID, cj.Namespace, cj.Name)

	// 计算初始状态（CronJob 状态始终为正常）
	status := CalculateCronJobStatus(cj)

	// 同步到数据库
	return h.syncWorkloadToDatabase(ctx, clusterUUID, cj.Namespace, cj.Name, "cronjob", cj.Labels, cj.Annotations, status)
}

// handleCronJobUpdate 处理 CronJob 更新事件
// CronJob 没有副本数概念，状态始终为正常
func (h *DefaultEventHandler) handleCronJobUpdate(ctx context.Context, clusterUUID string, cj *batchv1.CronJob) error {
	logger := logx.WithContext(ctx)

	// 正在删除中的跳过
	if cj.DeletionTimestamp != nil {
		logger.Debugf("[CronJob-UPDATE] 资源正在删除中，跳过状态更新: %s/%s", cj.Namespace, cj.Name)
		return nil
	}

	// 计算新状态（CronJob 状态始终为正常）
	newStatus := CalculateCronJobStatus(cj)

	logger.Debugf("[CronJob-UPDATE] ClusterUUID: %s, Namespace: %s, Name: %s, CalculatedStatus: %d",
		clusterUUID, cj.Namespace, cj.Name, newStatus)

	// 更新数据库中的状态
	return h.updateWorkloadStatus(ctx, clusterUUID, cj.Namespace, cj.Name, "cronjob", newStatus)
}

// handleCronJobDelete 处理 CronJob 删除事件
//
// 传入 annotations 用于检测 API 删除标记
func (h *DefaultEventHandler) handleCronJobDelete(ctx context.Context, clusterUUID string, cj *batchv1.CronJob) error {
	logger := logx.WithContext(ctx)
	logger.Infof("[CronJob-DELETE] ClusterUUID: %s, Namespace: %s, Name: %s",
		clusterUUID, cj.Namespace, cj.Name)

	// 传入 annotations 用于检测 API 删除标记
	return h.deleteWorkloadFromDatabase(ctx, clusterUUID, cj.Namespace, cj.Name, "cronjob", cj.Annotations)
}
