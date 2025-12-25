package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetIngressErrorsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Ingress 错误指标
func NewGetIngressErrorsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetIngressErrorsLogic {
	return &GetIngressErrorsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetIngressErrorsLogic) GetIngressErrors(req *types.GetIngressErrorsRequest) (resp *types.GetIngressErrorsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	errors, err := ingress.GetIngressErrors(req.Namespace, req.IngressName, timeRange)
	if err != nil {
		l.Errorf("获取 Ingress 错误指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetIngressErrorsResponse{
		Data: convertIngressErrorMetrics(errors),
	}

	l.Infof("获取 Ingress %s/%s 错误指标成功", req.Namespace, req.IngressName)
	return resp, nil
}
