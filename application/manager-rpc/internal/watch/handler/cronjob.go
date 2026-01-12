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
// - UPDATE: 跳过（按需求只处理新增和删除）
// - DELETE: 软删除版本记录
func (h *DefaultEventHandler) HandleCronJobEvent(ctx context.Context, event *incremental.ResourceEvent) error {
	switch event.Type {
	case incremental.EventAdd:
		cj, ok := event.NewObject.(*batchv1.CronJob)
		if !ok {
			return errors.New("invalid cronjob object")
		}
		return h.handleCronJobAdd(ctx, event.ClusterUUID, cj)

	case incremental.EventUpdate:
		return nil

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

	// 同步到数据库
	return h.syncWorkloadToDatabase(ctx, clusterUUID, cj.Namespace, cj.Name, "cronjob", cj.Labels, cj.Annotations)
}

// handleCronJobDelete 处理 CronJob 删除事件
func (h *DefaultEventHandler) handleCronJobDelete(ctx context.Context, clusterUUID string, cj *batchv1.CronJob) error {
	logger := logx.WithContext(ctx)
	logger.Infof("[CronJob-DELETE] ClusterUUID: %s, Namespace: %s, Name: %s",
		clusterUUID, cj.Namespace, cj.Name)

	return h.deleteWorkloadFromDatabase(ctx, clusterUUID, cj.Namespace, cj.Name, "cronjob")
}
