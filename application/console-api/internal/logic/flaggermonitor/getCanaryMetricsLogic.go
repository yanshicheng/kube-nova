package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetCanaryMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Canary 指标
func NewGetCanaryMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCanaryMetricsLogic {
	return &GetCanaryMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCanaryMetricsLogic) GetCanaryMetrics(req *types.GetCanaryMetricsRequest) (resp *types.GetCanaryMetricsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	metrics, err := flagger.GetCanaryMetrics(req.Namespace, req.Name, timeRange)
	if err != nil {
		l.Errorf("获取 Canary 指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetCanaryMetricsResponse{
		Data: convertCanaryMetrics(metrics),
	}

	l.Infof("获取 Canary %s/%s 指标成功", req.Namespace, req.Name)
	return resp, nil
}
