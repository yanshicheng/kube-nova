package menu

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSysMenuLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSysMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSysMenuLogic {
	return &UpdateSysMenuLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSysMenuLogic) UpdateSysMenu(req *types.UpdateSysMenuRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务更新菜单
	_, err = l.svcCtx.PortalRpc.MenuUpdate(l.ctx, &pb.UpdateSysMenuReq{
		Id:            req.Id,
		ParentId:      req.ParentId,
		MenuType:      req.MenuType,
		Name:          req.Name,
		Path:          req.Path,
		Component:     req.Component,
		Redirect:      req.Redirect,
		Label:         req.Label,
		Title:         req.Title,
		Icon:          req.Icon,
		Sort:          req.Sort,
		Link:          req.Link,
		IsEnable:      req.IsEnable,
		IsMenu:        req.IsMenu,
		KeepAlive:     req.KeepAlive,
		IsHide:        req.IsHide,
		IsIframe:      req.IsIframe,
		IsHideTab:     req.IsHideTab,
		ShowBadge:     req.ShowBadge,
		ShowTextBadge: req.ShowTextBadge,
		IsFirstLevel:  req.IsFirstLevel,
		FixedTab:      req.FixedTab,
		IsFullPage:    req.IsFullPage,
		ActivePath:    req.ActivePath,
		Roles:         req.Roles,
		AuthName:      req.AuthName,
		AuthLabel:     req.AuthLabel,
		AuthIcon:      req.AuthIcon,
		AuthSort:      req.AuthSort,
		Status:        req.Status,
		UpdateBy:      username,
	})
	if err != nil {
		l.Errorf("更新菜单失败: operator=%s, menuId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("更新菜单成功: operator=%s, menuId=%d", username, req.Id)
	return "更新菜单成功", nil
}
