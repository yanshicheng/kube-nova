package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetHostMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取单个 Host 级别监控详情
func NewGetHostMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetHostMetricsLogic {
	return &GetHostMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetHostMetricsLogic) GetHostMetrics(req *types.GetHostMetricsRequest) (resp *types.GetHostMetricsResponse, err error) {
	l.Infof("开始查询 Host 监控指标: host=%s, clusterUuid=%s", req.Host, req.ClusterUuid)

	// 获取 Prometheus 客户端
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: clusterUuid=%s, error=%v", req.ClusterUuid, err)
		return nil, err
	}

	ingressOp := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, req.Step)

	// 查询单个 host 的详细指标
	metric, err := ingressOp.GetHostMetricsDetail(req.Host, timeRange)
	if err != nil {
		l.Errorf("获取 Host 指标失败: host=%s, error=%v", req.Host, err)
		return nil, err
	}

	// 转换 prometheusmanager types 到 API types
	resp = &types.GetHostMetricsResponse{
		Data: convertHostMetrics(metric),
	}

	l.Infof("成功获取 Host %s 的监控指标", req.Host)
	return resp, nil
}
