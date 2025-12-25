// roleGetYamlLogic.go
package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleGetYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleGetYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleGetYamlLogic {
	return &RoleGetYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleGetYamlLogic) RoleGetYaml(req *types.ClusterNamespaceResourceRequest) (resp string, err error) {
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

	// 获取 Role operator
	roleOp := client.Roles()

	// 获取 YAML
	yamlStr, err := roleOp.GetYaml(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Role YAML 失败: %v", err)
		return "", fmt.Errorf("获取 Role YAML 失败")
	}

	l.Infof("用户: %s, 成功获取 Role YAML: %s/%s", username, req.Namespace, req.Name)
	return yamlStr, nil
}
