package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterRoleListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 ClusterRole 列表
func NewClusterRoleListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleListLogic {
	return &ClusterRoleListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleListLogic) ClusterRoleList(req *types.ClusterResourceListRequest) (resp *types.ClusterRoleListResponse, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	crOp := client.ClusterRoles()
	result, err := crOp.List(req.Search, req.LabelSelector)
	if err != nil {
		l.Errorf("获取 ClusterRole 列表失败: %v", err)
		return nil, fmt.Errorf("获取 ClusterRole 列表失败")
	}

	resp = &types.ClusterRoleListResponse{
		Total: result.Total,
		Items: make([]types.ClusterRoleListItem, 0, len(result.Items)),
	}

	for _, item := range result.Items {
		resp.Items = append(resp.Items, types.ClusterRoleListItem{
			Name:              item.Name,
			RuleCount:         item.RuleCount,
			AggregationLabels: item.AggregationLabels,
			Age:               item.Age,
			CreationTimestamp: item.CreationTimestamp,
			Labels:            item.Labels,
			Annotations:       item.Annotations,
		})
	}

	return resp, nil
}
