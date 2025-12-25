package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetIngressLatencyByHostLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 按 Host 获取延迟指标
func NewGetIngressLatencyByHostLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetIngressLatencyByHostLogic {
	return &GetIngressLatencyByHostLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetIngressLatencyByHostLogic) GetIngressLatencyByHost(req *types.GetIngressLatencyByHostRequest) (resp *types.GetIngressLatencyByHostResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	latency, err := ingress.GetIngressLatencyByHost(req.Host, timeRange)
	if err != nil {
		l.Errorf("按 Host 获取延迟指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetIngressLatencyByHostResponse{
		Data: convertIngressLatencyStats(latency),
	}

	l.Infof("按 Host %s 获取延迟指标成功", req.Host)
	return resp, nil
}
