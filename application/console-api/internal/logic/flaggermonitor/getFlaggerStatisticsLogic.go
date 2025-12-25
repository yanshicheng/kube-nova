package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetFlaggerStatisticsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Flagger 统计信息
func NewGetFlaggerStatisticsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFlaggerStatisticsLogic {
	return &GetFlaggerStatisticsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetFlaggerStatisticsLogic) GetFlaggerStatistics(req *types.GetFlaggerStatisticsRequest) (resp *types.GetFlaggerStatisticsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	statistics, err := flagger.GetFlaggerStatistics(timeRange)
	if err != nil {
		l.Errorf("获取 Flagger 统计信息失败: %v", err)
		return nil, err
	}

	resp = &types.GetFlaggerStatisticsResponse{
		Data: convertFlaggerStatistics(statistics),
	}

	l.Infof("获取 Flagger 统计信息成功")
	return resp, nil
}
