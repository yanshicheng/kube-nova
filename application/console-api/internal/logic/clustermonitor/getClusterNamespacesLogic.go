package clustermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterNamespacesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取集群 Namespace 统计
func NewGetClusterNamespacesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterNamespacesLogic {
	return &GetClusterNamespacesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterNamespacesLogic) GetClusterNamespaces(req *types.GetClusterNamespacesRequest) (resp *types.GetClusterNamespacesResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	cluster := client.Cluster()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	namespaces, err := cluster.GetClusterNamespaces(timeRange)
	if err != nil {
		l.Errorf("获取集群 Namespace 统计失败: %v", err)
		return nil, err
	}

	resp = &types.GetClusterNamespacesResponse{
		Data: convertClusterNamespaceMetrics(namespaces),
	}

	l.Infof("获取集群 Namespace 统计成功: Total=%d", namespaces.Total)
	return resp, nil
}
