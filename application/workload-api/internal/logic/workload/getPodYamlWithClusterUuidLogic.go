package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPodYamlWithClusterUuidLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 通过 clusterUuid 获取 pod yaml
func NewGetPodYamlWithClusterUuidLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPodYamlWithClusterUuidLogic {
	return &GetPodYamlWithClusterUuidLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPodYamlWithClusterUuidLogic) GetPodYamlWithClusterUuid(req *types.GetPodYamlWithClusterUuidRequest) (resp string, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		logx.Errorf("集群管理器获取失败: %s", err.Error())
		return "", fmt.Errorf("集群管理器获取失败: %s", err.Error())
	}
	resp, err = client.Pods().GetYaml(req.Namespace, req.PodName)
	if err != nil {
		l.Errorf("获取PodYaml失败: %v", err)
		return "", fmt.Errorf("获取PodYaml失败")
	}
	return
}
