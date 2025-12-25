package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EvictPodWithClusterUuidLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 通过 clusteruuid 驱逐 pod
func NewEvictPodWithClusterUuidLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EvictPodWithClusterUuidLogic {
	return &EvictPodWithClusterUuidLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EvictPodWithClusterUuidLogic) EvictPodWithClusterUuid(req *types.GetPodDetailWithClusterUuidRequest) (resp string, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		logx.Errorf("集群管理器获取失败: %s", err.Error())
		return "", fmt.Errorf("集群管理器获取失败: %s", err.Error())
	}

	podOperaro := client.Pods()
	err = podOperaro.Evict(req.Namespace, req.PodName)
	if err != nil {
		l.Errorf("驱逐pod失败: %v", err)
		recordAuditLogByClusterUuid(l.ctx, l.svcCtx, req.ClusterUuid, "驱逐Pod",
			fmt.Sprintf("Pod %s/%s 驱逐失败: %v", req.Namespace, req.PodName, err), 2)
		return "", fmt.Errorf("驱逐pod失败")
	}

	recordAuditLogByClusterUuid(l.ctx, l.svcCtx, req.ClusterUuid, "驱逐Pod",
		fmt.Sprintf("Pod %s/%s 驱逐成功", req.Namespace, req.PodName), 1)
	return "驱逐pod成功", nil
}
