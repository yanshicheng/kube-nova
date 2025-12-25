package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterRoleBindingGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 ClusterRoleBinding 详情
func NewClusterRoleBindingGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleBindingGetLogic {
	return &ClusterRoleBindingGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleBindingGetLogic) ClusterRoleBindingGet(req *types.ClusterResourceNameRequest) (resp *types.ClusterRoleBindingDetail, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	crbOp := client.ClusterRoleBindings()
	crb, err := crbOp.Get(req.Name)
	if err != nil {
		l.Errorf("获取 ClusterRoleBinding 详情失败: %v", err)
		return nil, fmt.Errorf("获取 ClusterRoleBinding 详情失败")
	}

	resp = &types.ClusterRoleBindingDetail{
		Name: crb.Name,
		RoleRef: types.RoleRefInfo{
			Kind:     crb.RoleRef.Kind,
			Name:     crb.RoleRef.Name,
			APIGroup: crb.RoleRef.APIGroup,
		},
		Subjects:          make([]types.SubjectInfo, 0, len(crb.Subjects)),
		Labels:            crb.Labels,
		Annotations:       crb.Annotations,
		Age:               formatAge(crb.CreationTimestamp.Time),
		CreationTimestamp: crb.CreationTimestamp.UnixMilli(),
	}

	for _, s := range crb.Subjects {
		resp.Subjects = append(resp.Subjects, types.SubjectInfo{
			Kind:      s.Kind,
			Name:      s.Name,
			Namespace: s.Namespace,
			APIGroup:  s.APIGroup,
		})
	}

	return resp, nil
}
