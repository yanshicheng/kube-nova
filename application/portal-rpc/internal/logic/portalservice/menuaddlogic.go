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

type MenuAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMenuAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MenuAddLogic {
	return &MenuAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -----------------------系统菜单表-----------------------
func (l *MenuAddLogic) MenuAdd(in *pb.AddSysMenuReq) (*pb.AddSysMenuResp, error) {
	// 参数验证
	if in.PlatformId == 0 {
		l.Errorf("添加菜单失败：平台ID不能为空")
		return nil, errorx.Msg("平台ID不能为空")
	}

	if in.Name == "" {
		l.Errorf("添加菜单失败：菜单名称不能为空")
		return nil, errorx.Msg("菜单名称不能为空")
	}

	if in.Title == "" {
		l.Errorf("添加菜单失败：菜单标题不能为空")
		return nil, errorx.Msg("菜单标题不能为空")
	}

	// 验证菜单类型：1目录 2菜单 3按钮
	if in.MenuType < 1 || in.MenuType > 3 {
		l.Errorf("添加菜单失败：无效的菜单类型: %d", in.MenuType)
		return nil, errorx.Msg("无效的菜单类型，必须是1(目录)、2(菜单)或3(按钮)")
	}

	// 如果有父菜单，验证父菜单是否存在且属于同一平台
	if in.ParentId > 0 {
		parentMenu, err := l.svcCtx.SysMenu.FindOne(l.ctx, in.ParentId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("添加菜单失败：父菜单不存在, parentId: %d", in.ParentId)
				return nil, errorx.Msg("父菜单不存在")
			}
			l.Errorf("查询父菜单失败: %v", err)
			return nil, errorx.Msg("查询父菜单失败")
		}
		// 验证父菜单是否属于同一平台
		if parentMenu.PlatformId != in.PlatformId {
			l.Errorf("添加菜单失败：父菜单不属于同一平台, parentId: %d, parentPlatformId: %d, platformId: %d",
				in.ParentId, parentMenu.PlatformId, in.PlatformId)
			return nil, errorx.Msg("父菜单不属于同一平台")
		}
	}

	// 检查同级菜单名称是否重复（同一平台下）
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

	query := utils.RemoveQueryADN(conditions)
	existingMenus, err := l.svcCtx.SysMenu.SearchNoPage(l.ctx, "", true, query, args...)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("检查菜单名称重复失败: platformId=%d, error=%v", in.PlatformId, err)
		return nil, errorx.Msg("检查菜单名称重复失败")
	}
	if len(existingMenus) > 0 {
		l.Errorf("添加菜单失败：同级菜单名称已存在, name: %s, parentId: %d, platformId: %d", in.Name, in.ParentId, in.PlatformId)
		return nil, errorx.Msg("同级菜单名称已存在")
	}

	// 如果是菜单类型，检查路由name是否重复（同一平台下）
	if in.MenuType == 2 && in.Name != "" {
		routeConditions := []string{"`platform_id` = ? AND `name` = ? AND `menu_type` = ? AND"}
		routeArgs := []interface{}{in.PlatformId, in.Name, 2}
		routeQuery := utils.RemoveQueryADN(routeConditions)

		existingRoutes, err := l.svcCtx.SysMenu.SearchNoPage(l.ctx, "", true, routeQuery, routeArgs...)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			l.Errorf("检查路由名称重复失败: platformId=%d, error=%v", in.PlatformId, err)
			return nil, errorx.Msg("检查路由名称重复失败")
		}
		if len(existingRoutes) > 0 {
			l.Errorf("添加菜单失败：路由名称已存在, name: %s, platformId: %d", in.Name, in.PlatformId)
			return nil, errorx.Msg("路由名称已存在")
		}
	}

	// 构造菜单数据
	now := time.Now()
	menu := &model.SysMenu{
		ParentId:      in.ParentId,
		PlatformId:    in.PlatformId,
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
		CreateTime:    now,
		UpdateTime:    now,
		CreateBy:      in.CreateBy,
		UpdateBy:      in.UpdateBy,
	}

	// 保存到数据库
	_, err = l.svcCtx.SysMenu.Insert(l.ctx, menu)
	if err != nil {
		l.Errorf("添加菜单失败: platformId=%d, error=%v", in.PlatformId, err)
		return nil, errorx.Msg("添加菜单失败")
	}

	l.Infof("添加菜单成功，菜单名称: %s, 菜单类型: %d, 平台ID: %d, 创建人: %s",
		in.Name, in.MenuType, in.PlatformId, in.CreateBy)
	return &pb.AddSysMenuResp{}, nil
}
