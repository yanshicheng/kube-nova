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
		// 按需求只处理新增和删除，UPDATE 事件跳过
		return nil

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
			logger.Debugf("[Deployment-ADD] 平台创建的资源，跳过: %s/%s", deploy.Namespace, deploy.Name)
			return nil
		}
	}

	logger.Infof("[Deployment-ADD] ClusterUUID: %s, Namespace: %s, Name: %s",
		clusterUUID, deploy.Namespace, deploy.Name)

	// 同步到数据库，传入 labels 和 annotations 用于 Flagger 识别
	return h.syncWorkloadToDatabase(ctx, clusterUUID, deploy.Namespace, deploy.Name, "deployment", deploy.Labels, deploy.Annotations)
}

// handleDeploymentDelete 处理 Deployment 删除事件
func (h *DefaultEventHandler) handleDeploymentDelete(ctx context.Context, clusterUUID string, deploy *appsv1.Deployment) error {
	logger := logx.WithContext(ctx)
	logger.Infof("[Deployment-DELETE] ClusterUUID: %s, Namespace: %s, Name: %s",
		clusterUUID, deploy.Namespace, deploy.Name)

	// 从数据库删除（软删除），如果不存在会自动跳过
	return h.deleteWorkloadFromDatabase(ctx, clusterUUID, deploy.Namespace, deploy.Name, "deployment")
}
