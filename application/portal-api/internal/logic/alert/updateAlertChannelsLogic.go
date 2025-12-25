package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAlertChannelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAlertChannelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAlertChannelsLogic {
	return &UpdateAlertChannelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateAlertChannelsLogic) UpdateAlertChannels(req *types.UpdateAlertChannelsRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("更新告警渠道请求: operator=%s, id=%d, channelName=%s", username, req.Id, req.ChannelName)

	// 调用 RPC 服务更新告警渠道
	_, err = l.svcCtx.AlertPortalRpc.AlertChannelsUpdate(l.ctx, &pb.UpdateAlertChannelsReq{
		Id:          req.Id,
		ChannelName: req.ChannelName,
		ChannelType: req.ChannelType,
		Config:      req.Config,
		Description: req.Description,
		RetryTimes:  req.RetryTimes,
		Timeout:     req.Timeout,
		RateLimit:   req.RateLimit,
		UpdatedBy:   username,
	})
	if err != nil {
		l.Errorf("更新告警渠道失败: operator=%s, id=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("更新告警渠道成功: operator=%s, id=%d", username, req.Id)
	return "更新告警渠道成功", nil
}
