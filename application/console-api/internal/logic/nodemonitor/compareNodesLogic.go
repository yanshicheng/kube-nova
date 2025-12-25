package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CompareNodesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 对比多个节点
func NewCompareNodesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompareNodesLogic {
	return &CompareNodesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CompareNodesLogic) CompareNodes(req *types.CompareNodesRequest) (resp *types.CompareNodesResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	comparison, err := node.CompareNodes(req.NodeNames, timeRange)
	if err != nil {
		l.Errorf("对比节点失败: NodeCount=%d, Error=%v", len(req.NodeNames), err)
		return nil, err
	}

	resp = &types.CompareNodesResponse{
		Data: convertNodeComparison(comparison),
	}

	l.Infof("对比节点成功: NodeCount=%d", len(req.NodeNames))
	return resp, nil
}
