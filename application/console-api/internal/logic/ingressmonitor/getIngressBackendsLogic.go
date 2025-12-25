package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetIngressBackendsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Ingress 后端健康状态
func NewGetIngressBackendsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetIngressBackendsLogic {
	return &GetIngressBackendsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetIngressBackendsLogic) GetIngressBackends(req *types.GetIngressBackendsRequest) (resp *types.GetIngressBackendsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	backends, err := ingress.GetIngressBackends(req.Namespace, req.IngressName, timeRange)
	if err != nil {
		l.Errorf("获取 Ingress 后端健康状态失败: %v", err)
		return nil, err
	}

	resp = &types.GetIngressBackendsResponse{
		Data: convertIngressBackendMetrics(backends),
	}

	l.Infof("获取 Ingress %s/%s 后端健康状态成功", req.Namespace, req.IngressName)
	return resp, nil
}
