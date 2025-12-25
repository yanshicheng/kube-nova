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

type GetResourceQuotaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetResourceQuotaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetResourceQuotaLogic {
	return &GetResourceQuotaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetResourceQuotaLogic) GetResourceQuota(req *types.DefaultIdRequest) (resp *types.ResourcesResponse, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	var resources *types2.ResourcesResponse

	switch resourceType {
	case "DEPLOYMENT":
		resources, err = client.Deployment().GetResources(versionDetail.Namespace, versionDetail.ResourceName)
	case "STATEFULSET":
		resources, err = client.StatefulSet().GetResources(versionDetail.Namespace, versionDetail.ResourceName)
	case "DAEMONSET":
		resources, err = client.DaemonSet().GetResources(versionDetail.Namespace, versionDetail.ResourceName)
	case "JOB":
		resources, err = client.Job().GetResources(versionDetail.Namespace, versionDetail.ResourceName)
	case "CRONJOB":
		resources, err = client.CronJob().GetResources(versionDetail.Namespace, versionDetail.ResourceName)
	default:
		return nil, fmt.Errorf("资源类型 %s 不支持查询资源配额", resourceType)
	}

	if err != nil {
		l.Errorf("获取资源配额失败: %v", err)
		return nil, fmt.Errorf("获取资源配额失败")
	}

	resp = convertToResourcesResponse(resources)
	return resp, nil
}

func convertToResourcesResponse(resources *types2.ResourcesResponse) *types.ResourcesResponse {
	containers := make([]types.ContainerResources, 0, len(resources.Containers))

	for _, container := range resources.Containers {
		containers = append(containers, types.ContainerResources{
			ContainerName: container.ContainerName,
			Resources: types.ResourceRequirements{
				Limits: types.ResourceList{
					Cpu:    container.Resources.Limits.Cpu,
					Memory: container.Resources.Limits.Memory,
				},
				Requests: types.ResourceList{
					Cpu:    container.Resources.Requests.Cpu,
					Memory: container.Resources.Requests.Memory,
				},
			},
		})
	}

	return &types.ResourcesResponse{
		Containers: containers,
	}
}
