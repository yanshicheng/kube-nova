package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListIngressMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列出 Namespace 下的 Ingress 指标
func NewListIngressMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListIngressMetricsLogic {
	return &ListIngressMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListIngressMetricsLogic) ListIngressMetrics(req *types.ListIngressMetricsRequest) (resp *types.ListIngressMetricsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	metricsList, err := ingress.ListIngressMetrics(req.Namespace, timeRange)
	if err != nil {
		l.Errorf("列出 Namespace 下的 Ingress 指标失败: %v", err)
		return nil, err
	}

	resp = &types.ListIngressMetricsResponse{
		Data: convertIngressMetricsList(metricsList),
	}

	l.Infof("列出 Namespace %s 下的 Ingress 指标成功，共 %d 个", req.Namespace, len(metricsList))
	return resp, nil
}
