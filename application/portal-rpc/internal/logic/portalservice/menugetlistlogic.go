package portalservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/vars"
	"github.com/yanshicheng/kube-nova/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type MenuGetListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMenuGetListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MenuGetListLogic {
	return &MenuGetListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// MenuGetList 获取菜单列表（扁平结构，支持分页）
func (l *MenuGetListLogic) MenuGetList(in *pb.GetSysMenuListReq) (*pb.GetSysMenuListResp, error) {

	// 参数验证
	if in.PlatformId == 0 {
		l.Errorf("获取菜单列表失败：平台ID不能为空")
		return nil, errorx.Msg("平台ID不能为空")
	}

	// 设置默认参数
	if in.Page <= 0 {
		in.Page = vars.Page
	}
	if in.PageSize <= 0 {
		in.PageSize = vars.PageSize
	}
	if in.OrderField == "" {
		in.OrderField = vars.OrderField
	}

	// 构建查询条件
	var conditions []string
	var args []interface{}

	// 平台ID条件（必须）
	conditions = append(conditions, "`platform_id` = ? AND")
	args = append(args, in.PlatformId)

	// 父菜单ID条件
	if in.ParentId > 0 {
		conditions = append(conditions, "`parent_id` = ? AND")
		args = append(args, in.ParentId)
	}

	// 菜单类型条件
	if in.MenuType > 0 && in.MenuType <= 3 {
		conditions = append(conditions, "`menu_type` = ? AND")
		args = append(args, in.MenuType)
	}

	// 菜单名称模糊查询
	if in.Name != "" {
		conditions = append(conditions, "`name` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.Name)+"%")
	}

	// 菜单标题模糊查询
	if in.Title != "" {
		conditions = append(conditions, "`title` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.Title)+"%")
	}

	// 权限标识模糊查询
	if in.Label != "" {
		conditions = append(conditions, "`label` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.Label)+"%")
	}

	// 状态条件
	if in.Status >= 0 && in.Status <= 1 {
		conditions = append(conditions, "`status` = ? AND")
		args = append(args, in.Status)
	}

	// 角色权限模糊查询
	if in.Roles != "" {
		conditions = append(conditions, "`roles` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.Roles)+"%")
	}

	// 是否为菜单条件
	if in.IsMenu >= 0 && in.IsMenu <= 1 {
		conditions = append(conditions, "`is_menu` = ? AND")
		args = append(args, in.IsMenu)
	}

	// 去掉最后一个 " AND "，避免 SQL 语法错误
	query := utils.RemoveQueryADN(conditions)

	// 执行分页查询
	menuList, total, err := l.svcCtx.SysMenu.Search(l.ctx, in.OrderField, in.IsAsc, in.Page, in.PageSize, query, args...)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询菜单列表失败: platformId=%d, error=%v", in.PlatformId, err)
		return nil, errorx.Msg("查询菜单列表失败")
	}

	// 转换结果
	pbMenus := l.convertToMenuList(menuList)

	return &pb.GetSysMenuListResp{
		Data:     pbMenus,
		Total:    total,
		Page:     in.Page,
		PageSize: in.PageSize,
	}, nil
}

// convertToMenuList 转换菜单列表
func (l *MenuGetListLogic) convertToMenuList(menuList []*model.SysMenu) []*pb.SysMenu {
	var pbMenus []*pb.SysMenu
	for _, menu := range menuList {
		pbMenu := &pb.SysMenu{
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
		}
		pbMenus = append(pbMenus, pbMenu)
	}
	return pbMenus
}
