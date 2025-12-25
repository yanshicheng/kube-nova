package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetControllerHealthLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Ingress Controller 健康状态
func NewGetControllerHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetControllerHealthLogic {
	return &GetControllerHealthLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetControllerHealthLogic) GetControllerHealth(req *types.GetControllerHealthRequest) (resp *types.GetControllerHealthResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	health, err := ingress.GetControllerHealth(timeRange)
	if err != nil {
		l.Errorf("获取 Ingress Controller 健康状态失败: %v", err)
		return nil, err
	}

	resp = &types.GetControllerHealthResponse{
		Data: convertIngressControllerHealth(health),
	}

	l.Infof("获取 Ingress Controller 健康状态成功")
	return resp, nil
}
