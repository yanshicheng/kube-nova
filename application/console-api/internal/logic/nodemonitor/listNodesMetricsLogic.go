package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListNodesMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列出所有节点指标
func NewListNodesMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListNodesMetricsLogic {
	return &ListNodesMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListNodesMetricsLogic) ListNodesMetrics(req *types.ListNodesMetricsRequest) (resp *types.ListNodesMetricsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	nodes, err := node.ListNodesMetrics(timeRange)
	if err != nil {
		l.Errorf("列出所有节点指标失败: %v", err)
		return nil, err
	}

	resp = &types.ListNodesMetricsResponse{}
	for _, n := range nodes {
		resp.Data = append(resp.Data, convertNodeMetrics(&n))
	}

	l.Infof("列出所有节点指标成功: Count=%d", len(nodes))
	return resp, nil
}
