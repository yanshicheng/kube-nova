package role

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchSysRoleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchSysRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchSysRoleLogic {
	return &SearchSysRoleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchSysRoleLogic) SearchSysRole(req *types.SearchSysRoleRequest) (resp *types.SearchSysRoleResponse, err error) {

	// 调用 RPC 服务搜索角色
	res, err := l.svcCtx.PortalRpc.RoleSearch(l.ctx, &pb.SearchSysRoleReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		OrderField: req.OrderStr,
		IsAsc:      req.IsAsc,
		Name:       req.Name,
		Code:       req.Code,
	})
	if err != nil {
		l.Errorf("搜索角色失败: error=%v", err)
		return nil, err
	}

	// 转换角色列表
	var roles []types.SysRole
	for _, role := range res.Data {
		roles = append(roles, types.SysRole{
			Id:        role.Id,
			Name:      role.Name,
			Code:      role.Code,
			Remark:    role.Remark,
			CreatedBy: role.CreatedBy,
			UpdatedBy: role.UpdatedBy,
			CreatedAt: role.CreatedAt,
			UpdatedAt: role.UpdatedAt,
		})
	}

	return &types.SearchSysRoleResponse{
		Items: roles,
		Total: res.Total,
	}, nil
}
