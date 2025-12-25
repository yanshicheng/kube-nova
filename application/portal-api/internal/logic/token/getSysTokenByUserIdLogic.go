package token

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSysTokenByUserIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSysTokenByUserIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSysTokenByUserIdLogic {
	return &GetSysTokenByUserIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSysTokenByUserIdLogic) GetSysTokenByUserId(req *types.GetSysTokenByUserIdRequest) (resp *types.GetSysTokenByUserIdResponse, err error) {

	// 调用 RPC 服务根据用户ID获取Token
	res, err := l.svcCtx.PortalRpc.TokenGetByUserId(l.ctx, &pb.GetSysTokenByUserIdReq{
		UserId: req.UserId,
	})
	if err != nil {
		l.Errorf("根据用户ID获取Token失败: userId=%d, error=%v", req.UserId, err)
		return nil, err
	}

	return &types.GetSysTokenByUserIdResponse{
		Token: res.Token,
	}, nil
}
