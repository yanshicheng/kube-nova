package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetIngressStatusCodesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Ingress 状态码分布
func NewGetIngressStatusCodesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetIngressStatusCodesLogic {
	return &GetIngressStatusCodesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetIngressStatusCodesLogic) GetIngressStatusCodes(req *types.GetIngressStatusCodesRequest) (resp *types.GetIngressStatusCodesResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	statusCodes, err := ingress.GetIngressStatusCodes(req.Namespace, req.IngressName, timeRange)
	if err != nil {
		l.Errorf("获取 Ingress 状态码分布失败: %v", err)
		return nil, err
	}

	resp = &types.GetIngressStatusCodesResponse{
		Data: convertIngressStatusCodeDistribution(statusCodes),
	}

	l.Infof("获取 Ingress %s/%s 状态码分布成功", req.Namespace, req.IngressName)
	return resp, nil
}
