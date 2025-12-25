package clustermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetSchedulerMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Scheduler 指标
func NewGetSchedulerMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSchedulerMetricsLogic {
	return &GetSchedulerMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSchedulerMetricsLogic) GetSchedulerMetrics(req *types.GetSchedulerMetricsRequest) (resp *types.GetSchedulerMetricsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	cluster := client.Cluster()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	scheduler, err := cluster.GetSchedulerMetrics(timeRange)
	if err != nil {
		l.Errorf("获取 Scheduler 指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetSchedulerMetricsResponse{
		Data: convertSchedulerMetrics(scheduler),
	}

	l.Infof("获取 Scheduler 指标成功")
	return resp, nil
}
