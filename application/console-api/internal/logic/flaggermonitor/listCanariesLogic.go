package flaggermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListCanariesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列出 Namespace 下的 Canary
func NewListCanariesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCanariesLogic {
	return &ListCanariesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListCanariesLogic) ListCanaries(req *types.ListCanariesRequest) (resp *types.ListCanariesResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	flagger := client.Flagger()

	canaries, err := flagger.ListCanaries(req.Namespace)
	if err != nil {
		l.Errorf("列出 Namespace 下的 Canary 失败: %v", err)
		return nil, err
	}

	resp = &types.ListCanariesResponse{
		Data: convertCanaryListMetrics(canaries),
	}

	l.Infof("列出 Namespace %s 下的 Canary 成功，共 %d 个", req.Namespace, len(canaries.Canaries))
	return resp, nil
}
