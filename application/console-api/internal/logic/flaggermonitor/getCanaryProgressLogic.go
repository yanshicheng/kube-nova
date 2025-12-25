package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetCanaryProgressLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Canary 进度
func NewGetCanaryProgressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCanaryProgressLogic {
	return &GetCanaryProgressLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCanaryProgressLogic) GetCanaryProgress(req *types.GetCanaryProgressRequest) (resp *types.GetCanaryProgressResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	progress, err := flagger.GetCanaryProgress(req.Namespace, req.Name, timeRange)
	if err != nil {
		l.Errorf("获取 Canary 进度失败: %v", err)
		return nil, err
	}

	resp = &types.GetCanaryProgressResponse{
		Data: convertCanaryProgress(progress),
	}

	l.Infof("获取 Canary %s/%s 进度成功", req.Namespace, req.Name)
	return resp, nil
}
