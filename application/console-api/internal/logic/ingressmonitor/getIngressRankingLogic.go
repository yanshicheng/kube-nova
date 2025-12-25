package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetIngressRankingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Ingress 排行
func NewGetIngressRankingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetIngressRankingLogic {
	return &GetIngressRankingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetIngressRankingLogic) GetIngressRanking(req *types.GetIngressRankingRequest) (resp *types.GetIngressRankingResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	ranking, err := ingress.GetIngressRanking(req.Limit, timeRange)
	if err != nil {
		l.Errorf("获取 Ingress 排行失败: %v", err)
		return nil, err
	}

	resp = &types.GetIngressRankingResponse{
		Data: convertIngressRanking(ranking),
	}

	l.Infof("获取 Ingress Top %d 排行成功", req.Limit)
	return resp, nil
}
