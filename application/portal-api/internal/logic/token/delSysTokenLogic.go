package token

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DelSysTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDelSysTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DelSysTokenLogic {
	return &DelSysTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DelSysTokenLogic) DelSysToken(req *types.DefaultIdRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务删除Token
	_, err = l.svcCtx.PortalRpc.TokenDel(l.ctx, &pb.DelSysTokenReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("删除Token失败: operator=%s, tokenId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("删除Token成功: operator=%s, tokenId=%d", username, req.Id)
	return "删除Token成功", nil
}
