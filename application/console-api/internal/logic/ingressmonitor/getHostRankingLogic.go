package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetHostRankingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Host 排行
func NewGetHostRankingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetHostRankingLogic {
	return &GetHostRankingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetHostRankingLogic) GetHostRanking(req *types.GetHostRankingRequest) (resp *types.GetHostRankingResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	ranking, err := ingress.GetHostRanking(req.Limit, timeRange)
	if err != nil {
		l.Errorf("获取 Host 排行失败: %v", err)
		return nil, err
	}

	resp = &types.GetHostRankingResponse{
		Data: convertHostRanking(ranking),
	}

	l.Infof("获取 Host Top %d 排行成功", req.Limit)
	return resp, nil
}
