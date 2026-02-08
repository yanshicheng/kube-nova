package portalservicelogic

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type MenuUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMenuUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MenuUpdateLogic {
	return &MenuUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// MenuUpdate 更新菜单
// MenuUpdate 更新菜单信息
func (l *MenuUpdateLogic) MenuUpdate(in *pb.UpdateSysMenuReq) (*pb.UpdateSysMenuResp, error) {

	// 参数验证
	if in.Id <= 0 {
		l.Errorf("更新菜单失败：菜单ID无效")
		return nil, errorx.Msg("菜单ID无效")
	}

	if in.PlatformId == 0 {
		l.Errorf("更新菜单失败：平台ID不能为空")
		return nil, errorx.Msg("平台ID不能为空")
	}

	if in.Name == "" {
		l.Errorf("更新菜单���败：菜单名称不能为空")
		return nil, errorx.Msg("菜单名称不能为空")
	}

	if in.Title == "" {
		l.Errorf("更新菜单失败：菜单标题不能为空")
		return nil, errorx.Msg("菜单标题不能为空")
	}

	// 验证菜单类型：1目录 2菜单 3按钮
	if in.MenuType < 1 || in.MenuType > 3 {
		l.Errorf("更新菜单失败：无效的菜单类型: %d", in.MenuType)
		return nil, errorx.Msg("无效的菜单类型，必须是1(目录)、2(菜单)或3(按钮)")
	}

	// 查询原菜单信息
	existingMenu, err := l.svcCtx.SysMenu.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("更新菜单失败：菜单不存在, menuId: %d", in.Id)
			return nil, errorx.Msg("菜单不存在")
		}
		l.Errorf("查询菜单信息失败: %v", err)
		return nil, errorx.Msg("查询菜单信息失败")
	}

	// 验证平台ID不允许修改
	if existingMenu.PlatformId != in.PlatformId {
		l.Errorf("更新菜单失败：不允许修改平台ID, menuId: %d, oldPlatformId: %d, newPlatformId: %d",
			in.Id, existingMenu.PlatformId, in.PlatformId)
		return nil, errorx.Msg("不允许修改平台ID")
	}

	// 防止将菜单设置为自己的子菜单（循环引用检查）
	if in.ParentId == in.Id {
		l.Errorf("更新菜单失败：不能将菜单设置为自己的父菜单, menuId: %d", in.Id)
		return nil, errorx.Msg("不能将菜单设置为自己的父菜单")
	}

	// 如果修改了父菜单，需要检查父菜单是否存在且属于同一平台
	if in.ParentId != existingMenu.ParentId && in.ParentId > 0 {
		parentMenu, err := l.svcCtx.SysMenu.FindOne(l.ctx, in.ParentId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("更新菜单失败：新的父菜单不存在, parentId: %d", in.ParentId)
				return nil, errorx.Msg("新的父菜单不存在")
			}
			l.Errorf("查询新的父菜单失败: %v", err)
			return nil, errorx.Msg("查询新的父菜单失败")
		}

		// 验证父菜单是否属于同一平台
		if parentMenu.PlatformId != in.PlatformId {
			l.Errorf("更新菜单失败：父菜单不属于同一平台, parentId: %d, parentPlatformId: %d, platformId: %d",
				in.ParentId, parentMenu.PlatformId, in.PlatformId)
			return nil, errorx.Msg("父菜单不属于同一平台")
		}

		// 检查循环引用
		if err := l.checkCircularReference(in.Id, in.ParentId); err != nil {
			l.Errorf("更新菜单失败：检测到循环引用, menuId: %d, parentId: %d", in.Id, in.ParentId)
			return nil, errorx.Msg("不能将菜单移动到其子菜单下，这会造成循环引用")
		}
	}

	// 检查同级菜单名称是否重复（排除自己，同一平台下）
	if in.Name != existingMenu.Name || in.ParentId != existingMenu.ParentId || in.MenuType != existingMenu.MenuType {
		var conditions []string
		var args []interface{}

		conditions = append(conditions, "`platform_id` = ? AND")
		args = append(args, in.PlatformId)

		conditions = append(conditions, "`parent_id` = ? AND")
		args = append(args, in.ParentId)

		conditions = append(conditions, "`name` = ? AND")
		args = append(args, in.Name)

		conditions = append(conditions, "`menu_type` = ? AND")
		args = append(args, in.MenuType)

		conditions = append(conditions, "`id` != ? AND")
		args = append(args, in.Id)

		query := utils.RemoveQueryADN(conditions)
		existingMenus, err := l.svcCtx.SysMenu.SearchNoPage(l.ctx, "", true, query, args...)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			l.Errorf("检查菜单名称重复失败: platformId=%d, error=%v", in.PlatformId, err)
			return nil, errorx.Msg("检查菜单名称重复失败")
		}
		if len(existingMenus) > 0 {
			l.Errorf("更新菜单失败：同级菜单名称已存在, name: %s, parentId: %d, platformId: %d", in.Name, in.ParentId, in.PlatformId)
			return nil, errorx.Msg("同级菜单名称已存在")
		}
	}

	// 如果是菜单类型，检查路由name是否重复（排除自己，同一平台下）
	if in.MenuType == 2 && in.Name != existingMenu.Name {
		routeConditions := []string{"`platform_id` = ? AND `name` = ? AND `menu_type` = ? AND `id` != ? AND"}
		routeArgs := []interface{}{in.PlatformId, in.Name, 2, in.Id}
		routeQuery := utils.RemoveQueryADN(routeConditions)

		existingRoutes, err := l.svcCtx.SysMenu.SearchNoPage(l.ctx, "", true, routeQuery, routeArgs...)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			l.Errorf("检查路由名称重复失败: platformId=%d, error=%v", in.PlatformId, err)
			return nil, errorx.Msg("检查路由名称重复失败")
		}
		if len(existingRoutes) > 0 {
			l.Errorf("更新菜单失败：路由名称已存在, name: %s, platformId: %d", in.Name, in.PlatformId)
			return nil, errorx.Msg("路由名称已存在")
		}
	}

	// 更新菜单信息
	updatedMenu := &model.SysMenu{
		Id:            in.Id,
		ParentId:      in.ParentId,
		PlatformId:    existingMenu.PlatformId, // 使用原有的平台ID，不允许修改
		MenuType:      in.MenuType,
		Name:          in.Name,
		Path:          in.Path,
		Component:     in.Component,
		Redirect:      in.Redirect,
		Label:         in.Label,
		Title:         in.Title,
		Icon:          in.Icon,
		Sort:          in.Sort,
		Link:          in.Link,
		IsEnable:      in.IsEnable,
		IsMenu:        in.IsMenu,
		KeepAlive:     in.KeepAlive,
		IsHide:        in.IsHide,
		IsIframe:      in.IsIframe,
		IsHideTab:     in.IsHideTab,
		ShowBadge:     in.ShowBadge,
		ShowTextBadge: in.ShowTextBadge,
		IsFirstLevel:  in.IsFirstLevel,
		FixedTab:      in.FixedTab,
		IsFullPage:    in.IsFullPage,
		ActivePath:    in.ActivePath,
		Roles:         sql.NullString{String: in.Roles, Valid: in.Roles != ""},
		AuthName:      in.AuthName,
		AuthLabel:     in.AuthLabel,
		AuthIcon:      in.AuthIcon,
		AuthSort:      in.AuthSort,
		Status:        in.Status,
		UpdateTime:    time.Now(),
		UpdateBy:      in.UpdateBy,
	}

	err = l.svcCtx.SysMenu.Update(l.ctx, updatedMenu)
	if err != nil {
		l.Errorf("更新菜单失败: platformId=%d, error=%v", existingMenu.PlatformId, err)
		return nil, errorx.Msg("更新菜单失败")
	}

	l.Infof("更新菜单成功，菜单ID: %d, 菜单名称: %s, 平台ID: %d, 更新人: %s",
		in.Id, in.Name, existingMenu.PlatformId, in.UpdateBy)
	return &pb.UpdateSysMenuResp{}, nil
}

// checkCircularReference 检查循环引用
// 检查 parentId 是否是 menuId 的子菜单
func (l *MenuUpdateLogic) checkCircularReference(menuId, parentId uint64) error {
	// 获取 parentId 的所有父级菜单，如果其中包含 menuId，则存在循环引用
	currentId := parentId
	visited := make(map[uint64]bool)

	for currentId > 0 {
		// 防止无限循环
		if visited[currentId] {
			break
		}
		visited[currentId] = true

		// 如果找到了目标菜单，说明存在循环引用
		if currentId == menuId {
			return errorx.Msg("循环引用")
		}

		// 查找当前菜单的父菜单
		menu, err := l.svcCtx.SysMenu.FindOne(l.ctx, currentId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				break
			}
			return err
		}
		currentId = menu.ParentId
	}

	return nil
}
