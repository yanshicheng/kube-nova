package clustermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterOverviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetClusterOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterOverviewLogic {
	return &GetClusterOverviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterOverviewLogic) GetClusterOverview(req *types.GetClusterOverviewRequest) (resp *types.GetClusterOverviewResponse, err error) {
	// 获取 Prometheus 客户端
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	// 获取 Cluster 操作器
	cluster := client.Cluster()

	// 解析时间范围
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	// 获取集群概览
	overview, err := cluster.GetClusterOverview(timeRange)
	if err != nil {
		l.Errorf("获取集群概览失败: %v", err)
		return nil, err
	}

	// 转换响应
	resp = &types.GetClusterOverviewResponse{
		Data: convertClusterOverview(overview),
	}

	l.Infof("获取集群概览成功: Cluster=%s", overview.ClusterName)
	return resp, nil
}
