package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetResourceQuotaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取命名空间资源配额
func NewGetResourceQuotaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetResourceQuotaLogic {
	return &GetResourceQuotaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetResourceQuotaLogic) GetResourceQuota(req *types.GetResourceQuotaRequest) (resp *types.GetResourceQuotaResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	metrics, err := pod.GetResourceQuota(req.Namespace)
	if err != nil {
		l.Errorf("获取资源配额失败: Namespace=%s, Error=%v", req.Namespace, err)
		return nil, err
	}

	resp = &types.GetResourceQuotaResponse{
		Data: types.ResourceQuotaMetrics{
			Namespace:     metrics.Namespace,
			QuotaName:     metrics.QuotaName,
			CPUUsed:       metrics.CPUUsed,
			CPUHard:       metrics.CPUHard,
			CPUPercent:    metrics.CPUPercent,
			MemoryUsed:    metrics.MemoryUsed,
			MemoryHard:    metrics.MemoryHard,
			MemoryPercent: metrics.MemoryPercent,
			PodsUsed:      metrics.PodsUsed,
			PodsHard:      metrics.PodsHard,
			PodsPercent:   metrics.PodsPercent,
			Resources:     make([]types.ResourceQuotaDetail, 0),
		},
	}

	for _, resource := range metrics.Resources {
		resp.Data.Resources = append(resp.Data.Resources, types.ResourceQuotaDetail{
			Resource: resource.Resource,
			Used:     resource.Used,
			Hard:     resource.Hard,
			Percent:  resource.Percent,
		})
	}

	l.Infof("获取资源配额成功: Namespace=%s, CPU=%.2f%%, Memory=%.2f%%",
		req.Namespace, metrics.CPUPercent, metrics.MemoryPercent)
	return resp, nil
}
