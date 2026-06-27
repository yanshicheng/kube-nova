package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchProjectLogic {
	return &SearchProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchProjectLogic) SearchProject(req *types.PortalSearchProjectReq) (resp *types.PortalSearchProjectResp, err error) {
	rpcResp, err := l.svcCtx.ProjectRpc.ListProjects(l.ctx, &pb.PortalListProjectsReq{
		Page:     req.Page,
		PageSize: req.PageSize,
		Name:     req.Name,
		Uuid:     req.Uuid,
		IsSystem: req.IsSystem,
	})
	if err != nil {
		l.Errorf("搜索项目失败: %v", err)
		return nil, err
	}

	items := make([]types.PortalProject, 0, len(rpcResp.Data))
	for _, p := range rpcResp.Data {
		items = append(items, types.PortalProject{
			Id:          p.Id,
			Name:        p.Name,
			Uuid:        p.Uuid,
			IsSystem:    p.IsSystem,
			Description: p.Description,
			CreatedBy:   p.CreatedBy,
			UpdatedBy:   p.UpdatedBy,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		})
	}

	return &types.PortalSearchProjectResp{
		Data:  items,
		Total: rpcResp.Total,
	}, nil
}
