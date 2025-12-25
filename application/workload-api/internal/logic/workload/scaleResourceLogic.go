package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ScaleResourceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 调整资源副本数
func NewScaleResourceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ScaleResourceLogic {
	return &ScaleResourceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ScaleResourceLogic) ScaleResource(req *types.ReplicasResourceRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	// 获取当前副本数用于审计日志
	var currentReplicas int32

	switch resourceType {
	case "DEPLOYMENT":
		replicasInfo, _ := client.Deployment().GetReplicas(versionDetail.Namespace, versionDetail.ResourceName)
		if replicasInfo != nil {
			currentReplicas = replicasInfo.Replicas
		}
		deploymentOperator := client.Deployment()
		err = deploymentOperator.Scale(&types2.ScaleRequest{
			Name:      versionDetail.ResourceName,
			Namespace: versionDetail.Namespace,
			Replicas:  req.Replicas,
		})
		if err != nil {
			l.Errorf("调整Deployment副本数失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "副本扩缩容",
				fmt.Sprintf("Deployment %s/%s 调整副本数失败, 当前副本: %d, 目标副本: %d, 错误: %v", versionDetail.Namespace, versionDetail.ResourceName, currentReplicas, req.Replicas, err), 2)
			return "", fmt.Errorf("调整Deployment副本数失败")
		}

	case "STATEFULSET":
		replicasInfo, _ := client.StatefulSet().GetReplicas(versionDetail.Namespace, versionDetail.ResourceName)
		if replicasInfo != nil {
			currentReplicas = replicasInfo.Replicas
		}
		statefulSetOperator := client.StatefulSet()
		err = statefulSetOperator.Scale(&types2.ScaleRequest{
			Name:      versionDetail.ResourceName,
			Namespace: versionDetail.Namespace,
			Replicas:  req.Replicas,
		})
		if err != nil {
			l.Errorf("调整StatefulSet副本数失败: %v", err)
			recordAuditLog(l.ctx, l.svcCtx, versionDetail, "副本扩缩容",
				fmt.Sprintf("StatefulSet %s/%s 调整副本数失败, 当前副本: %d, 目标副本: %d, 错误: %v", versionDetail.Namespace, versionDetail.ResourceName, currentReplicas, req.Replicas, err), 2)
			return "", fmt.Errorf("调整StatefulSet副本数失败")
		}
	default:
		return "", fmt.Errorf("不支持的资源类型")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "副本扩缩容",
		fmt.Sprintf("%s %s/%s 调整副本数成功, 副本数从 %d 调整为 %d", resourceType, versionDetail.Namespace, versionDetail.ResourceName, currentReplicas, req.Replicas), 1)
	return "调整副本数成功", nil
}
