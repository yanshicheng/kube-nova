package portalservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type MenuGetRolesMenuTreeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMenuGetRolesMenuTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MenuGetRolesMenuTreeLogic {
	return &MenuGetRolesMenuTreeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// MenuGetRolesMenuTree 根据角色编码获取菜单树状结构
func (l *MenuGetRolesMenuTreeLogic) MenuGetRolesMenuTree(in *pb.GetRolesMenuTreeReq) (*pb.GetRolesMenuTreeResp, error) {
	// 参数验证
	if in.PlatformId == 0 {
		l.Errorf("获取角色菜单树失败：平台ID不能为空")
		return nil, errorx.Msg("平台ID不能为空")
	}

	if len(in.RoleCodes) == 0 {
		l.Errorf("获取角色菜单树失败：角色编码列表不能为空")
		return nil, errorx.Msg("角色编码列表不能为空")
	}

	var menus []*model.SysMenu
	var err error

	// 检查是否包含超级管理员角色
	if l.isSuperAdmin(in.RoleCodes) {
		l.Infof("检测到超级管理员角色，返回所有启用的菜单, platformId=%d", in.PlatformId)
		// 如果是超级管理员，获取所有启用的菜单（指定平台）
		menus, err = l.getAllEnabledMenus(in.PlatformId)
		if err != nil {
			l.Errorf("获取所有启用菜单失败: platformId=%d, error=%v", in.PlatformId, err)
			return nil, errorx.Msg("获取所有启用菜单失败")
		}
	} else {
		// 普通角色，按照角色权限获取菜单（指定平台）
		menus, err = l.getMenusByRoleCodes(in.RoleCodes, in.PlatformId)
		if err != nil {
			l.Errorf("根据角色获取菜单失败: platformId=%d, error=%v", in.PlatformId, err)
			return nil, errorx.Msg("根据角色获取菜单失败")
		}
	}

	// 过滤启用状态且符合菜单类型的菜单（只返回 menu_type = 1 或 2 的菜单）
	filteredMenus := l.filterEnabledAndValidTypeMenus(menus)

	if len(filteredMenus) == 0 {
		l.Infof("未找到符合条件的菜单，返回空菜单树")
		return &pb.GetRolesMenuTreeResp{
			Data: []*pb.SysMenuTree{},
		}, nil
	}

	// 转换为protobuf格式
	pbMenus := l.convertToPbMenuTreeList(filteredMenus)

	// 构建树状结构
	menuTree := l.buildMenuTree(pbMenus)

	return &pb.GetRolesMenuTreeResp{
		Data: menuTree,
	}, nil
}

// isSuperAdmin 检查角色编码列表中是否包含超级管理员
func (l *MenuGetRolesMenuTreeLogic) isSuperAdmin(roleCodes []string) bool {
	for _, code := range roleCodes {
		// 转换为大写进行比较，确保大小写不敏感
		if strings.ToUpper(strings.TrimSpace(code)) == "SUPER_ADMIN" {
			return true
		}
	}
	return false
}

// getAllEnabledMenus 获取所有启用的菜单（超级管理员使用，指定平台）
func (l *MenuGetRolesMenuTreeLogic) getAllEnabledMenus(platformId uint64) ([]*model.SysMenu, error) {
	// 查询所有启用状态(status=1)且可见(is_enable=1)的菜单（指定平台）
	// 这里使用SearchNoPage方法，按sort字段升序排序
	menus, err := l.svcCtx.SysMenu.SearchNoPage(l.ctx, "sort", true, "`platform_id` = ? AND `status` = ? AND `is_enable` = ?", platformId, 1, 1)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Infof("系统中没有启用的菜单, platformId=%d", platformId)
			return []*model.SysMenu{}, nil
		}
		return nil, err
	}

	return menus, nil
}

// getMenusByRoleCodes 根据角色编码获取菜单（普通角色使用，指定平台）
func (l *MenuGetRolesMenuTreeLogic) getMenusByRoleCodes(roleCodes []string, platformId uint64) ([]*model.SysMenu, error) {
	// 根据角色编码获取角色ID列表
	roleIds, err := l.getRoleIdsByCodes(roleCodes)
	if err != nil {
		l.Errorf("根据角色编码获取角色ID失败: %v", err)
		return nil, err
	}

	if len(roleIds) == 0 {
		l.Errorf("获取角色菜单树失败：未找到对应的角色")
		return nil, errors.New("未找到对应的角色")
	}

	// 获取角色关联的菜单ID列表
	menuIds, err := l.getMenuIdsByRoleIds(roleIds)
	if err != nil {
		l.Errorf("根据角色ID获取菜单ID失败: %v", err)
		return nil, err
	}

	if len(menuIds) == 0 {
		l.Infof("角色未关联任何菜单，返回空列表")
		return []*model.SysMenu{}, nil
	}

	// 根据菜单ID列表获取菜单详情（过滤指定平台）
	menus, err := l.getMenusByIds(menuIds, platformId)
	if err != nil {
		l.Errorf("根据菜单ID获取菜单详情失败: platformId=%d, error=%v", platformId, err)
		return nil, err
	}

	return menus, nil
}

// getRoleIdsByCodes 根据角色编码获取角色ID列表
func (l *MenuGetRolesMenuTreeLogic) getRoleIdsByCodes(roleCodes []string) ([]uint64, error) {
	var roleIds []uint64

	for _, code := range roleCodes {
		trimmedCode := strings.TrimSpace(code)
		if trimmedCode == "" {
			continue
		}

		role, err := l.svcCtx.SysRole.FindOneByCode(l.ctx, trimmedCode)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("角色编码不存在: %s", code)
				continue
			}
			return nil, err
		}
		roleIds = append(roleIds, role.Id)
	}

	return roleIds, nil
}

// getMenuIdsByRoleIds 根据角色ID获取菜单ID列表
func (l *MenuGetRolesMenuTreeLogic) getMenuIdsByRoleIds(roleIds []uint64) ([]uint64, error) {
	var menuIds []uint64
	menuIdMap := make(map[uint64]bool) // 用于去重

	for _, roleId := range roleIds {
		roleMenus, err := l.svcCtx.SysRoleMenu.SearchNoPage(l.ctx, "", true, "`role_id` = ?", roleId)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			return nil, err
		}

		// 遍历角色菜单关联表，收集菜单ID
		for _, roleMenu := range roleMenus {
			if !menuIdMap[roleMenu.MenuId] {
				menuIdMap[roleMenu.MenuId] = true
				menuIds = append(menuIds, roleMenu.MenuId)
			}
		}
	}

	return menuIds, nil
}

// getMenusByIds 根据菜单ID列表获取菜单详情（过滤指定平台）
func (l *MenuGetRolesMenuTreeLogic) getMenusByIds(menuIds []uint64, platformId uint64) ([]*model.SysMenu, error) {
	var menus []*model.SysMenu

	for _, menuId := range menuIds {
		menu, err := l.svcCtx.SysMenu.FindOne(l.ctx, menuId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("菜单不存在: %d", menuId)
				continue
			}
			return nil, err
		}
		// 只返回指定平台的菜单
		if menu.PlatformId == platformId {
			menus = append(menus, menu)
		}
	}

	return menus, nil
}

// filterEnabledAndValidTypeMenus 过滤启用状态且菜单类型符合要求的菜单
// 只返回启用状态(status=1)、可见(is_enable=1)且菜单类型为1或2的菜单
// menu_type: 1=目录菜单, 2=页面菜单, 3=按钮权限(不返回)
func (l *MenuGetRolesMenuTreeLogic) filterEnabledAndValidTypeMenus(menus []*model.SysMenu) []*model.SysMenu {
	var filteredMenus []*model.SysMenu

	for _, menu := range menus {
		// 只返回启用状态(status=1)且可见(is_enable=1)的菜单
		if menu.Status == 1 && menu.IsEnable == 1 {
			// 只返回菜单类型为1(目录菜单)或2(页面菜单)的数据
			// 过滤掉菜单类型为3(按钮权限)的数据
			if menu.MenuType == 1 || menu.MenuType == 2 {
				filteredMenus = append(filteredMenus, menu)
			} else {
				l.Debugf("过滤掉菜单类型不符合要求的菜单: %s (类型: %d)", menu.Name, menu.MenuType)
			}
		} else {
			l.Debugf("过滤掉未启用的菜单: %s (status: %d, is_enable: %d)", menu.Name, menu.Status, menu.IsEnable)
		}
	}

	return filteredMenus
}

// convertToPbMenuTreeList 将数据库模型列表转换为protobuf格式列表
func (l *MenuGetRolesMenuTreeLogic) convertToPbMenuTreeList(menus []*model.SysMenu) []*pb.SysMenuTree {
	var pbMenus []*pb.SysMenuTree
	for _, menu := range menus {
		pbMenu := &pb.SysMenuTree{
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
func (l *MenuGetRolesMenuTreeLogic) buildMenuTree(menus []*pb.SysMenuTree) []*pb.SysMenuTree {
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
			// 顶级菜单（父ID为0的菜单）
			rootMenus = append(rootMenus, menu)
		} else {
			// 子菜单，找到其父菜单并加入到父菜单的children中
			if parent, exists := menuMap[menu.ParentId]; exists {
				parent.Children = append(parent.Children, menu)
			} else {
				// 父菜单不存在或未授权，将其作为顶级菜单处理
				l.Errorf("菜单 %d (名称: %s) 的父菜单 %d 不存在或未授权，将其作为顶级菜单处理",
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
func (l *MenuGetRolesMenuTreeLogic) sortMenuTree(menus []*pb.SysMenuTree) {
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
