package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type IngressClassGetUsageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 IngressClass 使用情况（哪些 Ingress 使用）
func NewIngressClassGetUsageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngressClassGetUsageLogic {
	return &IngressClassGetUsageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngressClassGetUsageLogic) IngressClassGetUsage(req *types.IngressClassNameRequest) (resp *types.IngressClassUsageResponse, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	icOp := client.IngressClasses()
	result, err := icOp.GetUsage(req.Name)
	if err != nil {
		l.Errorf("获取 IngressClass 使用情况失败: %v", err)
		return nil, fmt.Errorf("获取 IngressClass 使用情况失败")
	}

	resp = &types.IngressClassUsageResponse{
		IngressClassName: result.IngressClassName,
		IsDefault:        result.IsDefault,
		IngressCount:     result.IngressCount,
		NamespaceCount:   result.NamespaceCount,
		Ingresses:        make([]types.IngressRefInfo, 0, len(result.Ingresses)),
		NamespaceStats:   result.NamespaceStats,
		CanDelete:        result.CanDelete,
		DeleteWarning:    result.DeleteWarning,
	}

	// 转换 Ingresses
	for _, ing := range result.Ingresses {
		resp.Ingresses = append(resp.Ingresses, types.IngressRefInfo{
			Name:         ing.Name,
			Namespace:    ing.Namespace,
			Hosts:        ing.Hosts,
			Address:      ing.Address,
			TLSEnabled:   ing.TLSEnabled,
			RulesCount:   ing.RulesCount,
			BackendCount: ing.BackendCount,
			Age:          ing.Age,
		})
	}

	return resp, nil
}
