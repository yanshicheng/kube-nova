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

type EnableSysPlatformLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEnableSysPlatformLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnableSysPlatformLogic {
	return &EnableSysPlatformLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EnableSysPlatformLogic) EnableSysPlatform(req *types.EnableSysPlatformRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务启用平台
	_, err = l.svcCtx.PortalRpc.PlatformEnable(l.ctx, &pb.EnableSysPlatformReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("启用平台失败: operator=%s, platformId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("启用平台成功: operator=%s, platformId=%d", username, req.Id)
	return "启用平台成功", nil
}
