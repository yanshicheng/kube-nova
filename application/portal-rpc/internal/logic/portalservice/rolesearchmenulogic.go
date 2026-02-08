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

	// 如果没有关联菜单，直接返回空列表
	if len(roleMenus) == 0 {
		return &pb.SearchSysRoleMenuResp{
			MenuIds: []uint64{},
		}, nil
	}

	// 提取菜单ID列表
	menuIds := make([]uint64, 0, len(roleMenus))
	menuIdToRoleMenuId := make(map[uint64]uint64, len(roleMenus)) // 用于记录关联关系ID，便于清理无效数据
	for _, roleMenu := range roleMenus {
		menuIds = append(menuIds, roleMenu.MenuId)
		menuIdToRoleMenuId[roleMenu.MenuId] = roleMenu.Id
	}

	// 批量查询存在的菜单ID（单次数据库查询，解决 N+1 问题）
	existingIds, err := l.svcCtx.SysMenu.FindExistingIds(l.ctx, menuIds)
	if err != nil {
		l.Errorf("批量查询菜单失败: %v", err)
		return nil, errorx.Msg("查询菜单信息失败")
	}

	// 构建存在的菜单ID集合
	existingIdSet := make(map[uint64]struct{}, len(existingIds))
	for _, id := range existingIds {
		existingIdSet[id] = struct{}{}
	}

	// 找出无效的关联关系并异步清理
	var invalidRoleMenuIds []uint64
	for _, menuId := range menuIds {
		if _, exists := existingIdSet[menuId]; !exists {
			l.Infof("菜单不存在，关联数据已过期, menuId: %d", menuId)
			invalidRoleMenuIds = append(invalidRoleMenuIds, menuIdToRoleMenuId[menuId])
		}
	}

	// 异步清理无效的关联关系，不阻塞主流程
	if len(invalidRoleMenuIds) > 0 {
		go func(ids []uint64) {
			for _, id := range ids {
				if delErr := l.svcCtx.SysRoleMenu.DeleteSoft(l.ctx, id); delErr != nil {
					l.Errorf("删除无效的角色菜单关联失败: %v", delErr)
				}
			}
		}(invalidRoleMenuIds)
	}

	return &pb.SearchSysRoleMenuResp{
		MenuIds: existingIds, // 只返回有效的菜单ID
	}, nil
}
