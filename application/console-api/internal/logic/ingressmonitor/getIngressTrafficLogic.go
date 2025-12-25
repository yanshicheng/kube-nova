package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetIngressTrafficLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Ingress 流量指标
func NewGetIngressTrafficLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetIngressTrafficLogic {
	return &GetIngressTrafficLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetIngressTrafficLogic) GetIngressTraffic(req *types.GetIngressTrafficRequest) (resp *types.GetIngressTrafficResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	traffic, err := ingress.GetIngressTraffic(req.Namespace, req.IngressName, timeRange)
	if err != nil {
		l.Errorf("获取 Ingress 流量指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetIngressTrafficResponse{
		Data: convertIngressTrafficMetrics(traffic),
	}

	l.Infof("获取 Ingress %s/%s 流量指标成功", req.Namespace, req.IngressName)
	return resp, nil
}
