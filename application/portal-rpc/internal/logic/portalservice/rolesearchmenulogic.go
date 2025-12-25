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

type RoleSearchMenuLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRoleSearchMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleSearchMenuLogic {
	return &RoleSearchMenuLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询关联了哪些菜单
func (l *RoleSearchMenuLogic) RoleSearchMenu(in *pb.SearchSysRoleMenuReq) (*pb.SearchSysRoleMenuResp, error) {
	// 参数验证
	if in.RoleId <= 0 {
		l.Errorf("查询角色关联菜单失败：角色ID无效")
		return nil, errorx.Msg("角色ID无效")
	}

	// 验证角色是否存在
	_, err := l.svcCtx.SysRole.FindOne(l.ctx, in.RoleId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("查询角色关联菜单失败：角色不存在, roleId: %d", in.RoleId)
			return nil, errorx.Msg("角色不存在")
		}
		l.Errorf("查询角色信息失败: %v", err)
		return nil, errorx.Msg("查询角色信息失败")
	}

	// 查询角色关联的菜单ID列表
	roleMenus, err := l.svcCtx.SysRoleMenu.SearchNoPage(l.ctx, "", true, "`role_id` = ?", in.RoleId)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询角色菜单关联失败: %v", err)
		return nil, errorx.Msg("查询角色菜单关联失败")
	}

	// 提取菜单ID列表
	var menuIds []uint64
	validMenuIds := make([]uint64, 0, len(roleMenus))

	for _, roleMenu := range roleMenus {
		menuIds = append(menuIds, roleMenu.MenuId)

		// 验证菜单是否还存在（清理无效关联）
		_, err := l.svcCtx.SysMenu.FindOne(l.ctx, roleMenu.MenuId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("菜单不存在，关联数据可能已过期, menuId: %d", roleMenu.MenuId)
				// 可以选择在这里删除无效的关联关系
				if delErr := l.svcCtx.SysRoleMenu.DeleteSoft(l.ctx, roleMenu.Id); delErr != nil {
					l.Errorf("删除无效的角色菜单关联失败: %v", delErr)
				}
				continue
			}
			l.Errorf("查询菜单信息失败: %v", err)
			return nil, errorx.Msg("查询菜单信息失败")
		}
		validMenuIds = append(validMenuIds, roleMenu.MenuId)
	}

	return &pb.SearchSysRoleMenuResp{
		MenuIds: validMenuIds, // 只返回有效的菜单ID
	}, nil
}
