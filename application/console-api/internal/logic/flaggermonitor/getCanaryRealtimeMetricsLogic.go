package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetCanaryRealtimeMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Canary 实时监控指标
func NewGetCanaryRealtimeMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCanaryRealtimeMetricsLogic {
	return &GetCanaryRealtimeMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCanaryRealtimeMetricsLogic) GetCanaryRealtimeMetrics(req *types.GetCanaryRealtimeMetricsRequest) (resp *types.GetCanaryRealtimeMetricsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()

	metrics, err := flagger.GetCanaryRealtimeMetrics(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Canary 实时监控指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetCanaryRealtimeMetricsResponse{
		Data: convertCanaryRealtimeMetrics(metrics),
	}

	l.Infof("获取 Canary %s/%s 实时监控指标成功", req.Namespace, req.Name)
	return resp, nil
}
