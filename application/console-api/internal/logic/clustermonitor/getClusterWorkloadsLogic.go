package clustermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterWorkloadsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取集群工作负载统计
func NewGetClusterWorkloadsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterWorkloadsLogic {
	return &GetClusterWorkloadsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterWorkloadsLogic) GetClusterWorkloads(req *types.GetClusterWorkloadsRequest) (resp *types.GetClusterWorkloadsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	cluster := client.Cluster()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	workloads, err := cluster.GetClusterWorkloads(timeRange)
	if err != nil {
		l.Errorf("获取集群工作负载统计失败: %v", err)
		return nil, err
	}

	resp = &types.GetClusterWorkloadsResponse{
		Data: convertClusterWorkloadMetrics(workloads),
	}

	l.Infof("获取集群工作负载统计成功")
	return resp, nil
}
