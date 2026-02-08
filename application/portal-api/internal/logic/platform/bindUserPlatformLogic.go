// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type BindUserPlatformLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBindUserPlatformLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindUserPlatformLogic {
	return &BindUserPlatformLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BindUserPlatformLogic) BindUserPlatform(req *types.BindUserPlatformRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务绑定用户平台
	_, err = l.svcCtx.PortalRpc.UserPlatformBind(l.ctx, &pb.BindUserPlatformReq{
		UserId:      req.UserId,
		PlatformIds: req.PlatformIds,
	})
	if err != nil {
		l.Errorf("绑定用户平台失败: operator=%s, userId=%d, platformId=%d, error=%v", username, req.UserId, req.PlatformIds, err)
		return "", err
	}

	l.Infof("绑定用户平台成功: operator=%s, userId=%d, platformId=%d", username, req.UserId, req.PlatformIds)
	return "绑定用户平台成功", nil
}
