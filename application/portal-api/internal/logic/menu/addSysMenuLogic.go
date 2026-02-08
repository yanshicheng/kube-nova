package menu

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddSysMenuLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddSysMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddSysMenuLogic {
	return &AddSysMenuLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddSysMenuLogic) AddSysMenu(req *types.AddSysMenuRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 验证 platformId 参数
	if req.PlatformId == 0 {
		l.Errorf("平台ID不能为空: operator=%s", username)
		return "", fmt.Errorf("平台ID不能为空")
	}

	// 调用 RPC 服务添加菜单
	_, err = l.svcCtx.PortalRpc.MenuAdd(l.ctx, &pb.AddSysMenuReq{
		ParentId:      req.ParentId,
		PlatformId:    req.PlatformId,
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
		CreateBy:      username,
		UpdateBy:      username,
	})
	if err != nil {
		l.Errorf("添加菜单失败: operator=%s, platformId=%d, menuName=%s, error=%v", username, req.PlatformId, req.Name, err)
		return "", err
	}

	l.Infof("添加菜单成功: operator=%s, platformId=%d, menuName=%s", username, req.PlatformId, req.Name)
	return "添加菜单成功", nil
}
