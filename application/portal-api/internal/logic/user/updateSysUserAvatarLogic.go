package user

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSysUserAvatarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSysUserAvatarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSysUserAvatarLogic {
	return &UpdateSysUserAvatarLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSysUserAvatarLogic) UpdateSysUserAvatar(req *types.UpdateSysUserAvatarRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	userId, ok := l.ctx.Value("userId").(uint64)
	if !ok || userId == 0 {
		logx.WithContext(l.ctx).Error("更新头像失败: 未找到用户ID")
		return "", errors.New("未找到用户信息")
	}
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}
	// 调用 RPC 服务更新头像
	_, err = l.svcCtx.PortalRpc.UserUpdateAvatar(l.ctx, &pb.UpdateSysUserAvatarReq{
		Id:        userId,
		Avatar:    req.Avatar,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("更新头像失败: userId=%d, error=%v", userId, err)
		return "", err
	}

	l.Infof("更新头像成功: userId=%d", userId)
	return "更新头像成功", nil
}
