package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetIngressRateLimitLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Ingress 限流指标
func NewGetIngressRateLimitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetIngressRateLimitLogic {
	return &GetIngressRateLimitLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetIngressRateLimitLogic) GetIngressRateLimit(req *types.GetIngressRateLimitRequest) (resp *types.GetIngressRateLimitResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	rateLimit, err := ingress.GetIngressRateLimit(req.Namespace, req.IngressName, timeRange)
	if err != nil {
		l.Errorf("获取 Ingress 限流指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetIngressRateLimitResponse{
		Data: convertIngressRateLimitMetrics(rateLimit),
	}

	l.Infof("获取 Ingress %s/%s 限流指标成功", req.Namespace, req.IngressName)
	return resp, nil
}
