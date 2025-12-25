// roleBindingGetYamlLogic.go
package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleBindingGetYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleBindingGetYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleBindingGetYamlLogic {
	return &RoleBindingGetYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleBindingGetYamlLogic) RoleBindingGetYaml(req *types.ClusterNamespaceResourceRequest) (resp string, err error) {
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

	// 获取 YAML
	yamlStr, err := rbOp.GetYaml(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 RoleBinding YAML 失败: %v", err)
		return "", fmt.Errorf("获取 RoleBinding YAML 失败")
	}

	l.Infof("用户: %s, 成功获取 RoleBinding YAML: %s/%s", username, req.Namespace, req.Name)
	return yamlStr, nil
}
