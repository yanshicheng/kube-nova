package application

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type VersionSearchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询版本列表
func NewVersionSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VersionSearchLogic {
	return &VersionSearchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *VersionSearchLogic) VersionSearch(req *types.SearchOnecProjectVersionReq) (resp []types.OnecProjectVersion, err error) {
	searchResp, err := l.svcCtx.ManagerRpc.VersionSearch(l.ctx, &managerservice.SearchOnecProjectVersionReq{
		ApplicationId: req.ApplicationId,
	})
	if err != nil {
		l.Errorf("查询版本列表失败: %v", err)
		return nil, err
	}

	var versions []types.OnecProjectVersion
	for _, ver := range searchResp.Data {
		versions = append(versions, types.OnecProjectVersion{
			Id:            ver.Id,
			ApplicationId: ver.ApplicationId,
			Version:       ver.Version,
			ResourceName:  ver.ResourceName,
			CreatedBy:     ver.CreatedBy,
			UpdatedBy:     ver.UpdatedBy,
			CreatedAt:     ver.CreatedAt,
			UpdatedAt:     ver.UpdatedAt,
			Status:        ver.Status,
			ParentAppName: ver.ParentAppName,
			VersionRole:   ver.VersionRole,
		})
	}

	return versions, nil
}
