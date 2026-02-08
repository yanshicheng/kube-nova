package menu

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSysMenuListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSysMenuListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSysMenuListLogic {
	return &GetSysMenuListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSysMenuListLogic) GetSysMenuList(req *types.GetSysMenuListRequest) (resp *types.GetSysMenuListResponse, err error) {

	// 验证 platformId 参数
	if req.PlatformId == 0 {
		l.Errorf("平台ID不能为空")
		return nil, fmt.Errorf("平台ID不能为空")
	}

	// 调用 RPC 服务获取菜单列表
	res, err := l.svcCtx.PortalRpc.MenuGetList(l.ctx, &pb.GetSysMenuListReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		OrderField: req.OrderStr,
		IsAsc:      req.IsAsc,
		PlatformId: req.PlatformId,
		ParentId:   req.ParentId,
		MenuType:   req.MenuType,
		Name:       req.Name,
		Title:      req.Title,
		Label:      req.Label,
		Status:     req.Status,
		IsMenu:     req.IsMenu,
	})
	if err != nil {
		l.Errorf("获取菜单列表失败: platformId=%d, error=%v", req.PlatformId, err)
		return nil, err
	}

	// 转换菜单列表
	var menus []types.SysMenu
	for _, menu := range res.Data {
		menus = append(menus, types.SysMenu{
			Id:            menu.Id,
			ParentId:      menu.ParentId,
			PlatformId:    menu.PlatformId,
			MenuType:      menu.MenuType,
			Name:          menu.Name,
			Path:          menu.Path,
			Component:     menu.Component,
			Redirect:      menu.Redirect,
			Label:         menu.Label,
			Title:         menu.Title,
			Icon:          menu.Icon,
			Sort:          menu.Sort,
			Link:          menu.Link,
			IsEnable:      menu.IsEnable,
			IsMenu:        menu.IsMenu,
			KeepAlive:     menu.KeepAlive,
			IsHide:        menu.IsHide,
			IsIframe:      menu.IsIframe,
			IsHideTab:     menu.IsHideTab,
			ShowBadge:     menu.ShowBadge,
			ShowTextBadge: menu.ShowTextBadge,
			IsFirstLevel:  menu.IsFirstLevel,
			FixedTab:      menu.FixedTab,
			IsFullPage:    menu.IsFullPage,
			ActivePath:    menu.ActivePath,
			Roles:         menu.Roles,
			AuthName:      menu.AuthName,
			AuthLabel:     menu.AuthLabel,
			AuthIcon:      menu.AuthIcon,
			AuthSort:      menu.AuthSort,
			Status:        menu.Status,
			CreatedAt:     menu.CreateTime,
			UpdatedAt:     menu.UpdateTime,
			CreatedBy:     menu.CreateBy,
			UpdatedBy:     menu.UpdateBy,
		})
	}

	return &types.GetSysMenuListResponse{
		Items: menus,
		Total: res.Total,
	}, nil
}
