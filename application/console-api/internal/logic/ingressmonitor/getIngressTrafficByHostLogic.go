package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetIngressTrafficByHostLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 按 Host 获取流量指标
func NewGetIngressTrafficByHostLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetIngressTrafficByHostLogic {
	return &GetIngressTrafficByHostLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetIngressTrafficByHostLogic) GetIngressTrafficByHost(req *types.GetIngressTrafficByHostRequest) (resp *types.GetIngressTrafficByHostResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	traffic, err := ingress.GetIngressTrafficByHost(req.Host, timeRange)
	if err != nil {
		l.Errorf("按 Host 获取流量指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetIngressTrafficByHostResponse{
		Data: convertIngressTrafficMetrics(traffic),
	}

	l.Infof("按 Host %s 获取流量指标成功", req.Host)
	return resp, nil
}
