package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetCanaryHistoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Canary 历史记录
func NewGetCanaryHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCanaryHistoryLogic {
	return &GetCanaryHistoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCanaryHistoryLogic) GetCanaryHistory(req *types.GetCanaryHistoryRequest) (resp *types.GetCanaryHistoryResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	history, err := flagger.GetCanaryHistory(req.Namespace, req.Name, timeRange)
	if err != nil {
		l.Errorf("获取 Canary 历史记录失败: %v", err)
		return nil, err
	}

	resp = &types.GetCanaryHistoryResponse{
		Data: convertCanaryHistory(history),
	}

	l.Infof("获取 Canary %s/%s 历史记录成功，共 %d 条", req.Namespace, req.Name, len(history.Records))
	return resp, nil
}
