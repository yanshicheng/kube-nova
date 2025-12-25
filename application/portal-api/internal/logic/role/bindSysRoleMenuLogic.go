package role

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type BindSysRoleMenuLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBindSysRoleMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindSysRoleMenuLogic {
	return &BindSysRoleMenuLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// role/bindSysRoleMenuLogic.go
func (l *BindSysRoleMenuLogic) BindSysRoleMenu(req *types.BindSysRoleMenuRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务绑定角色菜单
	_, err = l.svcCtx.PortalRpc.RoleAddMenu(l.ctx, &pb.AddSysRoleMenuReq{
		RoleId:  req.RoleId,
		MenuIds: req.MenuIds,
	})
	if err != nil {
		l.Errorf("角色绑定菜单失败: operator=%s, roleId=%d, error=%v", username, req.RoleId, err)
		return "", err
	}

	l.Infof("角色绑定菜单成功: operator=%s, roleId=%d", username, req.RoleId)
	return "角色绑定菜单成功", nil
}
