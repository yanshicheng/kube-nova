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

type RoleDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRoleDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleDelLogic {
	return &RoleDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RoleDel 删除角色
func (l *RoleDelLogic) RoleDel(in *pb.DelSysRoleReq) (*pb.DelSysRoleResp, error) {

	// 参数验证
	if in.Id <= 0 {
		l.Errorf("删除角色失败：角色ID无效")
		return nil, errorx.Msg("角色ID无效")
	}

	// 检查角色是否存在
	existingRole, err := l.svcCtx.SysRole.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("删除角色失败：角色不存在, roleId: %d", in.Id)
			return nil, errorx.Msg("角色不存在")
		}
		l.Errorf("查询角色信息失败: %v", err)
		return nil, errorx.Msg("查询角色信息失败")
	}

	// 检查是否有用户关联到此角色
	userRoles, err := l.svcCtx.SysUserRole.SearchNoPage(l.ctx, "", true, "`role_id` = ?", in.Id)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("检查角色用户关联失败: %v", err)
		return nil, errorx.Msg("检查角色用户关联失败")
	}
	if len(userRoles) > 0 {
		l.Errorf("删除角色失败：有用户使用此角色，不能删除, roleId: %d, userCount: %d", in.Id, len(userRoles))
		return nil, errorx.Msg("该角色已被用户使用，请先解除用户角色关联")
	}

	// 检查是否有菜单关联到此角色
	roleMenus, err := l.svcCtx.SysRoleMenu.SearchNoPage(l.ctx, "", true, "`role_id` = ?", in.Id)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("检查角色菜单关联失败: %v", err)
		return nil, errorx.Msg("检查角色菜单关联失败")
	}
	if len(roleMenus) > 0 {
		l.Errorf("删除角色时清理菜单关联，roleId: %d, menuCount: %d", in.Id, len(roleMenus))
		// 删除角色菜单关联（可以自动清理）
		for _, roleMenu := range roleMenus {
			if err := l.svcCtx.SysRoleMenu.DeleteSoft(l.ctx, roleMenu.Id); err != nil {
				l.Errorf("删除角色菜单关联失败: %v", err)
			}
		}
	}

	// 检查是否有API权限关联到此角色
	roleAPIs, err := l.svcCtx.SysRoleApi.SearchNoPage(l.ctx, "", true, "`role_id` = ?", in.Id)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("检查角色API关联失败: %v", err)
		return nil, errorx.Msg("检查角色API关联失败")
	}
	if len(roleAPIs) > 0 {
		l.Errorf("删除角色时清理API权限关联，roleId: %d, apiCount: %d", in.Id, len(roleAPIs))
		// 删除角色API关联（可以自动清理）
		for _, roleAPI := range roleAPIs {
			if err := l.svcCtx.SysRoleApi.DeleteSoft(l.ctx, roleAPI.Id); err != nil {
				l.Errorf("删除角色API关联失败: %v", err)
			}
		}
	}

	// 检查是否为系统预设角色（如super、admin等）
	systemRoles := []string{"super", "admin", "system"}
	for _, sysRole := range systemRoles {
		if existingRole.Code == sysRole {
			l.Errorf("删除角色失败：不能删除系统预设角色, roleCode: %s", existingRole.Code)
			return nil, errorx.Msg("不能删除系统预设角色")
		}
	}

	// 执行软删除
	err = l.svcCtx.SysRole.DeleteSoft(l.ctx, in.Id)
	if err != nil {
		l.Errorf("删除角色失败: %v", err)
		return nil, errorx.Msg("删除角色失败")
	}

	l.Infof("删除角色成功，角色ID: %d, 角色名称: %s, 角色编码: %s",
		in.Id, existingRole.Name, existingRole.Code)
	return &pb.DelSysRoleResp{}, nil
}
