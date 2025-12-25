// roleGetAssociationLogic.go
package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleGetAssociationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleGetAssociationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleGetAssociationLogic {
	return &RoleGetAssociationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleGetAssociationLogic) RoleGetAssociation(req *types.ClusterNamespaceResourceRequest) (resp *types.RoleAssociation, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 获取 Role operator
	roleOp := client.Roles()

	// 获取关联信息
	association, err := roleOp.GetAssociation(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Role 关联信息失败: %v", err)
		return nil, fmt.Errorf("获取 Role 关联信息失败")
	}
	resp = &types.RoleAssociation{
		RoleName:     association.RoleName,
		Namespace:    association.Namespace,
		RoleBindings: association.RoleBindings,
		BindingCount: association.BindingCount,
		Subjects:     association.Subjects,
	}

	l.Infof("用户: %s, 成功获取 Role 关联信息: %s/%s", username, req.Namespace, req.Name)
	return resp, nil
}
