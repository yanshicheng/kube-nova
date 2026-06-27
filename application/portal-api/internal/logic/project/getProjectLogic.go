package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectLogic {
	return &GetProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProjectLogic) GetProject(req *types.PortalProjectIdReq) (resp *types.PortalProject, err error) {
	rpcResp, err := l.svcCtx.ProjectRpc.GetProject(l.ctx, &pb.PortalGetProjectReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return nil, err
	}

	if rpcResp.Data == nil {
		return nil, nil
	}

	return &types.PortalProject{
		Id:          rpcResp.Data.Id,
		Name:        rpcResp.Data.Name,
		Uuid:        rpcResp.Data.Uuid,
		IsSystem:    rpcResp.Data.IsSystem,
		Description: rpcResp.Data.Description,
		CreatedBy:   rpcResp.Data.CreatedBy,
		UpdatedBy:   rpcResp.Data.UpdatedBy,
		CreatedAt:   rpcResp.Data.CreatedAt,
		UpdatedAt:   rpcResp.Data.UpdatedAt,
	}, nil
}
