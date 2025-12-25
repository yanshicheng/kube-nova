package menu

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DelSysMenuLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDelSysMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DelSysMenuLogic {
	return &DelSysMenuLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DelSysMenuLogic) DelSysMenu(req *types.DefaultIdRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务删除菜单
	_, err = l.svcCtx.PortalRpc.MenuDel(l.ctx, &pb.DelSysMenuReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("删除菜单失败: operator=%s, menuId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("删除菜单成功: operator=%s, menuId=%d", username, req.Id)
	return "删除菜单成功", nil
}
