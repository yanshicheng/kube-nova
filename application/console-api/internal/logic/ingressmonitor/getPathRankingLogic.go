package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetPathRankingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Path 排行
func NewGetPathRankingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPathRankingLogic {
	return &GetPathRankingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPathRankingLogic) GetPathRanking(req *types.GetPathRankingRequest) (resp *types.GetPathRankingResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	ranking, err := ingress.GetPathRanking(req.Limit, timeRange)
	if err != nil {
		l.Errorf("获取 Path 排行失败: %v", err)
		return nil, err
	}

	resp = &types.GetPathRankingResponse{
		Data: convertPathRanking(ranking),
	}

	l.Infof("获取 Path Top %d 排行成功", req.Limit)
	return resp, nil
}
