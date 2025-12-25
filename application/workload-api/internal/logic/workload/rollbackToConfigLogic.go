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

	rollbackReq := &k8sTypes.RollbackToConfigRequest{
		Name:            versionDetail.ResourceName,
		Namespace:       versionDetail.Namespace,
		ConfigHistoryId: req.ConfigHistoryId,
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	switch resourceType {
	case "STATEFULSET":
		err = controller.StatefulSet.RollbackToConfig(rollbackReq)
	case "DAEMONSET":
		err = controller.DaemonSet.RollbackToConfig(rollbackReq)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持回滚到配置操作", resourceType)
	}

	if err != nil {
		l.Errorf("回滚到配置失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "配置回滚",
			fmt.Sprintf("资源 %s/%s 回滚到配置历史ID %d 失败: %v", versionDetail.Namespace, versionDetail.ResourceName, req.ConfigHistoryId, err), 2)
		return "", fmt.Errorf("回滚失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "配置回滚",
		fmt.Sprintf("资源 %s/%s 成功回滚到配置历史ID %d", versionDetail.Namespace, versionDetail.ResourceName, req.ConfigHistoryId), 1)
	return "回滚成功", nil
}
