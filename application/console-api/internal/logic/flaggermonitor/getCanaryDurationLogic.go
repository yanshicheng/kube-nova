package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetCanaryDurationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Canary 持续时间
func NewGetCanaryDurationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCanaryDurationLogic {
	return &GetCanaryDurationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCanaryDurationLogic) GetCanaryDuration(req *types.GetCanaryDurationRequest) (resp *types.GetCanaryDurationResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()

	duration, err := flagger.GetCanaryDuration(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Canary 持续时间失败: %v", err)
		return nil, err
	}

	resp = &types.GetCanaryDurationResponse{
		Data: convertCanaryDuration(duration),
	}

	l.Infof("获取 Canary %s/%s 持续时间成功", req.Namespace, req.Name)
	return resp, nil
}
