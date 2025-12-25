package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/client/repositoryservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateProjectQuotaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新项目配额
func NewUpdateProjectQuotaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProjectQuotaLogic {
	return &UpdateProjectQuotaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateProjectQuotaLogic) UpdateProjectQuota(req *types.UpdateProjectQuotaRequest) (resp string, err error) {
	// 🔧 修复：设置默认单位
	storageUnit := req.StorageUnit
	if storageUnit == "" {
		storageUnit = "GB" // 默认为 GB
	}

	_, err = l.svcCtx.RepositoryRpc.UpdateProjectQuota(l.ctx, &repositoryservice.UpdateProjectQuotaReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		//StorageLimit: req.StorageLimit,
		//StorageUnit:  storageUnit,
		CountLimit: req.CountLimit,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("更新项目配额成功: ProjectName=%s, StorageLimit=%d%s",
		req.ProjectName, req.StorageLimit, storageUnit)
	return "更新项目配额成功", nil
}
