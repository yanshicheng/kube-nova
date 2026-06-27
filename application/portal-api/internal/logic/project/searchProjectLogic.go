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
	userId := req.UserId
	isSystem := req.IsSystem
	platformId := req.PlatformId
	if !isSuperAdmin(currentRoles(l.ctx)) {
		userId = currentUserID(l.ctx)
		isSystem = 0
		if platformId == 0 {
			platformId, _ = l.ctx.Value("platformId").(uint64)
		}
		if userId == 0 {
			return &types.PortalSearchProjectResp{Data: []types.PortalProject{}, Total: 0}, nil
		}
		if platformId == 0 {
			return &types.PortalSearchProjectResp{Data: []types.PortalProject{}, Total: 0}, nil
		}
	}
	if isSuperAdmin(currentRoles(l.ctx)) && platformId == 0 {
		platformId, _ = l.ctx.Value("platformId").(uint64)
	}
	rpcResp, err := l.svcCtx.ProjectRpc.ListProjects(l.ctx, &pb.PortalListProjectsReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		Name:       req.Name,
		Uuid:       req.Uuid,
		IsSystem:   isSystem,
		UserId:     userId,
		PlatformId: platformId,
	})
	if err != nil {
		l.Errorf("搜索项目失败: %v", err)
		return nil, err
	}

	return &types.PortalSearchProjectResp{
		Data:  projectsToType(rpcResp.Data),
		Total: rpcResp.Total,
	}, nil
}
