package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetIngressTrafficByPathLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 按 Path 获取流量指标
func NewGetIngressTrafficByPathLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetIngressTrafficByPathLogic {
	return &GetIngressTrafficByPathLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetIngressTrafficByPathLogic) GetIngressTrafficByPath(req *types.GetIngressTrafficByPathRequest) (resp *types.GetIngressTrafficByPathResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	traffic, err := ingress.GetIngressTrafficByPath(req.Path, timeRange)
	if err != nil {
		l.Errorf("按 Path 获取流量指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetIngressTrafficByPathResponse{
		Data: convertIngressTrafficMetrics(traffic),
	}

	l.Infof("按 Path %s 获取流量指标成功", req.Path)
	return resp, nil
}
