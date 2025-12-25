package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新仓库项目
func NewUpdateProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProjectLogic {
	return &UpdateProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateProjectLogic) UpdateProject(req *types.UpdateProjectRequest) (resp string, err error) {
	// 🔧 修复：设置默认单位
	storageUnit := req.StorageUnit
	if storageUnit == "" {
		storageUnit = "GB" // 默认为 GB
	}

	_, err = l.svcCtx.RepositoryRpc.UpdateProject(l.ctx, &pb.UpdateProjectReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		IsPublic:     req.IsPublic,
		StorageLimit: req.StorageLimit,
		StorageUnit:  storageUnit,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("更新仓库项目成功: ProjectName=%s, StorageLimit=%d%s",
		req.ProjectName, req.StorageLimit, storageUnit)
	return "更新仓库项目成功", nil
}
