package clustermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetEtcdMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Etcd 指标
func NewGetEtcdMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetEtcdMetricsLogic {
	return &GetEtcdMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetEtcdMetricsLogic) GetEtcdMetrics(req *types.GetEtcdMetricsRequest) (resp *types.GetEtcdMetricsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	cluster := client.Cluster()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	etcd, err := cluster.GetEtcdMetrics(timeRange)
	if err != nil {
		l.Errorf("获取 Etcd 指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetEtcdMetricsResponse{
		Data: convertEtcdMetrics(etcd),
	}

	l.Infof("获取 Etcd 指标成功")
	return resp, nil
}
