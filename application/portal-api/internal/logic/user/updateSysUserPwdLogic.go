package user

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSysUserPwdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSysUserPwdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSysUserPwdLogic {
	return &UpdateSysUserPwdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSysUserPwdLogic) UpdateSysUserPwd(req *types.UpdateSysUserPwdRequest) (resp string, err error) {

	// 调用 RPC 服务修改密码
	_, err = l.svcCtx.PortalRpc.UserUpdatePwd(l.ctx, &pb.UpdateSysUserPwdReq{
		Username:        req.Username,
		OldPassword:     req.OldPassword,
		NewPassword:     req.NewPassword,
		ConfirmPassword: req.ConfirmPassword,
	})
	if err != nil {
		l.Errorf("修改密码失败: username=%s, error=%v", req.Username, err)
		return "", err
	}

	l.Infof("修改密码成功: username=%s", req.Username)
	return "修改密码成功", nil
}
