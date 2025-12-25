package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterRoleBindingListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 ClusterRoleBinding 列表
func NewClusterRoleBindingListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleBindingListLogic {
	return &ClusterRoleBindingListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleBindingListLogic) ClusterRoleBindingList(req *types.ClusterResourceListRequest) (resp *types.ClusterRoleBindingListResponse, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	crbOp := client.ClusterRoleBindings()
	result, err := crbOp.List(req.Search, req.LabelSelector)
	if err != nil {
		l.Errorf("获取 ClusterRoleBinding 列表失败: %v", err)
		return nil, fmt.Errorf("获取 ClusterRoleBinding 列表失败")
	}

	resp = &types.ClusterRoleBindingListResponse{
		Total: result.Total,
		Items: make([]types.ClusterRoleBindingListItem, 0, len(result.Items)),
	}

	for _, item := range result.Items {
		resp.Items = append(resp.Items, types.ClusterRoleBindingListItem{
			Name:              item.Name,
			Role:              item.Role,
			Users:             item.Users,
			Groups:            item.Groups,
			ServiceAccounts:   item.ServiceAccounts,
			SubjectCount:      item.SubjectCount,
			Age:               item.Age,
			CreationTimestamp: item.CreationTimestamp,
			Labels:            item.Labels,
			Annotations:       item.Annotations,
		})
	}

	return resp, nil
}
