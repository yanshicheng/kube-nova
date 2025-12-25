package clustermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetControllerManagerMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Controller Manager 指标
func NewGetControllerManagerMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetControllerManagerMetricsLogic {
	return &GetControllerManagerMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetControllerManagerMetricsLogic) GetControllerManagerMetrics(req *types.GetControllerManagerMetricsRequest) (resp *types.GetControllerManagerMetricsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	cluster := client.Cluster()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	cm, err := cluster.GetControllerManagerMetrics(timeRange)
	if err != nil {
		l.Errorf("获取 Controller Manager 指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetControllerManagerMetricsResponse{
		Data: convertControllerManagerMetrics(cm),
	}

	l.Infof("获取 Controller Manager 指标成功")
	return resp, nil
}
