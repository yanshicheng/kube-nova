package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type TestLinkLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTestLinkLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TestLinkLogic {
	return &TestLinkLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TestLinkLogic) TestLink(req *types.TestLinkRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("测试连接请求: operator=%s, channelType=%s", username, req.ChannelType)

	// 调用 RPC 服务测试连接
	_, err = l.svcCtx.AlertPortalRpc.AlertChannelsTestLink(l.ctx, &pb.TestLinkReq{
		ChannelType: req.ChannelType,
		Config:      req.Config,
		ToNotifys:   req.ToNotifys,
	})
	if err != nil {
		l.Errorf("测试连接失败: operator=%s, channelType=%s, error=%v", username, req.ChannelType, err)
		return "", err
	}

	l.Infof("测试连接成功: operator=%s, channelType=%s", username, req.ChannelType)
	return "测试连接成功", nil
}
