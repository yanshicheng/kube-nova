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

type MenuGetAllMenuTreeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMenuGetAllMenuTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MenuGetAllMenuTreeLogic {
	return &MenuGetAllMenuTreeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// MenuGetAllMenuTree 获取所有菜单树
func (l *MenuGetAllMenuTreeLogic) MenuGetAllMenuTree(in *pb.GetAllMenuTreeReq) (*pb.GetAllMenuTreeResp, error) {
	// 构建查询条件
	var query string
	var args []interface{}

	// 根据状态参数过滤
	// status = 0: 只查询停用的菜单
	// status = 1: 只查询启用的菜单
	// status = -1 或不传: 查询所有菜单
	if in.Status == 0 || in.Status == 1 {
		query = "`status` = ?"
		args = append(args, in.Status)
	} else {
		// 查询所有菜单（不过滤状态）
		query = ""
		args = nil
	}

	// 查询所有符合条件的菜单数据，按sort字段升序排列
	allMenus, err := l.svcCtx.SysMenu.SearchNoPage(l.ctx, "sort", true, query, args...)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询所有菜单失败: %v", err)
		return nil, errorx.Msg("查询所有菜单失败")
	}

	// 如果没有数据，返回空结果
	if len(allMenus) == 0 {
		l.Infof("获取所有菜单树完成，未找到任何菜单数据")
		return &pb.GetAllMenuTreeResp{
			Data: []*pb.SysMenuTree{},
		}, nil
	}

	// 转换为protobuf格式
	pbMenus := l.convertToPbMenuTreeList(allMenus)

	// 构建树状结构
	menuTree := l.buildMenuTree(pbMenus)

	return &pb.GetAllMenuTreeResp{
		Data: menuTree,
	}, nil
}

// convertToPbMenuTreeList 将数据库模型列表转换为protobuf格式列表
func (l *MenuGetAllMenuTreeLogic) convertToPbMenuTreeList(menus []*model.SysMenu) []*pb.SysMenuTree {
	var pbMenus []*pb.SysMenuTree
	for _, menu := range menus {
		pbMenu := &pb.SysMenuTree{
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
			Children:      []*pb.SysMenuTree{}, // 初始化子节点切片
		}
		pbMenus = append(pbMenus, pbMenu)
	}
	return pbMenus
}

// buildMenuTree 构建菜单树状结构
func (l *MenuGetAllMenuTreeLogic) buildMenuTree(menus []*pb.SysMenuTree) []*pb.SysMenuTree {
	// 创建ID到菜单的映射，便于快速查找
	menuMap := make(map[uint64]*pb.SysMenuTree)
	var rootMenus []*pb.SysMenuTree

	// 第一步：建立ID映射
	for _, menu := range menus {
		menuMap[menu.Id] = menu
	}

	// 第二步：构建父子关系
	for _, menu := range menus {
		if menu.ParentId == 0 {
			// 顶级菜单
			rootMenus = append(rootMenus, menu)
		} else {
			// 子菜单，找到其父菜单并加入到父菜单的children中
			if parent, exists := menuMap[menu.ParentId]; exists {
				parent.Children = append(parent.Children, menu)
			} else {
				// 父菜单不存在，可能数据有问题，将其作为顶级菜单处理
				l.Errorf("菜单 %d (名称: %s) 的父菜单 %d 不存在，将其作为顶级菜单处理",
					menu.Id, menu.Name, menu.ParentId)
				menu.ParentId = 0 // 重置父ID
				rootMenus = append(rootMenus, menu)
			}
		}
	}

	// 第三步：对每个层级的菜单按sort字段排序
	l.sortMenuTree(rootMenus)

	return rootMenus
}

// sortMenuTree 递归排序菜单树（按sort字段升序）
func (l *MenuGetAllMenuTreeLogic) sortMenuTree(menus []*pb.SysMenuTree) {
	if len(menus) <= 1 {
		return
	}

	// 使用冒泡排序按sort字段排序
	for i := 0; i < len(menus)-1; i++ {
		for j := 0; j < len(menus)-1-i; j++ {
			if menus[j].Sort > menus[j+1].Sort {
				menus[j], menus[j+1] = menus[j+1], menus[j]
			}
		}
	}

	// 递归排序子节点
	for _, menu := range menus {
		if len(menu.Children) > 0 {
			l.sortMenuTree(menu.Children)
		}
	}
}
