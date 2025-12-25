package menu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRolesMenuTreeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetRolesMenuTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRolesMenuTreeLogic {
	return &GetRolesMenuTreeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetRolesMenuTreeLogic) GetRolesMenuTree() (resp []types.AppRouteRecord, err error) {
	// 从上下文中获取用户ID
	userId, err := l.getUserIdFromContext()
	if err != nil {
		l.Errorf("获取用户ID失败: error=%v", err)
		return nil, fmt.Errorf("获取用户信息失败")
	}

	// 从上下文中获取角色列表
	roleCodes, err := l.getRolesFromContext()
	if err != nil {
		l.Errorf("获取用户角色失败: userId=%d, error=%v", userId, err)
		return nil, fmt.Errorf("获取用户角色失败")
	}

	// 如果用户没有任何角色，返回空菜单
	if len(roleCodes) == 0 {
		l.Infof("用户无任何角色: userId=%d", userId)
		return []types.AppRouteRecord{}, nil
	}

	l.Infof("用户角色列表: userId=%d, roleCodes=%v", userId, roleCodes)

	// 调用 RPC 服务获取角色菜单树
	res, err := l.svcCtx.PortalRpc.MenuGetRolesMenuTree(l.ctx, &pb.GetRolesMenuTreeReq{
		RoleCodes: roleCodes,
	})
	if err != nil {
		l.Errorf("获取角色菜单树失败: roleCodes=%v, error=%v", roleCodes, err)
		return nil, fmt.Errorf("获取菜单数据失败")
	}

	// 转换菜单树为前端路由格式
	var routes []types.AppRouteRecord
	for _, menu := range res.Data {
		route := l.convertToAppRouteRecord(menu, "")
		routes = append(routes, route)
	}

	l.Infof("菜单树转换成功: userId=%d, 菜单数量=%d", userId, len(routes))
	return routes, nil
}

// getUserIdFromContext 从上下文中获取用户ID
func (l *GetRolesMenuTreeLogic) getUserIdFromContext() (int64, error) {
	userIdVal := l.ctx.Value("userId")
	if userIdVal == nil {
		return 0, fmt.Errorf("userId not found in context")
	}

	// 类型断言
	switch v := userIdVal.(type) {
	case int64:
		return v, nil
	case uint64:
		return int64(v), nil

	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		var userId int64
		_, err := fmt.Sscanf(v, "%d", &userId)
		if err != nil {
			return 0, fmt.Errorf("invalid userId format: %v", err)
		}
		return userId, nil
	default:
		return 0, fmt.Errorf("unsupported userId type: %T", v)
	}
}

// getRolesFromContext 从上下文中获取角色列表
func (l *GetRolesMenuTreeLogic) getRolesFromContext() ([]string, error) {
	rolesVal := l.ctx.Value("roles")
	if rolesVal == nil {
		return nil, fmt.Errorf("roles not found in context")
	}

	// 类型断言 - 根据你的 JWT Claims 结构，可能是以下几种类型之一
	switch v := rolesVal.(type) {
	case []string:
		// 最理想的情况：直接是字符串数组
		return v, nil

	case []interface{}:
		// interface{} 数组，需要转换为字符串数组
		var roles []string
		for _, item := range v {
			if str, ok := item.(string); ok {
				roles = append(roles, str)
			}
		}
		return roles, nil

	case string:
		// 如果是逗号分隔的字符串
		if v == "" {
			return []string{}, nil
		}
		roles := strings.Split(v, ",")
		for i := range roles {
			roles[i] = strings.TrimSpace(roles[i])
		}
		return roles, nil

	case json.RawMessage:
		// 如果是 JSON 字符串
		var roles []string
		if err := json.Unmarshal(v, &roles); err != nil {
			return nil, fmt.Errorf("failed to unmarshal roles: %v", err)
		}
		return roles, nil

	default:
		return nil, fmt.Errorf("unsupported roles type: %T", v)
	}
}

// convertToAppRouteRecord 将 pb.SysMenuTree 转换为 types.AppRouteRecord
func (l *GetRolesMenuTreeLogic) convertToAppRouteRecord(pbMenu *pb.SysMenuTree, parentPath string) types.AppRouteRecord {
	// 解析角色权限字符串为数组
	var roles []string
	if pbMenu.Roles != "" {
		if err := json.Unmarshal([]byte(pbMenu.Roles), &roles); err != nil {
			l.Errorf("解析角色权限失败: roles=%s, error=%v", pbMenu.Roles, err)
			roles = []string{}
		}
	}

	// 构建权限按钮列表（如果菜单类型为按钮）
	var authList []types.AuthItem
	if pbMenu.MenuType == 3 && pbMenu.AuthName != "" {
		authList = append(authList, types.AuthItem{
			Title:    pbMenu.AuthName,
			AuthMark: pbMenu.AuthLabel,
		})
	}

	// 构建路由元数据
	meta := types.RouteMeta{
		Title:         pbMenu.Title,
		Icon:          pbMenu.Icon,
		ShowBadge:     pbMenu.ShowBadge == 1,
		ShowTextBadge: pbMenu.ShowTextBadge,
		IsHide:        pbMenu.IsHide == 1,
		IsHideTab:     pbMenu.IsHideTab == 1,
		Link:          pbMenu.Link,
		IsIframe:      pbMenu.IsIframe == 1,
		KeepAlive:     pbMenu.KeepAlive == 1,
		AuthList:      authList,
		IsFirstLevel:  pbMenu.IsFirstLevel == 1,
		Roles:         roles,
		FixedTab:      pbMenu.FixedTab == 1,
		ActivePath:    pbMenu.ActivePath,
		IsFullPage:    pbMenu.IsFullPage == 1,
		IsAuthButton:  pbMenu.MenuType == 3,
		AuthMark:      pbMenu.Label,
		ParentPath:    parentPath,
	}

	// 构建当前路由记录
	route := types.AppRouteRecord{
		Id:        pbMenu.Id,
		Name:      pbMenu.Name,
		Path:      pbMenu.Path,
		Component: pbMenu.Component,
		Redirect:  pbMenu.Redirect,
		Meta:      meta,
	}

	// 递归转换子菜单
	if len(pbMenu.Children) > 0 {
		var children []types.AppRouteRecord
		currentPath := pbMenu.Path
		if currentPath == "" {
			currentPath = parentPath
		} else if parentPath != "" {
			currentPath = strings.TrimSuffix(parentPath, "/") + "/" + strings.TrimPrefix(currentPath, "/")
		}

		for _, child := range pbMenu.Children {
			childRoute := l.convertToAppRouteRecord(child, currentPath)
			children = append(children, childRoute)
		}
		route.Children = children
	}

	return route
}
