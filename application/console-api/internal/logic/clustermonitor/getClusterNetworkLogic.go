package clustermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterNetworkLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取集群网络统计
func NewGetClusterNetworkLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterNetworkLogic {
	return &GetClusterNetworkLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterNetworkLogic) GetClusterNetwork(req *types.GetClusterNetworkRequest) (resp *types.GetClusterNetworkResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	cluster := client.Cluster()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	network, err := cluster.GetClusterNetwork(timeRange)
	if err != nil {
		l.Errorf("获取集群网络统计失败: %v", err)
		return nil, err
	}

	resp = &types.GetClusterNetworkResponse{
		Data: convertClusterNetworkMetrics(network),
	}

	l.Infof("获取集群网络统计成功")
	return resp, nil
}
