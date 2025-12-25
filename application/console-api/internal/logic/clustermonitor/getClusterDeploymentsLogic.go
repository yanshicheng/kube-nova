package clustermonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterDeploymentsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取集群 Deployment 统计
func NewGetClusterDeploymentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterDeploymentsLogic {
	return &GetClusterDeploymentsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterDeploymentsLogic) GetClusterDeployments(req *types.GetClusterDeploymentsRequest) (resp *types.GetClusterDeploymentsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	cluster := client.Cluster()

	deployments, err := cluster.GetClusterDeployments()
	if err != nil {
		l.Errorf("获取集群 Deployment 统计失败: %v", err)
		return nil, err
	}

	resp = &types.GetClusterDeploymentsResponse{
		Data: types.ClusterDeploymentMetrics{
			Total:               deployments.Total,
			AvailableReplicas:   deployments.AvailableReplicas,
			UnavailableReplicas: deployments.UnavailableReplicas,
			Updating:            deployments.Updating,
		},
	}

	l.Infof("获取集群 Deployment 统计成功: Total=%d", deployments.Total)
	return resp, nil
}
