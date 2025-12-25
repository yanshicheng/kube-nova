package user

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/portalservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSysUserRolesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSysUserRolesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSysUserRolesLogic {
	return &GetSysUserRolesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSysUserRolesLogic) GetSysUserRoles(req *types.DefaultIdRequest) (resp *types.GetSysUserRolesResponse, err error) {

	// 这里需要实现获取用户角色的逻辑
	res, err := l.svcCtx.PortalRpc.UserSearchRole(l.ctx, &portalservice.SearchSysUserRoleReq{
		UserId: req.Id,
	})
	if err != nil {
		l.Errorf("获取用户角色列表失败: %v", err)
		return nil, err
	}
	return &types.GetSysUserRolesResponse{
		RoleIds:   res.RoleIds,
		RoleNames: res.RoleNames,
	}, nil
}
