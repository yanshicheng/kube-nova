package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetIngressLatencyByPathLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 按 Path 获取延迟指标
func NewGetIngressLatencyByPathLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetIngressLatencyByPathLogic {
	return &GetIngressLatencyByPathLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetIngressLatencyByPathLogic) GetIngressLatencyByPath(req *types.GetIngressLatencyByPathRequest) (resp *types.GetIngressLatencyByPathResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	latency, err := ingress.GetIngressLatencyByPath(req.Path, timeRange)
	if err != nil {
		l.Errorf("按 Path 获取延迟指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetIngressLatencyByPathResponse{
		Data: convertIngressLatencyStats(latency),
	}

	l.Infof("按 Path %s 获取延迟指标成功", req.Path)
	return resp, nil
}
