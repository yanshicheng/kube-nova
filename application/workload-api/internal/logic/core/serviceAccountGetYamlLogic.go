// serviceAccountGetYamlLogic.go
package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceAccountGetYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceAccountGetYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceAccountGetYamlLogic {
	return &ServiceAccountGetYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceAccountGetYamlLogic) ServiceAccountGetYaml(req *types.ClusterNamespaceResourceRequest) (resp string, err error) {
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

	// 获取 ServiceAccount operator
	saOp := client.ServiceAccounts()

	// 获取 YAML
	yamlStr, err := saOp.GetYaml(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 ServiceAccount YAML 失败: %v", err)
		return "", fmt.Errorf("获取 ServiceAccount YAML 失败")
	}

	l.Infof("用户: %s, 成功获取 ServiceAccount YAML: %s/%s", username, req.Namespace, req.Name)
	return yamlStr, nil
}
