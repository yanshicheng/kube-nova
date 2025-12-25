package clustermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetAPIServerMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 API Server 指标
func NewGetAPIServerMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAPIServerMetricsLogic {
	return &GetAPIServerMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAPIServerMetricsLogic) GetAPIServerMetrics(req *types.GetAPIServerMetricsRequest) (resp *types.GetAPIServerMetricsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	cluster := client.Cluster()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	metrics, err := cluster.GetAPIServerMetrics(timeRange)
	if err != nil {
		l.Errorf("获取 API Server 指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetAPIServerMetricsResponse{
		Data: convertAPIServerMetrics(metrics),
	}

	l.Infof("获取 API Server 指标成功")
	return resp, nil
}
