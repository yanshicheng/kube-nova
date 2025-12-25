package user

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResetSysUserPwdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResetSysUserPwdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetSysUserPwdLogic {
	return &ResetSysUserPwdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ResetSysUserPwdLogic) ResetSysUserPwd(req *types.ResetSysUserPwdRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("重置用户密码请求: operator=%s, userId=%s", username, req.Id)

	// 调用 RPC 服务重置用户密码
	_, err = l.svcCtx.PortalRpc.UserResetPwd(l.ctx, &pb.ResetSysUserPwdReq{
		Id:        req.Id,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("重置用户密码失败: operator=%s, userId=%s, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("重置用户密码成功: operator=%s, userId=%s", username, req.Id)
	return "重置用户密码成功", nil
}
