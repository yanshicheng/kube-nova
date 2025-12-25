package auth

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type VerifyTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewVerifyTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyTokenLogic {
	return &VerifyTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *VerifyTokenLogic) VerifyToken(req *types.VerifyTokenRequest) (resp *types.VerifyTokenResponse, err error) {

	// 调用 RPC 服务验证 Token
	res, err := l.svcCtx.SysAuthRpc.VerifyToken(l.ctx, &pb.VerifyTokenRequest{
		Token: req.Token,
	})
	if err != nil {
		l.Errorf("验证Token失败: error=%v", err)
		return nil, err
	}

	l.Infof("验证Token成功: isValid=%t", res.IsValid)

	return &types.VerifyTokenResponse{
		IsValid:      res.IsValid,
		ErrorType:    res.ErrorType,
		ErrorMessage: res.ErrorMessage,
		ExpireTime:   res.ExpireTime,
		UserId:       res.UserId,
		Username:     res.Username,
		Uuid:         res.Uuid,
		NickName:     res.NickName,
		Roles:        res.Roles,
	}, nil
}
