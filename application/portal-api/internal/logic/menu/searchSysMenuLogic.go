package menu

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchSysMenuLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchSysMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchSysMenuLogic {
	return &SearchSysMenuLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchSysMenuLogic) SearchSysMenu(req *types.SearchSysMenuRequest) (resp *types.SearchSysMenuResponse, err error) {

	// 调用 RPC 服务搜索菜单树
	res, err := l.svcCtx.PortalRpc.MenuSearch(l.ctx, &pb.SearchSysMenuReq{
		ParentId: req.ParentId,
		MenuType: req.MenuType,
		Name:     req.Name,
		Title:    req.Title,
		Label:    req.Label,
		Status:   req.Status,
		IsMenu:   req.IsMenu,
	})
	if err != nil {
		l.Errorf("搜索菜单树失败: error=%v", err)
		return nil, err
	}

	// 转换菜单树
	var convertMenuTree func(*pb.SysMenuTree) types.SysMenuTree
	convertMenuTree = func(pbMenu *pb.SysMenuTree) types.SysMenuTree {
		menu := types.SysMenuTree{
			Id:            pbMenu.Id,
			ParentId:      pbMenu.ParentId,
			MenuType:      pbMenu.MenuType,
			Name:          pbMenu.Name,
			Path:          pbMenu.Path,
			Component:     pbMenu.Component,
			Redirect:      pbMenu.Redirect,
			Label:         pbMenu.Label,
			Title:         pbMenu.Title,
			Icon:          pbMenu.Icon,
			Sort:          pbMenu.Sort,
			Link:          pbMenu.Link,
			IsEnable:      pbMenu.IsEnable,
			IsMenu:        pbMenu.IsMenu,
			KeepAlive:     pbMenu.KeepAlive,
			IsHide:        pbMenu.IsHide,
			IsIframe:      pbMenu.IsIframe,
			IsHideTab:     pbMenu.IsHideTab,
			ShowBadge:     pbMenu.ShowBadge,
			ShowTextBadge: pbMenu.ShowTextBadge,
			IsFirstLevel:  pbMenu.IsFirstLevel,
			FixedTab:      pbMenu.FixedTab,
			IsFullPage:    pbMenu.IsFullPage,
			ActivePath:    pbMenu.ActivePath,
			Roles:         pbMenu.Roles,
			AuthName:      pbMenu.AuthName,
			AuthLabel:     pbMenu.AuthLabel,
			AuthIcon:      pbMenu.AuthIcon,
			AuthSort:      pbMenu.AuthSort,
			Status:        pbMenu.Status,
			CreatedAt:     pbMenu.CreateTime,
			UpdatedAt:     pbMenu.UpdateTime,
			CreatedBy:     pbMenu.CreateBy,
			UpdatedBy:     pbMenu.UpdateBy,
		}
		for _, child := range pbMenu.Children {
			menu.Children = append(menu.Children, convertMenuTree(child))
		}
		return menu
	}

	var menus []types.SysMenuTree
	for _, menu := range res.Data {
		menus = append(menus, convertMenuTree(menu))
	}

	return &types.SearchSysMenuResponse{
		Items: menus,
		Total: res.Total,
	}, nil
}
