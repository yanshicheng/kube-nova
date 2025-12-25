// ./workload-api/common.go
package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	k8sTypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"
)

// ResourceController 资源控制器接口
type ResourceController struct {
	Deployment  k8sTypes.DeploymentOperator
	StatefulSet k8sTypes.StatefulSetOperator
	DaemonSet   k8sTypes.DaemonSetOperator
	Job         k8sTypes.JobOperator
	CronJob     k8sTypes.CronJobOperator
	ReplicaSet  k8sTypes.ReplicaSetOperator
}

// getResourceController 获取资源控制器
func getResourceController(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	versionId uint64,
) (*managerservice.GetOnecProjectVersionDetailResp, *ResourceController, error) {
	// 1. 获取版本详情
	versionDetail, err := svcCtx.ManagerRpc.VersionDetail(ctx, &managerservice.GetOnecProjectVersionDetailReq{
		Id: versionId,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("获取版本详情失败: %v", err)
	}

	// 2. 获取集群客户端
	client, err := svcCtx.K8sManager.GetCluster(ctx, versionDetail.ClusterUuid)
	if err != nil {
		return nil, nil, fmt.Errorf("获取集群客户端失败: %v", err)
	}

	// 3. 初始化资源控制器
	controller := &ResourceController{
		Deployment:  client.Deployment(),
		StatefulSet: client.StatefulSet(),
		DaemonSet:   client.DaemonSet(),
		Job:         client.Job(),
		CronJob:     client.CronJob(),
		ReplicaSet:  client.ReplicaSet(),
	}

	return versionDetail, controller, nil
}

// executeResourceOperation 执行资源操作的通用方法
func executeResourceOperation(
	versionDetail *managerservice.GetOnecProjectVersionDetailResp,
	controller *ResourceController,
	operation func(namespace, name string) error,
) error {
	resourceType := strings.ToUpper(versionDetail.ResourceType)
	namespace := versionDetail.Namespace
	name := versionDetail.ResourceName

	switch resourceType {
	case "DEPLOYMENT":
		if controller.Deployment == nil {
			return fmt.Errorf("Deployment 控制器未初始化")
		}
		return operation(namespace, name)
	case "STATEFULSET":
		if controller.StatefulSet == nil {
			return fmt.Errorf("StatefulSet 控制器未初始化")
		}
		return operation(namespace, name)
	case "DAEMONSET":
		if controller.DaemonSet == nil {
			return fmt.Errorf("DaemonSet 控制器未初始化")
		}
		return operation(namespace, name)
	default:
		return fmt.Errorf("不支持的资源类型: %s", resourceType)
	}
}

// getResourceClusterClient
func getResourceClusterClient(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	versionId uint64,
) (cluster.Client, *managerservice.GetOnecProjectVersionDetailResp, error) {
	// 1. 获取版本详情
	versionDetail, err := svcCtx.ManagerRpc.VersionDetail(ctx, &managerservice.GetOnecProjectVersionDetailReq{
		Id: versionId,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("获取版本详情失败: %v", err)
	}
	client, err := svcCtx.K8sManager.GetCluster(ctx, versionDetail.ClusterUuid)
	if err != nil {
		return nil, nil, fmt.Errorf("获取集群客户端失败: %v", err)
	}
	return client, versionDetail, nil
}
