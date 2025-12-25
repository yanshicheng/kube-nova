package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PVGetYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 PV YAML
func NewPVGetYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVGetYamlLogic {
	return &PVGetYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVGetYamlLogic) PVGetYaml(req *types.ClusterResourceNameRequest) (resp string, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	pvOp := client.PersistentVolumes()
	yamlStr, err := pvOp.GetYaml(req.Name)
	if err != nil {
		l.Errorf("获取 PV YAML 失败: %v", err)
		return "", fmt.Errorf("获取 PV YAML 失败")
	}

	return yamlStr, nil
}
