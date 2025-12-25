package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceCanaryMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace Canary 指标
func NewGetNamespaceCanaryMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceCanaryMetricsLogic {
	return &GetNamespaceCanaryMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceCanaryMetricsLogic) GetNamespaceCanaryMetrics(req *types.GetNamespaceCanaryMetricsRequest) (resp *types.GetNamespaceCanaryMetricsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	metrics, err := flagger.GetNamespaceCanaryMetrics(req.Namespace, timeRange)
	if err != nil {
		l.Errorf("获取 Namespace Canary 指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespaceCanaryMetricsResponse{
		Data: convertNamespaceCanaryMetrics(metrics),
	}

	l.Infof("获取 Namespace %s Canary 指标成功", req.Namespace)
	return resp, nil
}
