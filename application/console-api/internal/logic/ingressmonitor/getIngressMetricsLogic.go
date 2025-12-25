package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetIngressMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Ingress 综合指标
func NewGetIngressMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetIngressMetricsLogic {
	return &GetIngressMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetIngressMetricsLogic) GetIngressMetrics(req *types.GetIngressMetricsRequest) (resp *types.GetIngressMetricsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	metrics, err := ingress.GetIngressMetrics(req.Namespace, req.IngressName, timeRange)
	if err != nil {
		l.Errorf("获取 Ingress 综合指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetIngressMetricsResponse{
		Data: convertIngressMetrics(metrics),
	}

	l.Infof("获取 Ingress %s/%s 综合指标成功", req.Namespace, req.IngressName)
	return resp, nil
}
