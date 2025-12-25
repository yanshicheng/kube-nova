package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceQuotaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace 资源配额
func NewGetNamespaceQuotaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceQuotaLogic {
	return &GetNamespaceQuotaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceQuotaLogic) GetNamespaceQuota(req *types.GetNamespaceQuotaRequest) (resp *types.GetNamespaceQuotaResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()

	metrics, err := namespace.GetNamespaceQuota(req.Namespace)
	if err != nil {
		l.Errorf("获取 Namespace Quota 失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespaceQuotaResponse{
		Data: convertNamespaceQuotaMetrics(metrics),
	}

	l.Infof("获取 Namespace %s Quota 成功", req.Namespace)
	return resp, nil
}
