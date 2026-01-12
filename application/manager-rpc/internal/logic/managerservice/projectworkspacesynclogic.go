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
	workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("查询工作空间失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("工作空间不存在")
	}

	workspaceId := workspace.Id
	workspaceName := workspace.Name
	namespace := workspace.Namespace
	clusterUuid := workspace.ClusterUuid
	svcCtx := l.svcCtx

	go func() {
		ctx := context.Background()
		logger := logx.WithContext(ctx)

		logger.Infof("开始异步同步工作空间 [%s] 状态", workspaceName)

		// 重新查询工作空间（因为这是新的goroutine，需要最新数据）
		ws, err := svcCtx.OnecProjectWorkspaceModel.FindOne(ctx, workspaceId)
		if err != nil {
			logger.Errorf("重新查询工作空间失败，ID: %d, 错误: %v", workspaceId, err)
			return
		}

		// 获取集群客户端
		clusterClient, err := svcCtx.K8sManager.GetCluster(ctx, clusterUuid)
		if err != nil {
			logger.Errorf("获取集群失败，工作空间ID: %d, 错误: %v", workspaceId, err)
			return
		}

		limitRangeOperator := clusterClient.LimitRange()
		resourceQuotaOperator := clusterClient.ResourceQuota()

		limitRangeAllocated, err := limitRangeOperator.GetLimits(namespace, "ikubeops"+workspaceName)
		if err != nil {
			logger.Errorf("获取LimitRange失败: %v", err)
			return
		}

		quotaAllocated, err := resourceQuotaOperator.GetAllocated(namespace, "ikubeops"+workspaceName)
		if err != nil {
			logger.Errorf("获取ResourceQuota失败: %v", err)
			return
		}

		// 更新工作空间数据
		ws.CpuAllocated = quotaAllocated.CPUAllocated
		ws.MemAllocated = quotaAllocated.MemoryAllocated
		ws.StorageAllocated = quotaAllocated.StorageAllocated
		ws.GpuAllocated = quotaAllocated.GPUAllocated
		ws.PodsAllocated = quotaAllocated.PodsAllocated
		ws.ConfigmapAllocated = quotaAllocated.ConfigMapsAllocated
		ws.SecretAllocated = quotaAllocated.SecretsAllocated
		ws.PvcAllocated = quotaAllocated.PVCsAllocated
		ws.EphemeralStorageAllocated = quotaAllocated.EphemeralStorageAllocated
		ws.ServiceAllocated = quotaAllocated.ServicesAllocated
		ws.LoadbalancersAllocated = quotaAllocated.LoadBalancersAllocated
		ws.NodeportsAllocated = quotaAllocated.NodePortsAllocated
		ws.DeploymentsAllocated = quotaAllocated.DeploymentsAllocated
		ws.JobsAllocated = quotaAllocated.JobsAllocated
		ws.CronjobsAllocated = quotaAllocated.CronJobsAllocated
		ws.DaemonsetsAllocated = quotaAllocated.DaemonSetsAllocated
		ws.StatefulsetsAllocated = quotaAllocated.StatefulSetsAllocated
		ws.IngressesAllocated = quotaAllocated.IngressesAllocated

		ws.PodMinCpu = limitRangeAllocated.PodMinCPU
		ws.PodMinMemory = limitRangeAllocated.PodMinMemory
		ws.PodMinEphemeralStorage = limitRangeAllocated.PodMinEphemeralStorage
		ws.PodMaxCpu = limitRangeAllocated.PodMaxCPU
		ws.PodMaxMemory = limitRangeAllocated.PodMaxMemory
		ws.PodMaxEphemeralStorage = limitRangeAllocated.PodMaxEphemeralStorage

		ws.ContainerMaxCpu = limitRangeAllocated.ContainerMaxCPU
		ws.ContainerMaxMemory = limitRangeAllocated.ContainerMaxMemory
		ws.ContainerMaxEphemeralStorage = limitRangeAllocated.PodMaxEphemeralStorage
		ws.ContainerMinCpu = limitRangeAllocated.ContainerMinCPU
		ws.ContainerMinMemory = limitRangeAllocated.ContainerMinMemory
		ws.ContainerMinEphemeralStorage = limitRangeAllocated.ContainerMinEphemeralStorage

		ws.ContainerDefaultCpu = limitRangeAllocated.ContainerDefaultCPU
		ws.ContainerDefaultMemory = limitRangeAllocated.ContainerDefaultMemory
		ws.ContainerDefaultEphemeralStorage = limitRangeAllocated.ContainerDefaultEphemeralStorage
		ws.ContainerDefaultRequestMemory = limitRangeAllocated.ContainerDefaultRequestMemory
		ws.ContainerDefaultRequestCpu = limitRangeAllocated.ContainerDefaultRequestCPU
		ws.ContainerDefaultRequestEphemeralStorage = limitRangeAllocated.ContainerDefaultRequestEphemeralStorage

		err = svcCtx.OnecProjectWorkspaceModel.Update(ctx, ws)
		if err != nil {
			logger.Errorf("更新工作空间状态失败，ID: %d, 错误: %v", workspaceId, err)
			return
		}

		logger.Infof("工作空间 [%s] 状态同步完成", workspaceName)
	}()

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
