package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterRoleBindingGetBySubjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 根据主体获取绑定列表
func NewClusterRoleBindingGetBySubjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleBindingGetBySubjectLogic {
	return &ClusterRoleBindingGetBySubjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleBindingGetBySubjectLogic) ClusterRoleBindingGetBySubject(req *types.GetBySubjectRequest) (resp *types.SubjectBindingsResponse, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	crbOp := client.ClusterRoleBindings()
	result, err := crbOp.GetBySubject(req.SubjectKind, req.SubjectName, req.SubjectNamespace)
	if err != nil {
		l.Errorf("根据主体获取绑定列表失败: %v", err)
		return nil, fmt.Errorf("根据主体获取绑定列表失败")
	}

	resp = &types.SubjectBindingsResponse{
		SubjectKind:           result.SubjectKind,
		SubjectName:           result.SubjectName,
		SubjectNamespace:      result.SubjectNamespace,
		ClusterRoleBindings:   make([]types.ClusterRoleBindingListItem, 0, len(result.ClusterRoleBindings)),
		RoleBindings:          make([]types.RoleBindingInfo, 0, len(result.RoleBindings)),
		EffectiveClusterRoles: result.EffectiveClusterRoles,
		EffectiveRoles:        make([]types.NamespacedRoleInfo, 0, len(result.EffectiveRoles)),
		TotalBindings:         result.TotalBindings,
	}

	// 转换 ClusterRoleBindings - 这里 result.ClusterRoleBindings 是 []ClusterRoleBindingInfo 类型
	for _, crb := range result.ClusterRoleBindings {
		resp.ClusterRoleBindings = append(resp.ClusterRoleBindings, types.ClusterRoleBindingListItem{
			Name:              crb.Name,
			Role:              crb.Role,
			Users:             crb.Users,
			Groups:            crb.Groups,
			ServiceAccounts:   crb.ServiceAccounts,
			SubjectCount:      crb.SubjectCount,
			Age:               crb.Age,
			CreationTimestamp: crb.CreationTimestamp,
			Labels:            crb.Labels,
			Annotations:       crb.Annotations,
		})
	}

	// 转换 RoleBindings
	for _, rb := range result.RoleBindings {
		resp.RoleBindings = append(resp.RoleBindings, types.RoleBindingInfo{
			Name:      rb.Name,
			Namespace: rb.Namespace,
			RoleName:  rb.RoleName,
			RoleKind:  rb.RoleKind,
			Age:       rb.Age,
		})
	}

	// 转换 EffectiveRoles
	for _, role := range result.EffectiveRoles {
		resp.EffectiveRoles = append(resp.EffectiveRoles, types.NamespacedRoleInfo{
			RoleName:  role.RoleName,
			RoleKind:  role.RoleKind,
			Namespace: role.Namespace,
		})
	}

	return resp, nil
}
