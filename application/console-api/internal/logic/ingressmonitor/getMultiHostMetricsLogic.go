package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetMultiHostMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 批量获取 Host 级别监控详情
func NewGetMultiHostMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMultiHostMetricsLogic {
	return &GetMultiHostMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetMultiHostMetricsLogic) GetMultiHostMetrics(req *types.GetMultiHostMetricsRequest) (resp *types.GetMultiHostMetricsResponse, err error) {
	l.Infof("开始批量查询 %d 个 Host 的监控指标: clusterUuid=%s, hosts=%v", len(req.Hosts), req.ClusterUuid, req.Hosts)

	// 获取 Prometheus 客户端
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: clusterUuid=%s, error=%v", req.ClusterUuid, err)
		return nil, err
	}

	ingressOp := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, req.Step)

	// 批量查询多个 host 的指标
	metricsList, err := ingressOp.GetMultiHostMetrics(req.Hosts, timeRange)
	if err != nil {
		l.Errorf("批量获取 Host 指标失败: error=%v", err)
		return nil, err
	}

	// 转换 prometheusmanager types 到 API types
	resp = &types.GetMultiHostMetricsResponse{
		Data: convertHostMetricsList(metricsList),
	}

	l.Infof("批量查询完成: 成功获取 %d/%d 个 Host 的监控指标", len(resp.Data), len(req.Hosts))
	return resp, nil
}
