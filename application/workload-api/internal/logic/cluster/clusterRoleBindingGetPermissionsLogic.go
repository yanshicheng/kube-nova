package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterRoleBindingGetPermissionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 ClusterRoleBinding 实际权限
func NewClusterRoleBindingGetPermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleBindingGetPermissionsLogic {
	return &ClusterRoleBindingGetPermissionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleBindingGetPermissionsLogic) ClusterRoleBindingGetPermissions(req *types.ClusterResourceNameRequest) (resp *types.ClusterRoleBindingPermissionsResponse, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	crbOp := client.ClusterRoleBindings()
	result, err := crbOp.GetEffectivePermissions(req.Name)
	if err != nil {
		l.Errorf("获取 ClusterRoleBinding 实际权限失败: %v", err)
		return nil, fmt.Errorf("获取 ClusterRoleBinding 实际权限失败")
	}

	resp = &types.ClusterRoleBindingPermissionsResponse{
		BindingName:     result.BindingName,
		RoleName:        result.RoleName,
		RoleExists:      result.RoleExists,
		Subjects:        make([]types.SubjectInfo, 0, len(result.Subjects)),
		Rules:           make([]types.PolicyRuleInfo, 0, len(result.Rules)),
		EffectiveScopes: result.EffectiveScopes,
		IsSuperAdmin:    result.IsSuperAdmin,
		RiskLevel:       result.RiskLevel,
	}

	for _, s := range result.Subjects {
		resp.Subjects = append(resp.Subjects, types.SubjectInfo{
			Kind:      s.Kind,
			Name:      s.Name,
			Namespace: s.Namespace,
			APIGroup:  s.APIGroup,
		})
	}

	for _, rule := range result.Rules {
		resp.Rules = append(resp.Rules, types.PolicyRuleInfo{
			Verbs:           rule.Verbs,
			APIGroups:       rule.APIGroups,
			Resources:       rule.Resources,
			ResourceNames:   rule.ResourceNames,
			NonResourceURLs: rule.NonResourceURLs,
		})
	}

	return resp, nil
}
