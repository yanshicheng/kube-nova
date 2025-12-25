package auth

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RefreshTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshTokenLogic {
	return &RefreshTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RefreshTokenLogic) RefreshToken(req *types.RefreshTokenRequest) (resp *types.RefreshTokenResponse, err error) {
	l.Infof("刷新Token请求")

	// 调用 RPC 服务刷新 Token
	res, err := l.svcCtx.SysAuthRpc.RefreshToken(l.ctx, &pb.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		l.Errorf("刷新Token失败: error=%v", err)
		return nil, err
	}

	l.Infof("刷新Token成功")

	return &types.RefreshTokenResponse{
		AccessToken:     res.AccessToken,
		AccessExpiresIn: res.AccessExpiresIn,
	}, nil
}
