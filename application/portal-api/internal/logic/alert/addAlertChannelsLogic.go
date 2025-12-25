package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddAlertChannelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddAlertChannelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddAlertChannelsLogic {
	return &AddAlertChannelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddAlertChannelsLogic) AddAlertChannels(req *types.AddAlertChannelsRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("添加告警渠道请求: operator=%s, channelName=%s", username, req.ChannelName)

	// 调用 RPC 服务添加告警渠道
	_, err = l.svcCtx.AlertPortalRpc.AlertChannelsAdd(l.ctx, &pb.AddAlertChannelsReq{
		ChannelName: req.ChannelName,
		ChannelType: req.ChannelType,
		Config:      req.Config,
		Description: req.Description,
		RetryTimes:  req.RetryTimes,
		Timeout:     req.Timeout,
		RateLimit:   req.RateLimit,
		CreatedBy:   username,
		UpdatedBy:   username,
	})
	if err != nil {
		l.Errorf("添加告警渠道失败: operator=%s, channelName=%s, error=%v", username, req.ChannelName, err)
		return "", err
	}

	l.Infof("添加告警渠道成功: operator=%s, channelName=%s", username, req.ChannelName)
	return "添加告警渠道成功", nil
}
