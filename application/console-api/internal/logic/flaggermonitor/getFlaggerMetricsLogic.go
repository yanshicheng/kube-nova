package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetFlaggerMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Flagger 综合指标
func NewGetFlaggerMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFlaggerMetricsLogic {
	return &GetFlaggerMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetFlaggerMetricsLogic) GetFlaggerMetrics(req *types.GetFlaggerMetricsRequest) (resp *types.GetFlaggerMetricsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	metrics, err := flagger.GetFlaggerMetrics(timeRange)
	if err != nil {
		l.Errorf("获取 Flagger 综合指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetFlaggerMetricsResponse{
		Data: convertFlaggerMetrics(metrics),
	}

	l.Infof("获取 Flagger 综合指标成功")
	return resp, nil
}
