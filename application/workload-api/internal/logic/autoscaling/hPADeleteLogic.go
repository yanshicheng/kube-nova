package autoscaling

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/logic"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type HPADeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 HPA 资源
func NewHPADeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HPADeleteLogic {
	return &HPADeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *HPADeleteLogic) HPADelete(req *types.VersionIdRequest) (resp string, err error) {
	client, versionDetail, err := logic.GetClusterClientAndVersion(l.ctx, l.svcCtx, req.VersionId, l.Logger)
	if err != nil {
		return "", err
	}

	hpaOperator := client.HPA()

	possibleNames := []string{
		versionDetail.ResourceName,
		fmt.Sprintf("%s-hpa", versionDetail.ResourceName),
		fmt.Sprintf("hpa-%s", versionDetail.ResourceName),
	}

	var deletedName string
	var deleteErr error

	for _, name := range possibleNames {
		if err := hpaOperator.Delete(versionDetail.Namespace, name); err == nil {
			deletedName = name
			break
		} else {
			deleteErr = err
		}
	}

	if deletedName == "" {
		l.Errorf("删除 HPA 资源失败: %v", deleteErr)
		return "", fmt.Errorf("未找到与资源 %s 关联的 HPA", versionDetail.ResourceName)
	}

	logic.AddAuditLog(l.ctx, l.svcCtx, versionDetail, "删除 HPA", "DELETE",
		fmt.Sprintf("删除 HPA 资源: %s", deletedName), l.Logger)

	l.Infof("HPA 资源删除成功: %s", deletedName)
	return fmt.Sprintf("HPA 资源 %s 删除成功", deletedName), nil
}
