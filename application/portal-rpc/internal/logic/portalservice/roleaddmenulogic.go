package portalservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleAddMenuLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRoleAddMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleAddMenuLogic {
	return &RoleAddMenuLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RoleAddMenu 角色菜单关联
func (l *RoleAddMenuLogic) RoleAddMenu(in *pb.AddSysRoleMenuReq) (*pb.AddSysRoleMenuResp, error) {
	// 参数验证
	if in.RoleId <= 0 {
		l.Errorf("设置角色菜单关联失败：角色ID无效")
		return nil, errorx.Msg("角色ID无效")
	}

	// 验证角色是否存在
	_, err := l.svcCtx.SysRole.FindOne(l.ctx, in.RoleId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("设置角色菜单关联失败：角色不存在, roleId: %d", in.RoleId)
			return nil, errorx.Msg("角色不存在")
		}
		l.Errorf("查询角色信息失败: %v", err)
		return nil, errorx.Msg("查询角色信息失败")
	}

	// 验证所有菜单ID是否存在
	validMenuIds := make([]uint64, 0, len(in.MenuIds))
	for _, menuId := range in.MenuIds {
		if menuId <= 0 {
			continue // 跳过无效的菜单ID
		}

		_, err := l.svcCtx.SysMenu.FindOne(l.ctx, menuId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("菜单不存在，跳过, menuId: %d", menuId)
				continue
			}
			l.Errorf("查询菜单信息失败: %v", err)
			return nil, errorx.Msg("查询菜单信息失败")
		}
		validMenuIds = append(validMenuIds, menuId)
	}

	// 使用事务处理角色菜单关联
	err = l.svcCtx.SysRoleMenu.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 1. 删除该角色的所有现有菜单关联（同时清理缓存）
		if err := l.svcCtx.SysRoleMenu.DeleteByRoleIdWithCache(ctx, session, in.RoleId); err != nil {
			l.Errorf("删除角色菜单关联失败: %v", err)
			return err
		}

		// 2. 添加新的菜单关联
		for _, menuId := range validMenuIds {
			roleMenu := &model.SysRoleMenu{
				RoleId: in.RoleId,
				MenuId: menuId,
			}
			if err := l.svcCtx.SysRoleMenu.InsertWithSession(ctx, session, roleMenu); err != nil {
				l.Errorf("添加角色菜单关联失败: %v", err)
				return err
			}
		}

		return nil
	})

	if err != nil {
		l.Errorf("设置角色菜单关联失败: %v", err)
		return nil, errorx.Msg("设置角色菜单关联失败")
	}

	l.Infof("设置角色菜单关联成功，角色ID: %d, 有效菜单数量: %d", in.RoleId, len(validMenuIds))
	return &pb.AddSysRoleMenuResp{}, nil
}
