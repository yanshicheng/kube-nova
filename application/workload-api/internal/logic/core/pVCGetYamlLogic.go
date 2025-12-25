package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type PVCGetYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 PVC YAML
func NewPVCGetYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVCGetYamlLogic {
	return &PVCGetYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVCGetYamlLogic) PVCGetYaml(req *types.ClusterNamespaceResourceRequest) (resp string, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	pvcClient := client.PVC()

	yamlStr, err := pvcClient.GetYaml(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 PVC YAML 失败: %v", err)
		return "", fmt.Errorf("获取 PVC YAML 失败")
	}

	l.Infof("成功获取 PVC YAML: %s/%s", req.Namespace, req.Name)
	return yamlStr, nil
}
