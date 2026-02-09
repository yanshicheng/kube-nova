package handler

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/watch/incremental"
)

// HandleDeploymentEvent 处理 Deployment 事件
func (h *DefaultEventHandler) HandleDeploymentEvent(ctx context.Context, event *incremental.ResourceEvent) error {
	switch event.Type {
	case incremental.EventAdd:
		deploy, ok := event.NewObject.(*appsv1.Deployment)
		if !ok {
			return errors.New("invalid deployment object")
		}
		return h.handleDeploymentAdd(ctx, event.ClusterUUID, deploy)

	case incremental.EventUpdate:
		deploy, ok := event.NewObject.(*appsv1.Deployment)
		if !ok {
			return errors.New("invalid deployment object")
		}
		return h.handleDeploymentUpdate(ctx, event.ClusterUUID, deploy)

	case incremental.EventDelete:
		deploy, ok := event.OldObject.(*appsv1.Deployment)
		if !ok {
			return errors.New("invalid deployment object")
		}
		return h.handleDeploymentDelete(ctx, event.ClusterUUID, deploy)
	}

	return nil
}

// handleDeploymentAdd 处理 Deployment 创建事件
func (h *DefaultEventHandler) handleDeploymentAdd(ctx context.Context, clusterUUID string, deploy *appsv1.Deployment) error {
	logger := logx.WithContext(ctx)

	// 如果是平台创建的资源，跳过
	// 平台创建的资源会带有 ikubeops.com/managed-by: kube-nova 注解
	if deploy.Annotations != nil {
		if managedBy, ok := deploy.Annotations[utils.AnnotationManagedBy]; ok && managedBy == utils.ManagedByPlatform {
			logger.Infof("[Deployment-ADD] 平台创建的资源，跳过同步: %s/%s (注解: %s=%s)",
				deploy.Namespace, deploy.Name, utils.AnnotationManagedBy, managedBy)
			return nil
		}
	}

	logger.Infof("[Deployment-ADD] ClusterUUID: %s, Namespace: %s, Name: %s",
		clusterUUID, deploy.Namespace, deploy.Name)

	// 计算初始状态
	status := CalculateDeploymentStatus(deploy)

	// 同步到数据库，传入 labels 和 annotations 用于 Flagger 识别，以及计算好的状态
	return h.syncWorkloadToDatabase(ctx, clusterUUID, deploy.Namespace, deploy.Name, "deployment", deploy.Labels, deploy.Annotations, status)
}

// handleDeploymentUpdate 处理 Deployment 更新事件
// 主要用于更新版本的 status 字段（根据副本数是否满足判断健康状态）
func (h *DefaultEventHandler) handleDeploymentUpdate(ctx context.Context, clusterUUID string, deploy *appsv1.Deployment) error {
	logger := logx.WithContext(ctx)

	// 如果是平台创建的资源，也需要更新状态
	// 这里不跳过，因为状态更新对所有资源都适用

	// 正在删除中的跳过
	if deploy.DeletionTimestamp != nil {
		logger.Debugf("[Deployment-UPDATE] 资源正在删除中，跳过状态更新: %s/%s", deploy.Namespace, deploy.Name)
		return nil
	}

	// 计算新状态
	newStatus := CalculateDeploymentStatus(deploy)

	logger.Debugf("[Deployment-UPDATE] ClusterUUID: %s, Namespace: %s, Name: %s, CalculatedStatus: %d",
		clusterUUID, deploy.Namespace, deploy.Name, newStatus)

	// 更新数据库中的状态
	return h.updateWorkloadStatus(ctx, clusterUUID, deploy.Namespace, deploy.Name, "deployment", newStatus)
}

// handleDeploymentDelete 处理 Deployment 删除事件
func (h *DefaultEventHandler) handleDeploymentDelete(ctx context.Context, clusterUUID string, deploy *appsv1.Deployment) error {
	logger := logx.WithContext(ctx)
	logger.Infof("[Deployment-DELETE] ClusterUUID: %s, Namespace: %s, Name: %s",
		clusterUUID, deploy.Namespace, deploy.Name)

	// 从数据库删除（硬删除），如果不存在会自动跳过
	// 传入 annotations 用于检测 API 删除标记
	return h.deleteWorkloadFromDatabase(ctx, clusterUUID, deploy.Namespace, deploy.Name, "deployment", deploy.Annotations)
}
