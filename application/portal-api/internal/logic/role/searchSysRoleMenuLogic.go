package role

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchSysRoleMenuLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchSysRoleMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchSysRoleMenuLogic {
	return &SearchSysRoleMenuLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchSysRoleMenuLogic) SearchSysRoleMenu(req *types.SearchSysRoleMenuRequest) (resp []uint64, err error) {

	// 调用 RPC 服务查询角色菜单
	res, err := l.svcCtx.PortalRpc.RoleSearchMenu(l.ctx, &pb.SearchSysRoleMenuReq{
		RoleId: req.RoleId,
	})
	if err != nil {
		l.Errorf("查询角色菜单失败: roleId=%d, error=%v", req.RoleId, err)
		return nil, err
	}

	return res.MenuIds, nil
}
