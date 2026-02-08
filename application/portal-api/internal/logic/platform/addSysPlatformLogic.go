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

type AddSysPlatformLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddSysPlatformLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddSysPlatformLogic {
	return &AddSysPlatformLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddSysPlatformLogic) AddSysPlatform(req *types.AddSysPlatformRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务添加平台
	_, err = l.svcCtx.PortalRpc.PlatformAdd(l.ctx, &pb.AddSysPlatformReq{
		PlatformCode: req.PlatformCode,
		PlatformName: req.PlatformName,
		PlatformDesc: req.PlatformDesc,
		PlatformIcon: req.PlatformIcon,
		Sort:         req.Sort,
		IsEnable:     req.IsEnable,
		CreateBy:     username,
	})
	if err != nil {
		l.Errorf("添加平台失败: operator=%s, platformCode=%s, error=%v", username, req.PlatformCode, err)
		return "", err
	}

	l.Infof("添加平台成功: operator=%s, platformCode=%s", username, req.PlatformCode)
	return "添加平台成功", nil
}
