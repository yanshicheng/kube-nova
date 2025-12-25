package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigMapGetUsageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 ConfigMap 引用情况
func NewConfigMapGetUsageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigMapGetUsageLogic {
	return &ConfigMapGetUsageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigMapGetUsageLogic) ConfigMapGetUsage(req *types.DefaultNameRequest) (resp *types.ConfigMapUsageResponse, err error) {
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败")
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	configMapClient := client.ConfigMaps()

	usage, err := configMapClient.GetUsage(workloadInfo.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 ConfigMap 引用情况失败: %v", err)
		return nil, fmt.Errorf("获取 ConfigMap 引用情况失败")
	}

	usedBy := make([]types.ConfigMapUsageReference, 0, len(usage.UsedBy))
	for _, ref := range usage.UsedBy {
		usedBy = append(usedBy, types.ConfigMapUsageReference{
			ResourceType:   ref.ResourceType,
			ResourceName:   ref.ResourceName,
			Namespace:      ref.Namespace,
			UsageType:      ref.UsageType,
			UsedKeys:       ref.UsedKeys,
			ContainerNames: ref.ContainerNames,
		})
	}

	l.Infof("成功获取 ConfigMap 引用情况: %s, 被 %d 个资源引用", req.Name, len(usedBy))
	return &types.ConfigMapUsageResponse{
		ConfigMapName:      usage.ConfigMapName,
		ConfigMapNamespace: usage.ConfigMapNamespace,
		UsedBy:             usedBy,
		TotalUsageCount:    usage.TotalUsageCount,
		CanDelete:          usage.CanDelete,
		DeleteWarning:      usage.DeleteWarning,
	}, nil
}
