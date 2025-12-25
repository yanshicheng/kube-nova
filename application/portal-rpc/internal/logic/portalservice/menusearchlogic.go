package portalservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type MenuSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMenuSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MenuSearchLogic {
	return &MenuSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 搜索和列表操作
func (l *MenuSearchLogic) MenuSearch(in *pb.SearchSysMenuReq) (*pb.SearchSysMenuResp, error) {

	// 构建查询条件
	var conditions []string
	var args []interface{}

	// 菜单名称模糊查询
	if in.Name != "" {
		conditions = append(conditions, "`name` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.Name)+"%")
	}

	// 路由地址模糊查询
	if in.Path != "" {
		conditions = append(conditions, "`path` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.Path)+"%")
	}

	// 组件路径模糊查询
	if in.Component != "" {
		conditions = append(conditions, "`component` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.Component)+"%")
	}

	// 权限标识模糊查询
	if in.Label != "" {
		conditions = append(conditions, "`label` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.Label)+"%")
	}

	// 菜单标题模糊查询
	if in.Title != "" {
		conditions = append(conditions, "`title` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.Title)+"%")
	}

	// 创建人模糊查询
	if in.CreateBy != "" {
		conditions = append(conditions, "`create_by` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.CreateBy)+"%")
	}

	// 更新人模糊查询
	if in.UpdateBy != "" {
		conditions = append(conditions, "`update_by` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.UpdateBy)+"%")
	}

	// 去掉最后一个 " AND "，避免 SQL 语法错误
	query := utils.RemoveQueryADN(conditions)

	// 查询所有符合条件的菜单数据，按sort字段升序排列
	allMenus, err := l.svcCtx.SysMenu.SearchNoPage(l.ctx, "sort", true, query, args...)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询菜单列表失败: %v", err)
		return nil, errorx.Msg("查询菜单列表失败")
	}

	// 如果没有数据，返回空结果
	if len(allMenus) == 0 {
		l.Infof("搜索菜单树状结构完成，未找到任何数据")
		return &pb.SearchSysMenuResp{
			Data:  []*pb.SysMenuTree{},
			Total: 0,
		}, nil
	}

	// 转换为protobuf格式
	pbMenus := l.convertToPbMenuTreeList(allMenus)

	// 构建树状结构
	menuTree := l.buildMenuTree(pbMenus)

	return &pb.SearchSysMenuResp{
		Data:  menuTree,
		Total: uint64(len(allMenus)),
	}, nil
}

// convertToPbMenuTreeList 将数据库模型列表转换为protobuf格式列表
func (l *MenuSearchLogic) convertToPbMenuTreeList(menus []*model.SysMenu) []*pb.SysMenuTree {
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
func (l *MenuSearchLogic) buildMenuTree(menus []*pb.SysMenuTree) []*pb.SysMenuTree {
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
func (l *MenuSearchLogic) sortMenuTree(menus []*pb.SysMenuTree) {
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
