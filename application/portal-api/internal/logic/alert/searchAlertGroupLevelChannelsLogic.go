package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchAlertGroupLevelChannelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchAlertGroupLevelChannelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchAlertGroupLevelChannelsLogic {
	return &SearchAlertGroupLevelChannelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchAlertGroupLevelChannelsLogic) SearchAlertGroupLevelChannels(req *types.SearchAlertGroupLevelChannelsRequest) (resp *types.SearchAlertGroupLevelChannelsResponse, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("搜索告警组级别渠道请求: operator=%s, page=%d, pageSize=%d", username, req.Page, req.PageSize)

	// 调用 RPC 服务搜索告警组级别渠道
	result, err := l.svcCtx.AlertPortalRpc.AlertGroupLevelChannelsSearch(l.ctx, &pb.SearchAlertGroupLevelChannelsReq{
		Page:     req.Page,
		PageSize: req.PageSize,
		OrderStr: req.OrderStr,
		IsAsc:    req.IsAsc,
		GroupId:  req.GroupId,
		Severity: req.Severity,
	})
	if err != nil {
		l.Errorf("搜索告警组级别渠道失败: operator=%s, error=%v", username, err)
		return nil, err
	}

	// 转换数据
	items := make([]types.AlertGroupLevelChannels, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, types.AlertGroupLevelChannels{
			Id:        item.Id,
			GroupId:   item.GroupId,
			Severity:  item.Severity,
			ChannelId: item.ChannelId,
			CreatedBy: item.CreatedBy,
			UpdatedBy: item.UpdatedBy,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		})
	}

	resp = &types.SearchAlertGroupLevelChannelsResponse{
		Items: items,
	}

	l.Infof("搜索告警组级别渠道成功: operator=%s, count=%d", username, len(items))
	return resp, nil
}
