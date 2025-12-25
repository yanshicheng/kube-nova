package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetIngressPerformanceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Ingress 性能指标
func NewGetIngressPerformanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetIngressPerformanceLogic {
	return &GetIngressPerformanceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetIngressPerformanceLogic) GetIngressPerformance(req *types.GetIngressPerformanceRequest) (resp *types.GetIngressPerformanceResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	performance, err := ingress.GetIngressPerformance(req.Namespace, req.IngressName, timeRange)
	if err != nil {
		l.Errorf("获取 Ingress 性能指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetIngressPerformanceResponse{
		Data: convertIngressPerformanceMetrics(performance),
	}

	l.Infof("获取 Ingress %s/%s 性能指标成功", req.Namespace, req.IngressName)
	return resp, nil
}
