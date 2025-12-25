package clustermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterResourcesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取集群资源指标
func NewGetClusterResourcesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterResourcesLogic {
	return &GetClusterResourcesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterResourcesLogic) GetClusterResources(req *types.GetClusterResourcesRequest) (resp *types.GetClusterResourcesResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	cluster := client.Cluster()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	resources, err := cluster.GetClusterResources(timeRange)
	if err != nil {
		l.Errorf("获取集群资源指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetClusterResourcesResponse{
		Data: convertClusterResourceMetrics(resources),
	}

	l.Infof("获取集群资源指标成功")
	return resp, nil
}
