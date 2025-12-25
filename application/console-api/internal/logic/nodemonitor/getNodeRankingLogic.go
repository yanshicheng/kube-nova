package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeRankingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点排行
func NewGetNodeRankingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeRankingLogic {
	return &GetNodeRankingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeRankingLogic) GetNodeRanking(req *types.GetNodeRankingRequest) (resp *types.GetNodeRankingResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	ranking, err := node.GetNodeRanking(req.Limit, timeRange)
	if err != nil {
		l.Errorf("获取节点排行失败: Limit=%d, Error=%v", req.Limit, err)
		return nil, err
	}

	resp = &types.GetNodeRankingResponse{
		Data: convertNodeRanking(ranking),
	}

	l.Infof("获取节点排行成功: Limit=%d", req.Limit)
	return resp, nil
}
