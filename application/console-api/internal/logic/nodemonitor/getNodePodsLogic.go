package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodePodsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点上的 Pod 信息
func NewGetNodePodsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodePodsLogic {
	return &GetNodePodsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodePodsLogic) GetNodePods(req *types.GetNodePodsRequest) (resp *types.GetNodePodsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	pods, err := node.GetNodePods(req.NodeName, timeRange)
	if err != nil {
		l.Errorf("获取节点 Pod 信息失败: Node=%s, Error=%v", req.NodeName, err)
		return nil, err
	}

	resp = &types.GetNodePodsResponse{
		Data: convertNodePodMetrics(pods),
	}

	l.Infof("获取节点 Pod 信息成功: Node=%s, Total=%d", req.NodeName, pods.TotalPods)
	return resp, nil
}
