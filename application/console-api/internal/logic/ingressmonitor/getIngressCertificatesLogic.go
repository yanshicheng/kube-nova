package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetIngressCertificatesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Ingress 证书信息
func NewGetIngressCertificatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetIngressCertificatesLogic {
	return &GetIngressCertificatesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetIngressCertificatesLogic) GetIngressCertificates(req *types.GetIngressCertificatesRequest) (resp *types.GetIngressCertificatesResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()

	certificates, err := ingress.GetIngressCertificates(req.Namespace, req.IngressName)
	if err != nil {
		l.Errorf("获取 Ingress 证书信息失败: %v", err)
		return nil, err
	}

	resp = &types.GetIngressCertificatesResponse{
		Data: convertIngressCertificateMetrics(certificates),
	}

	l.Infof("获取 Ingress %s/%s 证书信息成功", req.Namespace, req.IngressName)
	return resp, nil
}
