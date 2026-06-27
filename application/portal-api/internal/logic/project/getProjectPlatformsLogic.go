// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectPlatformsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取项目平台
func NewGetProjectPlatformsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectPlatformsLogic {
	return &GetProjectPlatformsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProjectPlatformsLogic) GetProjectPlatforms(req *types.PortalProjectMemberReq) (resp *types.GetProjectPlatformsResp, err error) {
	rpcResp, err := l.svcCtx.ProjectRpc.GetProjectPlatforms(l.ctx, &pb.GetProjectPlatformsReq{
		ProjectId: req.Id,
	})
	if err != nil {
		l.Errorf("获取项目平台失败: %v", err)
		return nil, err
	}

	items := make([]types.ProjectPlatformBinding, 0, len(rpcResp.Data))
	for _, item := range rpcResp.Data {
		items = append(items, types.ProjectPlatformBinding{
			Id:           item.Id,
			ProjectId:    item.ProjectId,
			PlatformId:   item.PlatformId,
			PlatformCode: item.PlatformCode,
			PlatformName: item.PlatformName,
			CreatedBy:    item.CreatedBy,
			CreatedAt:    item.CreatedAt,
		})
	}

	return &types.GetProjectPlatformsResp{Data: items}, nil
}
