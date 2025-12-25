package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type IngressClassGetYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 IngressClass YAML
func NewIngressClassGetYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngressClassGetYamlLogic {
	return &IngressClassGetYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngressClassGetYamlLogic) IngressClassGetYaml(req *types.IngressClassNameRequest) (resp string, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	icOp := client.IngressClasses()
	yamlStr, err := icOp.GetYaml(req.Name)
	if err != nil {
		l.Errorf("获取 IngressClass YAML 失败: %v", err)
		return "", fmt.Errorf("获取 IngressClass YAML 失败")
	}

	return yamlStr, nil
}
