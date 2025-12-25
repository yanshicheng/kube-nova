package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EvictPodLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// pod驱逐
func NewEvictPodLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EvictPodLogic {
	return &EvictPodLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EvictPodLogic) EvictPod(req *types.DefaultPodNameRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	podOperaro := client.Pods()
	err = podOperaro.Evict(versionDetail.Namespace, req.PodName)
	if err != nil {
		l.Errorf("驱逐pod失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "驱逐Pod",
			fmt.Sprintf("资源 %s/%s 驱逐Pod %s 失败: %v", versionDetail.Namespace, versionDetail.ResourceName, req.PodName, err), 2)
		return "", fmt.Errorf("驱逐pod失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "驱逐Pod",
		fmt.Sprintf("资源 %s/%s 驱逐Pod %s 成功", versionDetail.Namespace, versionDetail.ResourceName, req.PodName), 1)
	return "驱逐pod成功", nil
}
