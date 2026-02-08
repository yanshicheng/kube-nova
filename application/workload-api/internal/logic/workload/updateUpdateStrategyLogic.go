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

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	// 获取原更新策略配置
	var oldStrategy *k8sTypes.UpdateStrategyResponse
	switch resourceType {
	case "DEPLOYMENT":
		oldStrategy, err = controller.Deployment.GetUpdateStrategy(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		oldStrategy, err = controller.StatefulSet.GetUpdateStrategy(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		oldStrategy, err = controller.DaemonSet.GetUpdateStrategy(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持修改更新策略", resourceType)
	}

	if err != nil {
		l.Errorf("获取原更新策略配置失败: %v", err)
		// 继续执行
	}

	// 构建新的滚动更新配置
	var newRollingUpdate *k8sTypes.RollingUpdateConfig
	if req.RollingUpdate != nil {
		newRollingUpdate = &k8sTypes.RollingUpdateConfig{
			MaxUnavailable: req.RollingUpdate.MaxUnavailable,
			MaxSurge:       req.RollingUpdate.MaxSurge,
			Partition:      req.RollingUpdate.Partition,
		}
	}

	updateReq := &k8sTypes.UpdateStrategyRequest{
		Name:          versionDetail.ResourceName,
		Namespace:     versionDetail.Namespace,
		Type:          req.Type,
		RollingUpdate: newRollingUpdate,
	}

	// 执行更新
	switch resourceType {
	case "DEPLOYMENT":
		err = controller.Deployment.UpdateStrategy(updateReq)
	case "STATEFULSET":
		err = controller.StatefulSet.UpdateStrategy(updateReq)
	case "DAEMONSET":
		err = controller.DaemonSet.UpdateStrategy(updateReq)
	}

	// 生成变更详情
	var changeDetail string
	if oldStrategy != nil {
		changeDetail = CompareUpdateStrategy(oldStrategy, req.Type, newRollingUpdate)
	} else {
		changeDetail = fmt.Sprintf("更新策略变更 (无法获取原配置): 新策略类型 %s", req.Type)
		if newRollingUpdate != nil {
			changeDetail += fmt.Sprintf(", MaxUnavailable: %s, MaxSurge: %s", newRollingUpdate.MaxUnavailable, newRollingUpdate.MaxSurge)
		}
	}

	if err != nil {
		l.Errorf("修改更新策略失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改更新策略",
			fmt.Sprintf("%s %s/%s 修改更新策略失败, %s, 错误: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail, err), 2)
		return "", fmt.Errorf("修改更新策略失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改更新策略",
		fmt.Sprintf("%s %s/%s 修改更新策略成功, %s", resourceType, versionDetail.Namespace, versionDetail.ResourceName, changeDetail), 1)
	return "修改更新策略成功", nil
}
