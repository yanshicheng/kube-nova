package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPodDetailWithClusterUuidLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 通过 clusterUuid 获取 pod 详情
func NewGetPodDetailWithClusterUuidLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPodDetailWithClusterUuidLogic {
	return &GetPodDetailWithClusterUuidLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPodDetailWithClusterUuidLogic) GetPodDetailWithClusterUuid(req *types.GetPodDetailWithClusterUuidRequest) (resp string, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		logx.Errorf("集群管理器获取失败: %s", err.Error())
		return "", fmt.Errorf("集群管理器获取失败: %s", err.Error())
	}
	return client.Pods().GetDescribe(req.Namespace, req.PodName)

}
