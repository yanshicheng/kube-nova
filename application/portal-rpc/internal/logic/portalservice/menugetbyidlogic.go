package portalservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type MenuGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMenuGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MenuGetByIdLogic {
	return &MenuGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// MenuGetById 根据ID获取菜单
// MenuGetById 根据ID获取菜单详细信息
func (l *MenuGetByIdLogic) MenuGetById(in *pb.GetSysMenuByIdReq) (*pb.GetSysMenuByIdResp, error) {

	// 参数验证
	if in.Id <= 0 {
		l.Errorf("查询菜单失败：菜单ID无效")
		return nil, errorx.Msg("菜单ID无效")
	}

	// 查询菜单信息
	menu, err := l.svcCtx.SysMenu.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("查询菜单失败：菜单不存在, menuId: %d", in.Id)
			return nil, errorx.Msg("菜单不存在")
		}
		l.Errorf("查询菜单信息失败: %v", err)
		return nil, errorx.Msg("查询菜单信息失败")
	}

	// 转换为响应格式
	pbMenu := l.convertToPbMenu(menu)

	return &pb.GetSysMenuByIdResp{
		Data: pbMenu,
	}, nil
}

// convertToPbMenu 将数据库模型转换为protobuf格式
func (l *MenuGetByIdLogic) convertToPbMenu(menu *model.SysMenu) *pb.SysMenu {
	return &pb.SysMenu{
		Id:            menu.Id,
		ParentId:      menu.ParentId,
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
		Roles:         menu.Roles.String,
		AuthName:      menu.AuthName,
		AuthLabel:     menu.AuthLabel,
		AuthIcon:      menu.AuthIcon,
		AuthSort:      menu.AuthSort,
		Status:        menu.Status,
		CreateTime:    menu.CreateTime.Unix(),
		UpdateTime:    menu.UpdateTime.Unix(),
		CreateBy:      menu.CreateBy,
		UpdateBy:      menu.UpdateBy,
	}
}
