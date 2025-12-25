package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetFlaggerWebhookStatisticsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Flagger Webhook 统计
func NewGetFlaggerWebhookStatisticsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFlaggerWebhookStatisticsLogic {
	return &GetFlaggerWebhookStatisticsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetFlaggerWebhookStatisticsLogic) GetFlaggerWebhookStatistics(req *types.GetFlaggerWebhookStatisticsRequest) (resp *types.GetFlaggerWebhookStatisticsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	webhookStats, err := flagger.GetFlaggerWebhookStatistics(timeRange)
	if err != nil {
		l.Errorf("获取 Flagger Webhook 统计失败: %v", err)
		return nil, err
	}

	resp = &types.GetFlaggerWebhookStatisticsResponse{
		Data: convertFlaggerWebhookStatistics(webhookStats),
	}

	l.Infof("获取 Flagger Webhook 统计成功")
	return resp, nil
}
