package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 根据ID获取项目详细信息
func NewGetProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectLogic {
	return &GetProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProjectLogic) GetProject(req *types.DefaultIdRequest) (resp *types.Project, err error) {
	userId := uint64(0)
	platformId := currentPlatformID(l.ctx)
	if !isSuperAdmin(currentRoles(l.ctx)) {
		userId = currentUserID(l.ctx)
		if userId == 0 {
			return nil, errorx.Msg("无项目访问权限")
		}
		if platformId == 0 {
			return nil, errorx.Msg("无项目访问权限")
		}
	}
	if isSuperAdmin(currentRoles(l.ctx)) && platformId == 0 {
		return nil, errorx.Msg("无项目访问权限")
	}

	result, err := l.svcCtx.ProjectRpc.ListProjects(l.ctx, &pb.PortalListProjectsReq{
		Page:     1,
		PageSize: 1,
		Id:       req.Id,
		IsSystem: 0,
		UserId:   userId,
		PlatformId: platformId,
	})

	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, errorx.Msg("项目不存在或无访问权限")
	}

	resp = &types.Project{}
	*resp = projectToType(result.Data[0])
	return resp, nil
}
