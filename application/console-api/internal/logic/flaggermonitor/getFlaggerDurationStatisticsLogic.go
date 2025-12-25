package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetFlaggerDurationStatisticsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Flagger 持续时间统计
func NewGetFlaggerDurationStatisticsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFlaggerDurationStatisticsLogic {
	return &GetFlaggerDurationStatisticsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetFlaggerDurationStatisticsLogic) GetFlaggerDurationStatistics(req *types.GetFlaggerDurationStatisticsRequest) (resp *types.GetFlaggerDurationStatisticsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	durationStats, err := flagger.GetFlaggerDurationStatistics(timeRange)
	if err != nil {
		l.Errorf("获取 Flagger 持续时间统计失败: %v", err)
		return nil, err
	}

	resp = &types.GetFlaggerDurationStatisticsResponse{
		Data: convertFlaggerDurationStatistics(durationStats),
	}

	l.Infof("获取 Flagger 持续时间统计成功")
	return resp, nil
}
