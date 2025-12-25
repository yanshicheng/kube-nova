package token

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddSysTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddSysTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddSysTokenLogic {
	return &AddSysTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddSysTokenLogic) AddSysToken(req *types.AddSysTokenRequest) (resp *types.AddSysTokenResponse, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务添加Token
	res, err := l.svcCtx.PortalRpc.TokenAdd(l.ctx, &pb.AddSysTokenReq{
		OwnerType:  req.OwnerType,
		OwnerId:    req.OwnerId,
		Name:       req.Name,
		Type:       req.Type,
		ExpireTime: req.ExpireTime,
		Status:     req.Status,
		CreatedBy:  username,
		UpdatedBy:  username,
	})
	if err != nil {
		l.Errorf("添加Token失败: operator=%s, tokenName=%s, error=%v", username, req.Name, err)
		return nil, err
	}

	l.Infof("添加Token成功: operator=%s, tokenName=%s", username, req.Name)

	return &types.AddSysTokenResponse{
		Token: res.Token,
	}, nil
}
