package ingressmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetExpiringCertificatesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取即将过期的证书
func NewGetExpiringCertificatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetExpiringCertificatesLogic {
	return &GetExpiringCertificatesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetExpiringCertificatesLogic) GetExpiringCertificates(req *types.GetExpiringCertificatesRequest) (resp *types.GetExpiringCertificatesResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	ingress := client.Ingress()

	certificates, err := ingress.GetExpiringCertificates(req.DaysThreshold)
	if err != nil {
		l.Errorf("获取即将过期的证书失败: %v", err)
		return nil, err
	}

	resp = &types.GetExpiringCertificatesResponse{
		Data: convertCertificateInfoList(certificates),
	}

	l.Infof("获取 %d 天内即将过期的证书成功，共 %d 个", req.DaysThreshold, len(certificates))
	return resp, nil
}
