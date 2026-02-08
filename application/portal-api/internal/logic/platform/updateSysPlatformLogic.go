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

type UpdateSysPlatformLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSysPlatformLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSysPlatformLogic {
	return &UpdateSysPlatformLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSysPlatformLogic) UpdateSysPlatform(req *types.UpdateSysPlatformRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务更新平台
	_, err = l.svcCtx.PortalRpc.PlatformUpdate(l.ctx, &pb.UpdateSysPlatformReq{
		Id:           req.Id,
		PlatformCode: req.PlatformCode,
		PlatformName: req.PlatformName,
		PlatformDesc: req.PlatformDesc,
		PlatformIcon: req.PlatformIcon,
		Sort:         req.Sort,
		IsEnable:     req.IsEnable,
		IsDefault:    req.IsDefault,
		UpdateBy:     username,
	})
	if err != nil {
		l.Errorf("更新平台失败: operator=%s, platformId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("更新平台成功: operator=%s, platformId=%d", username, req.Id)
	return "更新平台成功", nil
}
