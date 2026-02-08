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

type DelSysPlatformLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDelSysPlatformLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DelSysPlatformLogic {
	return &DelSysPlatformLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DelSysPlatformLogic) DelSysPlatform(req *types.DefaultIdRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务删除平台
	_, err = l.svcCtx.PortalRpc.PlatformDel(l.ctx, &pb.DelSysPlatformReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("删除平台失败: operator=%s, platformId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("删除平台成功: operator=%s, platformId=%d", username, req.Id)
	return "删除平台成功", nil
}
