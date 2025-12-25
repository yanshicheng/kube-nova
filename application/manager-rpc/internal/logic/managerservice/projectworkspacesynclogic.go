package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectWorkspaceSyncLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectWorkspaceSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectWorkspaceSyncLogic {
	return &ProjectWorkspaceSyncLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectWorkspaceSync 同步工作空间状态
func (l *ProjectWorkspaceSyncLogic) ProjectWorkspaceSync(in *pb.ProjectWorkspaceSyncReq) (*pb.ProjectWorkspaceSyncResp, error) {

	// 查询工作空间
	workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("查询工作空间失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("工作空间不存在")
	}

	//  调用ControllerRpc获取 namespace 实际资源使用情况
	clusterClient, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workspace.ClusterUuid)
	if err != nil {
		l.Logger.Errorf("获取集群失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("获取集群失败")
	}
	limitRangeOperator := clusterClient.LimitRange()
	resourceQuotaOperator := clusterClient.ResourceQuota()

	limitRangeAllocated, err := limitRangeOperator.GetLimits(workspace.Namespace, "ikubeops"+workspace.Name)
	quotaAllocated, err := resourceQuotaOperator.GetAllocated(workspace.Namespace, "ikubeops"+workspace.Name)

	workspace.CpuAllocated = quotaAllocated.CPUAllocated
	workspace.MemAllocated = quotaAllocated.MemoryAllocated
	workspace.StorageAllocated = quotaAllocated.StorageAllocated
	workspace.GpuAllocated = quotaAllocated.GPUAllocated
	workspace.PodsAllocated = quotaAllocated.PodsAllocated
	workspace.ConfigmapAllocated = quotaAllocated.ConfigMapsAllocated
	workspace.SecretAllocated = quotaAllocated.SecretsAllocated
	workspace.PvcAllocated = quotaAllocated.PVCsAllocated
	workspace.EphemeralStorageAllocated = quotaAllocated.EphemeralStorageAllocated
	workspace.ServiceAllocated = quotaAllocated.ServicesAllocated
	workspace.LoadbalancersAllocated = quotaAllocated.LoadBalancersAllocated
	workspace.NodeportsAllocated = quotaAllocated.NodePortsAllocated
	workspace.DeploymentsAllocated = quotaAllocated.DeploymentsAllocated
	workspace.JobsAllocated = quotaAllocated.JobsAllocated
	workspace.CronjobsAllocated = quotaAllocated.CronJobsAllocated
	workspace.DaemonsetsAllocated = quotaAllocated.DaemonSetsAllocated
	workspace.StatefulsetsAllocated = quotaAllocated.StatefulSetsAllocated
	workspace.IngressesAllocated = quotaAllocated.IngressesAllocated

	workspace.PodMinCpu = limitRangeAllocated.PodMinCPU
	workspace.PodMinMemory = limitRangeAllocated.PodMinMemory
	workspace.PodMinEphemeralStorage = limitRangeAllocated.PodMinEphemeralStorage

	workspace.PodMaxCpu = limitRangeAllocated.PodMaxCPU
	workspace.PodMaxMemory = limitRangeAllocated.PodMaxMemory
	workspace.PodMaxEphemeralStorage = limitRangeAllocated.PodMaxEphemeralStorage

	workspace.ContainerMaxCpu = limitRangeAllocated.ContainerMaxCPU
	workspace.ContainerMaxMemory = limitRangeAllocated.ContainerMaxMemory
	workspace.ContainerMaxEphemeralStorage = limitRangeAllocated.PodMaxEphemeralStorage

	workspace.ContainerMinCpu = limitRangeAllocated.ContainerMinCPU
	workspace.ContainerMinMemory = limitRangeAllocated.ContainerMinMemory
	workspace.ContainerMinEphemeralStorage = limitRangeAllocated.ContainerMinEphemeralStorage

	workspace.ContainerDefaultCpu = limitRangeAllocated.ContainerDefaultCPU
	workspace.ContainerDefaultMemory = limitRangeAllocated.ContainerDefaultMemory
	workspace.ContainerDefaultEphemeralStorage = limitRangeAllocated.ContainerDefaultEphemeralStorage
	workspace.ContainerDefaultRequestMemory = limitRangeAllocated.ContainerDefaultRequestMemory
	workspace.ContainerDefaultRequestCpu = limitRangeAllocated.ContainerDefaultRequestCPU
	workspace.ContainerDefaultRequestEphemeralStorage = limitRangeAllocated.ContainerDefaultRequestEphemeralStorage

	if err != nil {
		l.Logger.Errorf("获取 namespace 资源使用情况失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("获取 namespace 资源使用情况失败")
	}
	err = l.svcCtx.OnecProjectWorkspaceModel.Update(l.ctx, workspace)
	if err != nil {
		l.Logger.Errorf("更新工作空间状态失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("更新工作空间状态失败")
	}

	return &pb.ProjectWorkspaceSyncResp{}, nil
}

func convertWorkspaceToProto(ws *model.OnecProjectWorkspace, clusterName string) *pb.OnecProjectWorkspace {

	return &pb.OnecProjectWorkspace{
		Id:                                      ws.Id,
		ProjectClusterId:                        ws.ProjectClusterId,
		ClusterUuid:                             ws.ClusterUuid,
		Name:                                    ws.Name,
		Namespace:                               ws.Namespace,
		Description:                             ws.Description,
		CpuAllocated:                            ws.CpuAllocated,
		MemAllocated:                            ws.MemAllocated,
		StorageAllocated:                        ws.StorageAllocated,
		GpuAllocated:                            ws.GpuAllocated,
		PodsAllocated:                           ws.PodsAllocated,
		ConfigmapAllocated:                      ws.ConfigmapAllocated,
		SecretAllocated:                         ws.SecretAllocated,
		PvcAllocated:                            ws.PvcAllocated,
		EphemeralStorageAllocated:               ws.EphemeralStorageAllocated,
		ServiceAllocated:                        ws.ServiceAllocated,
		LoadbalancersAllocated:                  ws.LoadbalancersAllocated,
		NodeportsAllocated:                      ws.NodeportsAllocated,
		DeploymentsAllocated:                    ws.DeploymentsAllocated,
		JobsAllocated:                           ws.JobsAllocated,
		CronjobsAllocated:                       ws.CronjobsAllocated,
		DaemonsetsAllocated:                     ws.DaemonsetsAllocated,
		StatefulsetsAllocated:                   ws.StatefulsetsAllocated,
		IngressesAllocated:                      ws.IngressesAllocated,
		PodMaxCpu:                               ws.PodMaxCpu,
		PodMaxMemory:                            ws.PodMaxMemory,
		PodMaxEphemeralStorage:                  ws.PodMaxEphemeralStorage,
		PodMinCpu:                               ws.PodMinCpu,
		PodMinMemory:                            ws.PodMinMemory,
		PodMinEphemeralStorage:                  ws.PodMinEphemeralStorage,
		ContainerMaxCpu:                         ws.ContainerMaxCpu,
		ContainerMaxMemory:                      ws.ContainerMaxMemory,
		ContainerMaxEphemeralStorage:            ws.ContainerMaxEphemeralStorage,
		ContainerMinCpu:                         ws.ContainerMinCpu,
		ContainerMinMemory:                      ws.ContainerMinMemory,
		ContainerMinEphemeralStorage:            ws.ContainerMinEphemeralStorage,
		ContainerDefaultCpu:                     ws.ContainerDefaultCpu,
		ContainerDefaultMemory:                  ws.ContainerDefaultMemory,
		ContainerDefaultEphemeralStorage:        ws.ContainerDefaultEphemeralStorage,
		ContainerDefaultRequestCpu:              ws.ContainerDefaultRequestCpu,
		ContainerDefaultRequestMemory:           ws.ContainerDefaultRequestMemory,
		ContainerDefaultRequestEphemeralStorage: ws.ContainerDefaultRequestEphemeralStorage,
		IsSystem:                                ws.IsSystem,
		Status:                                  ws.Status,
		AppCreateTime:                           ws.AppCreateTime.Unix(),
		CreatedBy:                               ws.CreatedBy,
		UpdatedBy:                               ws.UpdatedBy,
		CreatedAt:                               ws.CreatedAt.Unix(),
		UpdatedAt:                               ws.UpdatedAt.Unix(),
		ClusterName:                             clusterName,
	}
}
