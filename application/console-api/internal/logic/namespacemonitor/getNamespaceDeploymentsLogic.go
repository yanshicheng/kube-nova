package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceDeploymentsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace Deployment 统计
func NewGetNamespaceDeploymentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceDeploymentsLogic {
	return &GetNamespaceDeploymentsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceDeploymentsLogic) GetNamespaceDeployments(req *types.GetNamespaceDeploymentsRequest) (resp *types.GetNamespaceDeploymentsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()

	metrics, err := namespace.GetNamespaceDeployments(req.Namespace)
	if err != nil {
		l.Errorf("获取 Namespace Deployments 失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespaceDeploymentsResponse{
		Data: convertNamespaceDeploymentStatistics(metrics),
	}

	l.Infof("获取 Namespace %s Deployments 成功", req.Namespace)
	return resp, nil
}
