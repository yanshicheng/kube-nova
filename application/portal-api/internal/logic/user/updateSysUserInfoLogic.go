package user

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSysUserInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSysUserInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSysUserInfoLogic {
	return &UpdateSysUserInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSysUserInfoLogic) UpdateSysUserInfo(req *types.UpdateSysUserInfoRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	userId, ok := l.ctx.Value("userId").(uint64)
	if !ok || userId == 0 {
		logx.WithContext(l.ctx).Error("更新用户信息失败: 未找到用户ID")
		return "", errors.New("未找到用户信息")
	}

	// 上下文中获取用户名
	userName, ok := l.ctx.Value("username").(string)
	if !ok || userName == "" {
		logx.WithContext(l.ctx).Error("更新用户信息失败: 未找到用户名")
		return "", errors.New("未找到用户名")
	}
	// 调用 RPC 服务更新个人信息
	_, err = l.svcCtx.PortalRpc.UserUpdateInfo(l.ctx, &pb.UpdateSysUserInfoReq{
		Id:        userId,
		Nickname:  req.Nickname,
		Email:     req.Email,
		Phone:     req.Phone,
		UpdatedBy: userName,
	})
	if err != nil {
		l.Errorf("更新个人信息失败: userId=%d, error=%v", userId, err)
		return "", err
	}

	l.Infof("更新个人信息成功: userId=%d", userId)
	return "更新个人信息成功", nil
}
