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

type RollbackToRevisionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRollbackToRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RollbackToRevisionLogic {
	return &RollbackToRevisionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RollbackToRevisionLogic) RollbackToRevision(req *types.RollbackToRevisionRequest) (resp string, err error) {
	versionDetail, controller, err := getResourceController(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取资源控制器失败: %v", err)
		return "", err
	}

	rollbackReq := &k8sTypes.RollbackToRevisionRequest{
		Name:      versionDetail.ResourceName,
		Namespace: versionDetail.Namespace,
		Revision:  req.Revision,
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	switch resourceType {
	case "DEPLOYMENT":
		err = controller.Deployment.Rollback(rollbackReq)
	case "DAEMONSET":
		err = controller.DaemonSet.Rollback(rollbackReq)
	case "STATEFULSET":
		err = controller.StatefulSet.Rollback(rollbackReq)
	default:
		return "", fmt.Errorf("资源类型 %s 不支持回滚到版本操作", resourceType)
	}

	if err != nil {
		l.Errorf("回滚到版本失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "版本回滚",
			fmt.Sprintf("%s %s/%s 回滚到版本 %d 失败: %v", resourceType, versionDetail.Namespace, versionDetail.ResourceName, req.Revision, err), 2)
		return "", fmt.Errorf("回滚失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "版本回滚",
		fmt.Sprintf("%s %s/%s 成功回滚到版本 %d", resourceType, versionDetail.Namespace, versionDetail.ResourceName, req.Revision), 1)
	return "回滚成功", nil
}
