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

type GetProjectByPlatformLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取平台下项目
func NewGetProjectByPlatformLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectByPlatformLogic {
	return &GetProjectByPlatformLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProjectByPlatformLogic) GetProjectByPlatform(req *types.GetProjectByPlatformReq) (resp *types.PortalSearchProjectResp, err error) {
	platformId, _ := l.ctx.Value("platformId").(uint64)
	targetPlatformId := choosePlatformID(req.PlatformId, platformId)
	if targetPlatformId == 0 {
		return &types.PortalSearchProjectResp{Data: []types.PortalProject{}, Total: 0}, nil
	}
	rpcResp, err := l.svcCtx.ProjectRpc.GetProjectByPlatform(l.ctx, &pb.GetProjectByPlatformReq{
		PlatformId: targetPlatformId,
	})
	if err != nil {
		l.Errorf("获取平台项目失败: %v", err)
		return nil, err
	}

	data := projectsToType(rpcResp.Data)
	if !isSuperAdmin(currentRoles(l.ctx)) {
		userId := currentUserID(l.ctx)
		if userId == 0 {
			return &types.PortalSearchProjectResp{Data: []types.PortalProject{}, Total: 0}, nil
		}
		allowedResp, err := l.svcCtx.ProjectRpc.ListProjects(l.ctx, &pb.PortalListProjectsReq{
			Page:       1,
			PageSize:   uint64(len(data)),
			IsSystem:   0,
			UserId:     userId,
			PlatformId: targetPlatformId,
		})
		if err != nil {
			l.Errorf("校验平台项目权限失败: %v", err)
			return nil, err
		}
		data = projectsToType(allowedResp.Data)
	}

	return &types.PortalSearchProjectResp{
		Data:  data,
		Total: uint64(len(data)),
	}, nil
}
