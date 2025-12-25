package autoscaling

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/logic"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type VPADeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 VPA 资源
func NewVPADeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VPADeleteLogic {
	return &VPADeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *VPADeleteLogic) VPADelete(req *types.VersionIdRequest) (resp string, err error) {
	client, versionDetail, err := logic.GetClusterClientAndVersion(l.ctx, l.svcCtx, req.VersionId, l.Logger)
	if err != nil {
		return "", err
	}

	vpaOperator := client.VPA()

	// 尝试多种可能的 VPA 名称
	possibleNames := []string{
		versionDetail.ResourceName,
		fmt.Sprintf("%s-vpa", versionDetail.ResourceName),
		fmt.Sprintf("vpa-%s", versionDetail.ResourceName),
	}

	var deletedName string
	var deleteErr error

	for _, name := range possibleNames {
		if err := vpaOperator.Delete(versionDetail.Namespace, name); err == nil {
			deletedName = name
			break
		} else {
			deleteErr = err
		}
	}

	if deletedName == "" {
		l.Errorf("删除 VPA 资源失败: %v", deleteErr)
		return "", fmt.Errorf("未找到与资源 %s 关联的 VPA", versionDetail.ResourceName)
	}

	logic.AddAuditLog(l.ctx, l.svcCtx, versionDetail, "删除 VPA", "DELETE",
		fmt.Sprintf("删除 VPA 资源: %s", deletedName), l.Logger)

	return fmt.Sprintf("VPA 资源 %s 删除成功", deletedName), nil
}
