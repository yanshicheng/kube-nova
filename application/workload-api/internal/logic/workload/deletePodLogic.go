package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeletePodLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除pod
func NewDeletePodLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeletePodLogic {
	return &DeletePodLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeletePodLogic) DeletePod(req *types.DefaultPodNameRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}
	podOperator := client.Pods()
	err = podOperator.Delete(versionDetail.Namespace, req.PodName)
	if err != nil {
		l.Errorf("删除Pod失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "删除Pod",
			fmt.Sprintf("资源 %s/%s 删除Pod %s 失败: %v", versionDetail.Namespace, versionDetail.ResourceName, req.PodName, err), 2)
		return "", fmt.Errorf("删除Pod失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "删除Pod",
		fmt.Sprintf("资源 %s/%s 删除Pod %s 成功", versionDetail.Namespace, versionDetail.ResourceName, req.PodName), 1)
	return "删除Pod成功", nil
}
