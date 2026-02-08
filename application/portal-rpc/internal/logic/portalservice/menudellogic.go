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

type MenuDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMenuDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MenuDelLogic {
	return &MenuDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// MenuDel 删除菜单
func (l *MenuDelLogic) MenuDel(in *pb.DelSysMenuReq) (*pb.DelSysMenuResp, error) {
	// 记录请求日志
	// 参数验证
	if in.Id <= 0 {
		l.Errorf("删除菜单失败：菜单ID无效")
		return nil, errorx.Msg("菜单ID无效")
	}

	// 检查菜单是否存在
	existingMenu, err := l.svcCtx.SysMenu.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("删除菜单失败：菜单不存在, menuId: %d", in.Id)
			return nil, errorx.Msg("菜单不存在")
		}
		l.Errorf("查询菜单信息失败: %v", err)
		return nil, errorx.Msg("查询菜单信息失败")
	}

	// 检查是否有子菜单（同一平台下）
	subMenus, err := l.svcCtx.SysMenu.SearchNoPage(l.ctx, "", true, "`platform_id` = ? AND `parent_id` = ?", existingMenu.PlatformId, in.Id)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("检查子菜单失败: platformId=%d, error=%v", existingMenu.PlatformId, err)
		return nil, errorx.Msg("检查子菜单失败")
	}
	if len(subMenus) > 0 {
		l.Errorf("删除菜单失败：存在子菜单，不能删除, menuId: %d, platformId: %d, subMenuCount: %d", in.Id, existingMenu.PlatformId, len(subMenus))
		return nil, errorx.Msg("该菜单下存在子菜单，请先删除子菜单")
	}

	// 检查是否有角色关联到此菜单
	roleMenus, err := l.svcCtx.SysRoleMenu.SearchNoPage(l.ctx, "", true, "`menu_id` = ?", in.Id)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("检查菜单角色关联失败: %v", err)
		return nil, errorx.Msg("检查菜单角色关联失败")
	}
	if len(roleMenus) > 0 {
		l.Errorf("删除菜单失败：有角色关联此菜单，不能删除, menuId: %d, roleCount: %d", in.Id, len(roleMenus))
		return nil, errorx.Msg("该菜单已被角色关联，请先解除关联关系")
	}

	// 执行软删除
	err = l.svcCtx.SysMenu.DeleteSoft(l.ctx, in.Id)
	if err != nil {
		l.Errorf("删除菜单失败: platformId=%d, error=%v", existingMenu.PlatformId, err)
		return nil, errorx.Msg("删除菜单失败")
	}

	l.Infof("删除菜单成功，菜单ID: %d, 菜单名称: %s, 菜单类型: %d, 平台ID: %d",
		in.Id, existingMenu.Name, existingMenu.MenuType, existingMenu.PlatformId)
	return &pb.DelSysMenuResp{}, nil
}
