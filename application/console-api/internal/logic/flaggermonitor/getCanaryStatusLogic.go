package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetCanaryStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Canary 状态
func NewGetCanaryStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCanaryStatusLogic {
	return &GetCanaryStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCanaryStatusLogic) GetCanaryStatus(req *types.GetCanaryStatusRequest) (resp *types.GetCanaryStatusResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()

	status, err := flagger.GetCanaryStatus(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Canary 状态失败: %v", err)
		return nil, err
	}

	resp = &types.GetCanaryStatusResponse{
		Data: convertCanaryStatus(status),
	}

	l.Infof("获取 Canary %s/%s 状态成功", req.Namespace, req.Name)
	return resp, nil
}
