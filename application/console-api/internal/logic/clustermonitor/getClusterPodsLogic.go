package clustermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterPodsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取集群 Pod 统计
func NewGetClusterPodsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterPodsLogic {
	return &GetClusterPodsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterPodsLogic) GetClusterPods(req *types.GetClusterPodsRequest) (resp *types.GetClusterPodsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	cluster := client.Cluster()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	pods, err := cluster.GetClusterPods(timeRange)
	if err != nil {
		l.Errorf("获取集群 Pod 统计失败: %v", err)
		return nil, err
	}

	resp = &types.GetClusterPodsResponse{
		Data: convertClusterPodMetrics(pods),
	}

	l.Infof("获取集群 Pod 统计成功: Total=%d, Running=%d", pods.Total, pods.Running)
	return resp, nil
}
