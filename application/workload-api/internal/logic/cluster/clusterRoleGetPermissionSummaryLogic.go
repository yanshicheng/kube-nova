package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterRoleGetPermissionSummaryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 ClusterRole 权限摘要
func NewClusterRoleGetPermissionSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleGetPermissionSummaryLogic {
	return &ClusterRoleGetPermissionSummaryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleGetPermissionSummaryLogic) ClusterRoleGetPermissionSummary(req *types.ClusterResourceNameRequest) (resp *types.ClusterRolePermissionSummary, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	crOp := client.ClusterRoles()
	result, err := crOp.GetPermissionSummary(req.Name)
	if err != nil {
		l.Errorf("获取 ClusterRole 权限摘要失败: %v", err)
		return nil, fmt.Errorf("获取 ClusterRole 权限摘要失败")
	}

	return &types.ClusterRolePermissionSummary{
		ClusterRoleName: result.ClusterRoleName,
		HasAllVerbs:     result.HasAllVerbs,
		HasAllResources: result.HasAllResources,
		VerbSummary:     result.VerbSummary,
		ResourceSummary: result.ResourceSummary,
		IsSuperAdmin:    result.IsSuperAdmin,
		RiskLevel:       result.RiskLevel,
	}, nil
}
