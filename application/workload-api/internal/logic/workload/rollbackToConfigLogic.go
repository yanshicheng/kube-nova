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

type RollbackToConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRollbackToConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RollbackToConfigLogic {
	return &RollbackToConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RollbackToConfigLogic) RollbackToConfig(req *types.RollbackToConfigRequest) (resp string, err error) {
	versionDetail, controller, err := getResourceController(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取资源控制器失败: %v", err)
		return "", err
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	// 获取配置历史信息（用于审计日志记录）
	var configHistory []k8sTypes.ConfigHistoryInfo
	switch resourceType {
	case "STATEFULSET":
		configHistory, err = controller.StatefulSet.GetConfigHistory(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		configHistory, err = controller.DaemonSet.GetConfigHistory(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持回滚到配置操作", resourceType)
	}

	// 查找目标配置历史的描述信息
	var targetConfigDesc string
	if err == nil && configHistory != nil {
		for _, ch := range configHistory {
			if ch.Id == req.ConfigHistoryId {
				targetConfigDesc = fmt.Sprintf("创建时间: %s, 原因: %s", ch.CreatedAt, ch.Reason)
				break
			}
		}
	}

	rollbackReq := &k8sTypes.RollbackToConfigRequest{
		Name:            versionDetail.ResourceName,
		Namespace:       versionDetail.Namespace,
		ConfigHistoryId: req.ConfigHistoryId,
	}

	// 执行回滚
	switch resourceType {
	case "STATEFULSET":
		err = controller.StatefulSet.RollbackToConfig(rollbackReq)
	case "DAEMONSET":
		err = controller.DaemonSet.RollbackToConfig(rollbackReq)
	}

	// 生成变更详情
	changeDetail := BuildConfigRollbackDescription(resourceType, versionDetail.Namespace, versionDetail.ResourceName, int64(req.ConfigHistoryId))
	if targetConfigDesc != "" {
		changeDetail += fmt.Sprintf(" (%s)", targetConfigDesc)
	}

	if err != nil {
		l.Errorf("回滚到配置失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "配置回滚",
			fmt.Sprintf("%s 失败: %v", changeDetail, err), 2)
		return "", fmt.Errorf("回滚失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "配置回滚",
		fmt.Sprintf("%s 成功", changeDetail), 1)
	return "回滚成功", nil
}
