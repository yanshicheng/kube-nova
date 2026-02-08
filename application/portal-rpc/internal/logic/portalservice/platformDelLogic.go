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

type PlatformDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlatformDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformDelLogic {
	return &PlatformDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PlatformDel 删除平台（软删除）
func (l *PlatformDelLogic) PlatformDel(in *pb.DelSysPlatformReq) (*pb.DelSysPlatformResp, error) {
	// 参数验证
	if in.Id <= 0 {
		l.Errorf("删除平台失败：平台ID无效")
		return nil, errorx.Msg("平台ID无效")
	}

	// 查询平台是否存在
	platform, err := l.svcCtx.SysPlatformModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("删除平台失败：平台不存在, id: %d", in.Id)
			return nil, errorx.Msg("平台不存在")
		}
		l.Errorf("查询平台失败: %v", err)
		return nil, errorx.Msg("查询平台失败")
	}

	// 检查是否为默认平台，默认平台不允许删除
	if platform.IsDefault == 1 {
		l.Errorf("删除平台失败：默认平台不允许删除, id: %d", in.Id)
		return nil, errorx.Msg("默认平台不允许删除，请先设置其他平台为默认平台")
	}

	// 检查平台下是否有菜单
	menus, err := l.svcCtx.SysMenuModel.SearchNoPage(l.ctx, "", true, "`platform_id` = ?", in.Id)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询平台菜单失败: %v", err)
		return nil, errorx.Msg("查询平台菜单失败")
	}
	if len(menus) > 0 {
		l.Errorf("删除平台失败：平台下存在菜单，请先删除菜单, id: %d, 菜单数量: %d", in.Id, len(menus))
		return nil, errorx.Msg("平台下存在菜单，请先删除菜单")
	}

	// 检查平台下是否有用户绑定
	userPlatforms, err := l.svcCtx.SysUserPlatformModel.SearchNoPage(l.ctx, "", true, "`platform_id` = ?", in.Id)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询平台用户绑定失败: %v", err)
		return nil, errorx.Msg("查询平台用户绑定失败")
	}
	if len(userPlatforms) > 0 {
		l.Errorf("删除平台失败：平台下存在用户绑定，请先解绑用户, id: %d, 用户数量: %d", in.Id, len(userPlatforms))
		return nil, errorx.Msg("平台下存在用户绑定，请先解绑用户")
	}

	// 执行软删除
	if err := l.svcCtx.SysPlatformModel.DeleteSoft(l.ctx, in.Id); err != nil {
		l.Errorf("删除平台失败: %v", err)
		return nil, errorx.Msg("删除平台失败")
	}

	l.Infof("删除平台成功，平台ID: %d", in.Id)
	return &pb.DelSysPlatformResp{}, nil
}
