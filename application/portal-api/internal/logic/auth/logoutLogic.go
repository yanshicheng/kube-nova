package auth

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LogoutLogic) Logout(req *types.LogoutRequest) (resp *types.LogoutResponse, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "unknown"
	}

	l.Infof("用户登出请求: username=%s", username)

	// 调用 RPC 服务进行用户登出
	_, err = l.svcCtx.SysAuthRpc.Logout(l.ctx, &pb.LogoutRequest{
		Username: username,
		Uuid:     req.Uuid,
	})
	if err != nil {
		l.Errorf("用户登出失败: username=%s, error=%v", username, err)
		return &types.LogoutResponse{
			Success: false,
			Message: "登出失败",
		}, nil
	}

	l.Infof("用户登出成功: username=%s", username)

	return &types.LogoutResponse{
		Success: true,
		Message: "登出成功",
	}, nil
}
