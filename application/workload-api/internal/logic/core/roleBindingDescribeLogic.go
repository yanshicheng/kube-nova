// roleBindingDescribeLogic.go
package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleBindingDescribeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleBindingDescribeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleBindingDescribeLogic {
	return &RoleBindingDescribeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleBindingDescribeLogic) RoleBindingDescribe(req *types.ClusterNamespaceResourceRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 获取 RoleBinding operator
	rbOp := client.RoleBindings()

	// 获取详细描述
	description, err := rbOp.Describe(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 RoleBinding 详细描述失败: %v", err)
		return "", fmt.Errorf("获取 RoleBinding 详细描述失败")
	}

	l.Infof("用户: %s, 成功获取 RoleBinding 详细描述: %s/%s", username, req.Namespace, req.Name)
	return description, nil
}
