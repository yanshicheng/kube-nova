package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterRoleGetYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 ClusterRole YAML
func NewClusterRoleGetYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleGetYamlLogic {
	return &ClusterRoleGetYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleGetYamlLogic) ClusterRoleGetYaml(req *types.ClusterResourceNameRequest) (resp string, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	crOp := client.ClusterRoles()
	yamlStr, err := crOp.GetYaml(req.Name)
	if err != nil {
		l.Errorf("获取 ClusterRole YAML 失败: %v", err)
		return "", fmt.Errorf("获取 ClusterRole YAML 失败")
	}

	return yamlStr, nil
}
