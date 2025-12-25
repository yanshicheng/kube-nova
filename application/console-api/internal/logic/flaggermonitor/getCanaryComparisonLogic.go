package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetCanaryComparisonLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Canary 对比
func NewGetCanaryComparisonLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCanaryComparisonLogic {
	return &GetCanaryComparisonLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCanaryComparisonLogic) GetCanaryComparison(req *types.GetCanaryComparisonRequest) (resp *types.GetCanaryComparisonResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	comparison, err := flagger.GetCanaryComparison(req.Namespace, req.Name, timeRange)
	if err != nil {
		l.Errorf("获取 Canary 对比失败: %v", err)
		return nil, err
	}

	resp = &types.GetCanaryComparisonResponse{
		Data: convertCanaryComparison(comparison),
	}

	l.Infof("获取 Canary %s/%s 对比成功", req.Namespace, req.Name)
	return resp, nil
}
