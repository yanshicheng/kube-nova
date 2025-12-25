package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetIngressObjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Ingress 对象信息
func NewGetIngressObjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetIngressObjectLogic {
	return &GetIngressObjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetIngressObjectLogic) GetIngressObject(req *types.GetIngressObjectRequest) (resp *types.GetIngressObjectResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()

	object, err := ingress.GetIngressObject(req.Namespace, req.IngressName)
	if err != nil {
		l.Errorf("获取 Ingress 对象信息失败: %v", err)
		return nil, err
	}

	resp = &types.GetIngressObjectResponse{
		Data: convertIngressObjectMetrics(object),
	}

	l.Infof("获取 Ingress %s/%s 对象信息成功", req.Namespace, req.IngressName)
	return resp, nil
}
