package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetCanaryAnalysisLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Canary 分析结果
func NewGetCanaryAnalysisLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCanaryAnalysisLogic {
	return &GetCanaryAnalysisLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCanaryAnalysisLogic) GetCanaryAnalysis(req *types.GetCanaryAnalysisRequest) (resp *types.GetCanaryAnalysisResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	analysis, err := flagger.GetCanaryAnalysis(req.Namespace, req.Name, timeRange)
	if err != nil {
		l.Errorf("获取 Canary 分析结果失败: %v", err)
		return nil, err
	}

	resp = &types.GetCanaryAnalysisResponse{
		Data: convertCanaryAnalysis(analysis),
	}

	l.Infof("获取 Canary %s/%s 分析结果成功", req.Namespace, req.Name)
	return resp, nil
}
