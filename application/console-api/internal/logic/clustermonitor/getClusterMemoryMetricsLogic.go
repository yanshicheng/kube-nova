package clustermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterMemoryMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取集群内存指标
func NewGetClusterMemoryMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterMemoryMetricsLogic {
	return &GetClusterMemoryMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterMemoryMetricsLogic) GetClusterMemoryMetrics(req *types.GetClusterMemoryMetricsRequest) (resp *types.GetClusterMemoryMetricsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	cluster := client.Cluster()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	metrics, err := cluster.GetClusterMemoryMetrics(timeRange)
	if err != nil {
		l.Errorf("获取集群内存指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetClusterMemoryMetricsResponse{
		Data: convertClusterResourceSummary(metrics),
	}

	l.Infof("获取集群内存指标成功")
	return resp, nil
}
