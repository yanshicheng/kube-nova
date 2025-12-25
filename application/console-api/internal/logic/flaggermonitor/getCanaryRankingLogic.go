package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetCanaryRankingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Canary 排行
func NewGetCanaryRankingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCanaryRankingLogic {
	return &GetCanaryRankingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCanaryRankingLogic) GetCanaryRanking(req *types.GetCanaryRankingRequest) (resp *types.GetCanaryRankingResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	ranking, err := flagger.GetCanaryRanking(req.Limit, timeRange)
	if err != nil {
		l.Errorf("获取 Canary 排行失败: %v", err)
		return nil, err
	}

	resp = &types.GetCanaryRankingResponse{
		Data: convertCanaryRanking(ranking),
	}

	l.Infof("获取 Canary Top %d 排行成功", req.Limit)
	return resp, nil
}
