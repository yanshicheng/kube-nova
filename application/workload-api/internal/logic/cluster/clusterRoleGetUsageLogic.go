package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterRoleGetUsageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 ClusterRole 使用情况（被哪些绑定引用）
func NewClusterRoleGetUsageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleGetUsageLogic {
	return &ClusterRoleGetUsageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleGetUsageLogic) ClusterRoleGetUsage(req *types.ClusterResourceNameRequest) (resp *types.ClusterRoleUsageResponse, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	crOp := client.ClusterRoles()
	result, err := crOp.GetUsage(req.Name)
	if err != nil {
		l.Errorf("获取 ClusterRole 使用情况失败: %v", err)
		return nil, fmt.Errorf("获取 ClusterRole 使用情况失败")
	}

	resp = &types.ClusterRoleUsageResponse{
		ClusterRoleName:     result.ClusterRoleName,
		BindingCount:        result.BindingCount,
		RoleBindingCount:    result.RoleBindingCount,
		ClusterRoleBindings: make([]types.ClusterRoleBindingRef, 0, len(result.ClusterRoleBindings)),
		RoleBindings:        make([]types.RoleBindingRef, 0, len(result.RoleBindings)),
		TotalSubjects:       result.TotalSubjects,
		ServiceAccountCount: result.ServiceAccountCount,
		UserCount:           result.UserCount,
		GroupCount:          result.GroupCount,
		CanDelete:           result.CanDelete,
		DeleteWarning:       result.DeleteWarning,
	}

	// 转换 ClusterRoleBindings
	for _, crb := range result.ClusterRoleBindings {
		crbRef := types.ClusterRoleBindingRef{
			Name:     crb.Name,
			Subjects: make([]types.SubjectInfo, 0, len(crb.Subjects)),
			Age:      crb.Age,
		}
		for _, s := range crb.Subjects {
			crbRef.Subjects = append(crbRef.Subjects, types.SubjectInfo{
				Kind:      s.Kind,
				Name:      s.Name,
				Namespace: s.Namespace,
				APIGroup:  s.APIGroup,
			})
		}
		resp.ClusterRoleBindings = append(resp.ClusterRoleBindings, crbRef)
	}

	// 转换 RoleBindings
	for _, rb := range result.RoleBindings {
		rbRef := types.RoleBindingRef{
			Name:      rb.Name,
			Namespace: rb.Namespace,
			Subjects:  make([]types.SubjectInfo, 0, len(rb.Subjects)),
			Age:       rb.Age,
		}
		for _, s := range rb.Subjects {
			rbRef.Subjects = append(rbRef.Subjects, types.SubjectInfo{
				Kind:      s.Kind,
				Name:      s.Name,
				Namespace: s.Namespace,
				APIGroup:  s.APIGroup,
			})
		}
		resp.RoleBindings = append(resp.RoleBindings, rbRef)
	}

	return resp, nil
}
