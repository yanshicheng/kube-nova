package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

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
	platformId := currentPlatformID(l.ctx)
	if !isSuperAdmin(currentRoles(l.ctx)) {
		userId := currentUserID(l.ctx)
		if userId == 0 {
			return nil, errorx.Msg("无项目访问权限")
		}
		if platformId == 0 {
			return nil, errorx.Msg("无项目访问权限")
		}
		listResp, err := l.svcCtx.ProjectRpc.ListProjects(l.ctx, &pb.PortalListProjectsReq{
			Page:       1,
			PageSize:   1,
			Id:         req.Id,
			IsSystem:   0,
			UserId:     userId,
			PlatformId: platformId,
		})
		if err != nil {
			l.Errorf("校验项目权限失败: %v", err)
			return nil, err
		}
		if len(listResp.Data) == 0 {
			return nil, errorx.Msg("无项目访问权限")
		}
	}

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

	data := projectToType(rpcResp.Data)
	return &data, nil
}
