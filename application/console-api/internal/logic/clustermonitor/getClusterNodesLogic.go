package clustermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterNodesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取集群节点统计
func NewGetClusterNodesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterNodesLogic {
	return &GetClusterNodesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterNodesLogic) GetClusterNodes(req *types.GetClusterNodesRequest) (resp *types.GetClusterNodesResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	cluster := client.Cluster()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	metrics, err := cluster.GetClusterNodes(timeRange)
	if err != nil {
		l.Errorf("获取集群节点统计失败: %v", err)
		return nil, err
	}

	resp = &types.GetClusterNodesResponse{
		Data: convertClusterNodeMetrics(metrics),
	}

	l.Infof("获取集群节点统计成功: Total=%d, Ready=%d", metrics.Total, metrics.Ready)
	return resp, nil
}
