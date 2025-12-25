package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8sTypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateUpdateStrategyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateUpdateStrategyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUpdateStrategyLogic {
	return &UpdateUpdateStrategyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateUpdateStrategyLogic) UpdateUpdateStrategy(req *types.UpdateStrategyRequest) (resp string, err error) {
	versionDetail, controller, err := getResourceController(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取资源控制器失败: %v", err)
		return "", err
	}

	updateReq := &k8sTypes.UpdateStrategyRequest{
		Name:      versionDetail.ResourceName,
		Namespace: versionDetail.Namespace,
		Type:      req.Type,
	}

	if req.RollingUpdate != nil {
		updateReq.RollingUpdate = &k8sTypes.RollingUpdateConfig{
			MaxUnavailable: req.RollingUpdate.MaxUnavailable,
			MaxSurge:       req.RollingUpdate.MaxSurge,
			Partition:      req.RollingUpdate.Partition,
		}
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	switch resourceType {
	case "DEPLOYMENT":
		err = controller.Deployment.UpdateStrategy(updateReq)
	case "STATEFULSET":
		err = controller.StatefulSet.UpdateStrategy(updateReq)
	case "DAEMONSET":
		err = controller.DaemonSet.UpdateStrategy(updateReq)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持修改更新策略", resourceType)
	}

	// 构建滚动更新详情
	rollingDetail := ""
	if req.RollingUpdate != nil {
		rollingDetail = fmt.Sprintf(", MaxUnavailable: %s, MaxSurge: %s", req.RollingUpdate.MaxUnavailable, req.RollingUpdate.MaxSurge)
		if req.RollingUpdate.Partition > 0 {
			rollingDetail += fmt.Sprintf(", Partition: %d", req.RollingUpdate.Partition)
		}
	}

	if err != nil {
		l.Errorf("修改更新策略失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改更新策略",
			fmt.Sprintf("%s %s/%s 修改更新策略失败, 目标策略: %s%s, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, req.Type, rollingDetail, err), 2)
		return "", fmt.Errorf("修改更新策略失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改更新策略",
		fmt.Sprintf("%s %s/%s 修改更新策略成功, 新策略: %s%s", resourceType, versionDetail.Namespace, versionDetail.ResourceName, req.Type, rollingDetail), 1)
	return "修改更新策略成功", nil
}
