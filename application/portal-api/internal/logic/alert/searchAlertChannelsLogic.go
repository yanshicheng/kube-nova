package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchAlertChannelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchAlertChannelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchAlertChannelsLogic {
	return &SearchAlertChannelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchAlertChannelsLogic) SearchAlertChannels(req *types.SearchAlertChannelsRequest) (resp *types.SearchAlertChannelsResponse, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("搜索告警渠道请求（不分页）: operator=%s, uuid=%s, channelName=%s, channelType=%s",
		username, req.Uuid, req.ChannelName, req.ChannelType)

	// 调用 RPC 服务搜索告警渠道（不分页）
	result, err := l.svcCtx.AlertPortalRpc.AlertChannelsSearch(l.ctx, &pb.SearchAlertChannelsReq{
		Uuid:        req.Uuid,
		ChannelName: req.ChannelName,
		ChannelType: req.ChannelType,
	})
	if err != nil {
		l.Errorf("搜索告警渠道失败: operator=%s, error=%v", username, err)
		return nil, err
	}

	// 转换数据
	items := make([]types.AlertChannels, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, types.AlertChannels{
			Id:          item.Id,
			Uuid:        item.Uuid,
			ChannelName: item.ChannelName,
			ChannelType: item.ChannelType,
			//Config:      item.Config,
			Description: item.Description,
			RetryTimes:  item.RetryTimes,
			Timeout:     item.Timeout,
			RateLimit:   item.RateLimit,
			CreatedBy:   item.CreatedBy,
			UpdatedBy:   item.UpdatedBy,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	resp = &types.SearchAlertChannelsResponse{
		Items: items,
	}

	l.Infof("搜索告警渠道成功: operator=%s, count=%d", username, len(items))
	return resp, nil
}
