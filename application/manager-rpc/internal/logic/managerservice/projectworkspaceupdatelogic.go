package managerservicelogic

import (
	"context"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ProjectWorkspaceUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectWorkspaceUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectWorkspaceUpdateLogic {
	return &ProjectWorkspaceUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectWorkspaceUpdate 更新项目工作空间
func (l *ProjectWorkspaceUpdateLogic) ProjectWorkspaceUpdate(in *pb.UpdateOnecProjectWorkspaceReq) (*pb.UpdateOnecProjectWorkspaceResp, error) {
	l.Logger.Infof("开始更新工作空间，ID: %d", in.Id)

	// 查询工作空间是否存在
	workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("查询工作空间失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("工作空间不存在")
	}

	// 查询项目集群配额
	projectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, workspace.ProjectClusterId)
	if err != nil {
		l.Logger.Errorf("查询项目集群配额失败，ID: %d, 错误: %v", workspace.ProjectClusterId, err)
		return nil, errorx.Msg("项目集群配额不存在")
	}
	// 获取 project
	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, projectCluster.ProjectId)
	if err != nil {
		l.Logger.Errorf("查询项目失败，ID: %d, 错误: %v", projectCluster.ProjectId, err)
	}
	// 检查资源变更是否超出配额
	if err := l.checkResourceAvailable(projectCluster, workspace, in); err != nil {
		l.Logger.Errorf("资源检查失败: %v", err)
		return nil, errorx.Msg(err.Error())
	}
	// 更新工作空间 注解
	clusterClient, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workspace.ClusterUuid)
	if err != nil {
		l.Logger.Errorf("获取集群客户端失败: %v", err)
		return nil, errorx.Msg("获取集群客户端失败")
	}
	ns, err := clusterClient.Namespaces().Get(workspace.Namespace)
	if err != nil {
		l.Logger.Errorf("获取命名空间失败: %v", err)
		return nil, errorx.Msg("获取命名空间失败")
	}
	utils.AddAnnotations(&ns.ObjectMeta, &utils.AnnotationsInfo{
		ProjectUuid:   project.Uuid,
		ProjectName:   project.Name,
		WorkspaceName: workspace.Name,
		ServiceName:   ns.Name,
	})

	// 创建 资源限制
	resourceQuotaReq := &types.ResourceQuotaRequest{
		Namespace:                 workspace.Namespace,
		Labels:                    make(map[string]string),
		Annotations:               ns.Annotations,
		Name:                      fmt.Sprintf("ikubeops-%s", workspace.Namespace),
		CPUAllocated:              in.CpuAllocated,
		MemoryAllocated:           in.MemAllocated,
		StorageAllocated:          in.StorageAllocated,
		GPUAllocated:              in.GpuAllocated,
		EphemeralStorageAllocated: in.EphemeralStorageAllocated,
		// 对象数量配额: in.
		PodsAllocated:          in.PodsAllocated,
		ConfigMapsAllocated:    in.ConfigmapAllocated,
		SecretsAllocated:       in.SecretAllocated,
		PVCsAllocated:          in.PvcAllocated,
		ServicesAllocated:      in.ServiceAllocated,
		LoadBalancersAllocated: in.LoadbalancersAllocated,
		NodePortsAllocated:     in.NodeportsAllocated,
		// 工作负载配额: in.
		DeploymentsAllocated:  in.DeploymentsAllocated,
		JobsAllocated:         in.JobsAllocated,
		CronJobsAllocated:     in.CronjobsAllocated,
		DaemonSetsAllocated:   in.DaemonsetsAllocated,
		StatefulSetsAllocated: in.StatefulsetsAllocated,
		IngressesAllocated:    in.IngressesAllocated,
	}
	// 创建 limitRange
	reqLimitRange := &types.LimitRangeRequest{
		Namespace:   workspace.Namespace,
		Labels:      make(map[string]string),
		Annotations: ns.Annotations,
		Name:        fmt.Sprintf("ikubeops-%s", workspace.Namespace),
		// Pod级别限制
		PodMaxCPU:              in.PodMaxCpu, // 核心数
		PodMaxMemory:           in.PodMaxMemory,
		PodMaxEphemeralStorage: in.PodMaxEphemeralStorage,
		PodMinCPU:              in.PodMinCpu,
		PodMinMemory:           in.PodMinMemory,
		PodMinEphemeralStorage: in.PodMinEphemeralStorage,
		// Container级别限制
		ContainerMaxCPU:              in.ContainerMaxCpu,
		ContainerMaxMemory:           in.ContainerMaxMemory,
		ContainerMaxEphemeralStorage: in.ContainerMaxEphemeralStorage,
		ContainerMinCPU:              in.ContainerMinCpu,
		ContainerMinMemory:           in.ContainerMinMemory,
		ContainerMinEphemeralStorage: in.ContainerMinEphemeralStorage,
		// Container默认限制（limits）
		ContainerDefaultCPU:              in.ContainerDefaultCpu,
		ContainerDefaultMemory:           in.ContainerDefaultMemory,
		ContainerDefaultEphemeralStorage: in.ContainerDefaultEphemeralStorage,
		// Container默认请求（requests）
		ContainerDefaultRequestCPU:              in.ContainerDefaultRequestCpu,
		ContainerDefaultRequestMemory:           in.ContainerDefaultRequestMemory,
		ContainerDefaultRequestEphemeralStorage: in.ContainerDefaultRequestEphemeralStorage,
	}

	// 使用事务更新两个表
	err = l.svcCtx.OnecProjectClusterModel.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		_, err = clusterClient.Namespaces().Update(ns)
		if err != nil {
			l.Logger.Errorf("更新命名空间失败: %v", err)
			return errorx.Msg("更新命名空间失败")
		}
		// 更新 resourceQuota
		err := clusterClient.Namespaces().CreateOrUpdateResourceQuota(resourceQuotaReq)
		if err != nil {
			l.Logger.Errorf("更新资源限制失败: %v", err)
			return errorx.Msg("更新资源限制失败")
		}
		// 创建 limitRange
		err = clusterClient.Namespaces().CreateOrUpdateLimitRange(reqLimitRange)
		if err != nil {
			l.Logger.Errorf("创建 limitRange 失败: %v", err)
			return errorx.Msg("创建 limitRange 失败")
		}

		// 1. 计算资源变化量
		inCpu, err := utils.CPUToCores(in.CpuAllocated)
		if err != nil {
			return fmt.Errorf("CPU转换失败: %v", err)
		}
		wsCpu, err := utils.CPUToCores(workspace.CpuAllocated)
		if err != nil {
			return fmt.Errorf("CPU转换失败: %v", err)
		}
		inMem, err := utils.MemoryToBytes(in.MemAllocated)
		if err != nil {
			return fmt.Errorf("内存转换失败: %v", err)
		}
		wsMem, err := utils.MemoryToBytes(workspace.MemAllocated)
		if err != nil {
			return fmt.Errorf("内存转换失败: %v", err)
		}
		inGpu, err := utils.GPUToCount(in.GpuAllocated)
		if err != nil {
			return fmt.Errorf("GPU转换失败: %v", err)
		}
		wsGpu, err := utils.GPUToCount(workspace.GpuAllocated)
		if err != nil {
			return fmt.Errorf("GPU转换失败: %v", err)
		}
		inStorage, err := utils.MemoryToGiB(in.StorageAllocated)
		if err != nil {
			return fmt.Errorf("存储转换失败: %v", err)
		}
		wsStorage, err := utils.MemoryToGiB(workspace.StorageAllocated)
		if err != nil {
			return fmt.Errorf("存储转换失败: %v", err)
		}
		inEphStorage, err := utils.MemoryToGiB(in.EphemeralStorageAllocated)
		if err != nil {
			return fmt.Errorf("临时存储转换错误: %v", err)
		}
		wsEphStorage, err := utils.MemoryToGiB(workspace.EphemeralStorageAllocated)
		if err != nil {
			return fmt.Errorf("临时存储转换错误: %v", err)
		}
		cpuDelta := inCpu - wsCpu
		memDelta := inMem - wsMem
		storageDelta := inStorage - wsStorage
		gpuDelta := inGpu - wsGpu
		podsDelta := in.PodsAllocated - workspace.PodsAllocated
		configmapDelta := in.ConfigmapAllocated - workspace.ConfigmapAllocated
		secretDelta := in.SecretAllocated - workspace.SecretAllocated
		pvcDelta := in.PvcAllocated - workspace.PvcAllocated
		ephemeralStorageDelta := inEphStorage - wsEphStorage
		serviceDelta := in.ServiceAllocated - workspace.ServiceAllocated
		loadbalancersDelta := in.LoadbalancersAllocated - workspace.LoadbalancersAllocated
		nodeportsDelta := in.NodeportsAllocated - workspace.NodeportsAllocated
		deploymentsDelta := in.DeploymentsAllocated - workspace.DeploymentsAllocated
		jobsDelta := in.JobsAllocated - workspace.JobsAllocated
		cronjobsDelta := in.CronjobsAllocated - workspace.CronjobsAllocated
		daemonsetsDelta := in.DaemonsetsAllocated - workspace.DaemonsetsAllocated
		statefulsetsDelta := in.StatefulsetsAllocated - workspace.StatefulsetsAllocated
		ingressesDelta := in.IngressesAllocated - workspace.IngressesAllocated

		// 2. 更新项目集群配额（调整已分配的资源）
		updateProjectClusterSQL := `UPDATE onec_project_cluster SET 
			cpu_allocated = cpu_allocated + ?,
			mem_allocated = mem_allocated + ?,
			storage_allocated = storage_allocated + ?,
			gpu_allocated = gpu_allocated + ?,
			pods_allocated = pods_allocated + ?,
			configmap_allocated = configmap_allocated + ?,
			secret_allocated = secret_allocated + ?,
			pvc_allocated = pvc_allocated + ?,
			ephemeral_storage_allocated = ephemeral_storage_allocated + ?,
			service_allocated = service_allocated + ?,
			loadbalancers_allocated = loadbalancers_allocated + ?,
			nodeports_allocated = nodeports_allocated + ?,
			deployments_allocated = deployments_allocated + ?,
			jobs_allocated = jobs_allocated + ?,
			cronjobs_allocated = cronjobs_allocated + ?,
			daemonsets_allocated = daemonsets_allocated + ?,
			statefulsets_allocated = statefulsets_allocated + ?,
			ingresses_allocated = ingresses_allocated + ?,
			updated_at = ?
		WHERE id = ? AND is_deleted = 0`

		_, err = l.svcCtx.OnecProjectClusterModel.TransOnSql(ctx, session, projectCluster.Id, updateProjectClusterSQL,
			cpuDelta,
			memDelta,
			storageDelta,
			gpuDelta,
			podsDelta,
			configmapDelta,
			secretDelta,
			pvcDelta,
			ephemeralStorageDelta,
			serviceDelta,
			loadbalancersDelta,
			nodeportsDelta,
			deploymentsDelta,
			jobsDelta,
			cronjobsDelta,
			daemonsetsDelta,
			statefulsetsDelta,
			ingressesDelta,
			time.Now(),
			projectCluster.Id,
		)
		if err != nil {
			l.Logger.Errorf("更新项目集群配额失败: %v", err)
			return fmt.Errorf("更新项目集群配额失败: %v", err)
		}

		// 3. 更新工作空间信息
		updateWorkspaceSQL := `UPDATE onec_project_workspace SET 
			name = ?, description = ?,
			cpu_allocated = ?, mem_allocated = ?, storage_allocated = ?, gpu_allocated = ?, pods_allocated = ?,
			configmap_allocated = ?, secret_allocated = ?, pvc_allocated = ?, ephemeral_storage_allocated = ?,
			service_allocated = ?, loadbalancers_allocated = ?, nodeports_allocated = ?,
			deployments_allocated = ?, jobs_allocated = ?, cronjobs_allocated = ?,
			daemonsets_allocated = ?, statefulsets_allocated = ?, ingresses_allocated = ?,
			pod_max_cpu = ?, pod_max_memory = ?, pod_max_ephemeral_storage = ?,
			pod_min_cpu = ?, pod_min_memory = ?, pod_min_ephemeral_storage = ?,
			container_max_cpu = ?, container_max_memory = ?, container_max_ephemeral_storage = ?,
			container_min_cpu = ?, container_min_memory = ?, container_min_ephemeral_storage = ?,
			container_default_cpu = ?, container_default_memory = ?, container_default_ephemeral_storage = ?,
			container_default_request_cpu = ?, container_default_request_memory = ?, container_default_request_ephemeral_storage = ?,
			updated_by = ?, updated_at = ?
		WHERE id = ? AND is_deleted = 0`

		_, err = l.svcCtx.OnecProjectWorkspaceModel.TransOnSql(ctx, session, workspace.Id, updateWorkspaceSQL,
			in.Name, in.Description,
			in.CpuAllocated, in.MemAllocated, in.StorageAllocated, in.GpuAllocated, in.PodsAllocated,
			in.ConfigmapAllocated, in.SecretAllocated, in.PvcAllocated, in.EphemeralStorageAllocated,
			in.ServiceAllocated, in.LoadbalancersAllocated, in.NodeportsAllocated,
			in.DeploymentsAllocated, in.JobsAllocated, in.CronjobsAllocated,
			in.DaemonsetsAllocated, in.StatefulsetsAllocated, in.IngressesAllocated,
			in.PodMaxCpu, in.PodMaxMemory, in.PodMaxEphemeralStorage,
			in.PodMinCpu, in.PodMinMemory, in.PodMinEphemeralStorage,
			in.ContainerMaxCpu, in.ContainerMaxMemory, in.ContainerMaxEphemeralStorage,
			in.ContainerMinCpu, in.ContainerMinMemory, in.ContainerMinEphemeralStorage,
			in.ContainerDefaultCpu, in.ContainerDefaultMemory, in.ContainerDefaultEphemeralStorage,
			in.ContainerDefaultRequestCpu, in.ContainerDefaultRequestMemory, in.ContainerDefaultRequestEphemeralStorage,
			in.UpdatedBy, time.Now(),
			workspace.Id,
		)
		if err != nil {
			l.Logger.Errorf("更新工作空间失败: %v", err)
			return fmt.Errorf("更新工作空间失败: %v", err)
		}

		return nil
	})

	if err != nil {
		l.Logger.Errorf("事务执行失败: %v", err)
		return nil, errorx.Msg("更新工作空间失败")
	}

	// ============ 事务成功后手动删除所有相关缓存 ============
	l.Logger.Infof("事务执行成功，开始清理缓存")

	// 1. 删除 ProjectCluster 的缓存（2个键）
	if err := l.svcCtx.OnecProjectClusterModel.DeleteCache(l.ctx, projectCluster.Id); err != nil {
		// 缓存删除失败只记录日志，不影响主流程
		l.Logger.Errorf("删除项目集群缓存失败 [id=%d]: %v", projectCluster.Id, err)
	} else {
		l.Logger.Infof("成功删除项目集群缓存 [id=%d, clusterUuid:projectId=%s:%d]",
			projectCluster.Id, projectCluster.ClusterUuid, projectCluster.ProjectId)
	}

	// 2. 删除 ProjectWorkspace 的缓存（2个键）
	if err := l.svcCtx.OnecProjectWorkspaceModel.DeleteCache(l.ctx, workspace.Id); err != nil {
		l.Logger.Errorf("删除工作空间缓存失败 [id=%d]: %v", workspace.Id, err)
	} else {
		l.Logger.Infof("成功删除工作空间缓存 [id=%d, projectClusterId:namespace=%d:%s]",
			workspace.Id, workspace.ProjectClusterId, workspace.Namespace)
	}

	l.Logger.Infof("缓存清理完成")
	// ========================================================

	l.Logger.Infof("更新工作空间成功，ID: %d", in.Id)
	return &pb.UpdateOnecProjectWorkspaceResp{}, nil
}

// checkResourceAvailable 检查资源变更是否超出配额
func (l *ProjectWorkspaceUpdateLogic) checkResourceAvailable(projectCluster *model.OnecProjectCluster, workspace *model.OnecProjectWorkspace, in *pb.UpdateOnecProjectWorkspaceReq) error {
	// CPU检查 - 使用 cpu_capacity（超分后容量）
	// 新的总分配量 = 集群当前已分配 - 旧工作空间分配 + 新工作空间分配
	inCpu, err := utils.CPUToCores(in.CpuAllocated)
	if err != nil {
		return fmt.Errorf("CPU转换错误: %v", err)
	}
	cpu, err := utils.CPUToCores(workspace.CpuAllocated)
	if err != nil {
		return fmt.Errorf("CPU转换错误: %v", err)
	}
	oldCpuAllocated, err := utils.CPUToCores(projectCluster.CpuAllocated)
	if err != nil {
		return fmt.Errorf("CPU转换错误: %v", err)
	}
	oldCpuCapacity, err := utils.CPUToCores(projectCluster.CpuCapacity)
	if err != nil {
		return fmt.Errorf("CPU转换错误: %v", err)
	}
	newCpuTotalAllocated := oldCpuAllocated - cpu + inCpu

	if newCpuTotalAllocated > oldCpuCapacity {
		l.Logger.Errorf("CPU资源不足，容量: %v，更新后总分配: %.2f核", projectCluster.CpuCapacity, newCpuTotalAllocated)
		return fmt.Errorf("CPU资源不足，容量: %v，更新后总分配: %.2f核", projectCluster.CpuCapacity, newCpuTotalAllocated)
	}

	// 内存检查 - 使用 mem_capacity（超分后容量）
	inMem, err := utils.MemoryToGiB(in.MemAllocated)
	if err != nil {
		return fmt.Errorf("内存转换错误: %v", err)
	}
	wsMem, err := utils.MemoryToGiB(workspace.MemAllocated)
	if err != nil {
		return fmt.Errorf("内存转换错误: %v", err)
	}
	oldMemAllocated, err := utils.MemoryToGiB(projectCluster.MemAllocated)
	if err != nil {
		return fmt.Errorf("内存转换错误: %v", err)
	}
	oldMemCapacity, err := utils.MemoryToGiB(projectCluster.MemCapacity)
	if err != nil {
		return fmt.Errorf("内存转换错误: %v", err)
	}
	newMemTotalAllocated := oldMemAllocated - wsMem + inMem
	if newMemTotalAllocated > oldMemCapacity {
		l.Logger.Errorf("内存资源不足，容量: %s，更新后总分配: %.2f GiB", projectCluster.MemCapacity, newMemTotalAllocated)
		return fmt.Errorf("内存资源不足，容量: %v GiB，更新后总分配: %.2f GiB", projectCluster.MemCapacity, newMemTotalAllocated)
	}

	// 存储检查 - 使用 storage_limit（不支持超分）
	inStorage, err := utils.MemoryToGiB(in.StorageAllocated)
	if err != nil {
		return fmt.Errorf("存储转换错误: %v", err)
	}
	wsStorage, err := utils.MemoryToGiB(workspace.StorageAllocated)
	if err != nil {
		return fmt.Errorf("存储转换错误: %v", err)
	}
	oldStorageAllocated, err := utils.MemoryToGiB(projectCluster.StorageAllocated)
	if err != nil {
		return fmt.Errorf("存储转换错误: %v", err)
	}
	oldStorageLimit, err := utils.MemoryToGiB(projectCluster.StorageLimit)
	if err != nil {
		return fmt.Errorf("存储转换错误: %v", err)
	}
	newStorageTotalAllocated := oldStorageAllocated - wsStorage + inStorage
	if newStorageTotalAllocated > oldStorageLimit {
		l.Logger.Errorf("存储资源不足，限制: %s，更新后总分配: %d GiB", projectCluster.StorageLimit, newStorageTotalAllocated)
		return fmt.Errorf("存储资源不足，限制: %s，更新后总分配: %d GiB", projectCluster.StorageLimit, newStorageTotalAllocated)
	}

	// GPU检查 - 使用 gpu_capacity（超分后容量）
	ingpu, err := utils.GPUToCount(in.GpuAllocated)
	if err != nil {
		return fmt.Errorf("GPU转换失败: %v", err)
	}
	ws, err := utils.GPUToCount(workspace.GpuAllocated)
	if err != nil {
		return fmt.Errorf("GPU转换失败: %v", err)
	}
	oldGpuAllocated, err := utils.GPUToCount(projectCluster.GpuAllocated)
	if err != nil {
		return fmt.Errorf("GPU转换失败: %v", err)
	}
	oldGpuCapacity, err := utils.GPUToCount(projectCluster.GpuCapacity)
	if err != nil {
		return fmt.Errorf("GPU转换失败: %v", err)
	}
	newGpuTotalAllocated := oldGpuAllocated - ws + ingpu
	if newGpuTotalAllocated > oldGpuCapacity {
		l.Logger.Errorf("GPU资源不足，容量: %s，更新后总分配: %.2f个", projectCluster.GpuCapacity, newGpuTotalAllocated)
		return fmt.Errorf("GPU资源不足，容量: %s，更新后总分配: %.2f个", projectCluster.GpuCapacity, newGpuTotalAllocated)
	}

	// Pods检查 - 使用 pods_limit
	newPodsTotalAllocated := projectCluster.PodsAllocated - workspace.PodsAllocated + in.PodsAllocated
	if newPodsTotalAllocated > projectCluster.PodsLimit {
		l.Logger.Errorf("Pod配额不足，限制: %d个，更新后总分配: %d个", projectCluster.PodsLimit, newPodsTotalAllocated)
		return fmt.Errorf("Pod配额不足，限制: %d个，更新后总分配: %d个", projectCluster.PodsLimit, newPodsTotalAllocated)
	}

	// ConfigMap检查 - 使用 configmap_limit
	newConfigmapTotalAllocated := projectCluster.ConfigmapAllocated - workspace.ConfigmapAllocated + in.ConfigmapAllocated
	if newConfigmapTotalAllocated > projectCluster.ConfigmapLimit {
		l.Logger.Errorf("ConfigMap配额不足，限制: %d个，更新后总分配: %d个", projectCluster.ConfigmapLimit, newConfigmapTotalAllocated)
		return fmt.Errorf("ConfigMap配额不足，限制: %d个，更新后总分配: %d个", projectCluster.ConfigmapLimit, newConfigmapTotalAllocated)
	}

	// Secret检查 - 使用 secret_limit
	newSecretTotalAllocated := projectCluster.SecretAllocated - workspace.SecretAllocated + in.SecretAllocated
	if newSecretTotalAllocated > projectCluster.SecretLimit {
		l.Logger.Errorf("Secret配额不足，限制: %d个，更新后总分配: %d个", projectCluster.SecretLimit, newSecretTotalAllocated)
		return fmt.Errorf("Secret配额不足，限制: %d个，更新后总分配: %d个", projectCluster.SecretLimit, newSecretTotalAllocated)
	}

	// PVC检查 - 使用 pvc_limit
	newPvcTotalAllocated := projectCluster.PvcAllocated - workspace.PvcAllocated + in.PvcAllocated
	if newPvcTotalAllocated > projectCluster.PvcLimit {
		l.Logger.Errorf("PVC配额不足，限制: %d个，更新后总分配: %d个", projectCluster.PvcLimit, newPvcTotalAllocated)
		return fmt.Errorf("PVC配额不足，限制: %d个，更新后总分配: %d个", projectCluster.PvcLimit, newPvcTotalAllocated)
	}

	// EphemeralStorage检查 - 使用 ephemeral_storage_limit
	inEphemeralStorage, err := utils.MemoryToGiB(in.EphemeralStorageAllocated)
	if err != nil {
		return fmt.Errorf("临时存储转换错误: %v", err)
	}
	wsEphemeralStorage, err := utils.MemoryToGiB(workspace.EphemeralStorageAllocated)
	if err != nil {
		return fmt.Errorf("临时存储转换错误: %v", err)
	}
	oldEphemeralStorageAllocated, err := utils.MemoryToGiB(projectCluster.EphemeralStorageAllocated)
	if err != nil {
		return fmt.Errorf("临时存储转换错误: %v", err)
	}
	oldEphemeralStorageLimit, err := utils.MemoryToGiB(projectCluster.EphemeralStorageLimit)
	if err != nil {
		return fmt.Errorf("临时存储转换错误: %v", err)
	}
	newEphemeralStorageTotalAllocated := oldEphemeralStorageAllocated - wsEphemeralStorage + inEphemeralStorage
	if newEphemeralStorageTotalAllocated > oldEphemeralStorageLimit {
		l.Logger.Errorf("临时存储配额不足，限制: %s，更新后总分配: %d GiB", projectCluster.EphemeralStorageLimit, newEphemeralStorageTotalAllocated)
		return fmt.Errorf("临时存储配额不足，限制: %s，更新后总分配: %d GiB", projectCluster.EphemeralStorageLimit, newEphemeralStorageTotalAllocated)
	}

	// Service检查 - 使用 service_limit
	newServiceTotalAllocated := projectCluster.ServiceAllocated - workspace.ServiceAllocated + in.ServiceAllocated
	if newServiceTotalAllocated > projectCluster.ServiceLimit {
		l.Logger.Errorf("Service配额不足，限制: %d个，更新后总分配: %d个", projectCluster.ServiceLimit, newServiceTotalAllocated)
		return fmt.Errorf("Service配额不足，限制: %d个，更新后总分配: %d个", projectCluster.ServiceLimit, newServiceTotalAllocated)
	}

	// LoadBalancer检查 - 使用 loadbalancers_limit
	newLoadbalancersTotalAllocated := projectCluster.LoadbalancersAllocated - workspace.LoadbalancersAllocated + in.LoadbalancersAllocated
	if newLoadbalancersTotalAllocated > projectCluster.LoadbalancersLimit {
		l.Logger.Errorf("LoadBalancer配额不足，限制: %d个，更新后总分配: %d个", projectCluster.LoadbalancersLimit, newLoadbalancersTotalAllocated)
		return fmt.Errorf("LoadBalancer配额不足，限制: %d个，更新后总分配: %d个", projectCluster.LoadbalancersLimit, newLoadbalancersTotalAllocated)
	}

	// NodePort检查 - 使用 nodeports_limit
	newNodeportsTotalAllocated := projectCluster.NodeportsAllocated - workspace.NodeportsAllocated + in.NodeportsAllocated
	if newNodeportsTotalAllocated > projectCluster.NodeportsLimit {
		l.Logger.Errorf("NodePort配额不足，限制: %d个，更新后总分配: %d个", projectCluster.NodeportsLimit, newNodeportsTotalAllocated)
		return fmt.Errorf("NodePort配额不足，限制: %d个，更新后总分配: %d个", projectCluster.NodeportsLimit, newNodeportsTotalAllocated)
	}

	// Deployment检查 - 使用 deployments_limit
	newDeploymentsTotalAllocated := projectCluster.DeploymentsAllocated - workspace.DeploymentsAllocated + in.DeploymentsAllocated
	if newDeploymentsTotalAllocated > projectCluster.DeploymentsLimit {
		l.Logger.Errorf("Deployment配额不足，限制: %d个，更新后总分配: %d个", projectCluster.DeploymentsLimit, newDeploymentsTotalAllocated)
		return fmt.Errorf("Deployment配额不足，限制: %d个，更新后总分配: %d个", projectCluster.DeploymentsLimit, newDeploymentsTotalAllocated)
	}

	// Jobs检查 - 使用 jobs_limit
	newJobsTotalAllocated := projectCluster.JobsAllocated - workspace.JobsAllocated + in.JobsAllocated
	if newJobsTotalAllocated > projectCluster.JobsLimit {
		l.Logger.Errorf("Job配额不足，限制: %d个，更新后总分配: %d个", projectCluster.JobsLimit, newJobsTotalAllocated)
		return fmt.Errorf("Job配额不足，限制: %d个，更新后总分配: %d个", projectCluster.JobsLimit, newJobsTotalAllocated)
	}

	// CronJobs检查 - 使用 cronjobs_limit
	newCronjobsTotalAllocated := projectCluster.CronjobsAllocated - workspace.CronjobsAllocated + in.CronjobsAllocated
	if newCronjobsTotalAllocated > projectCluster.CronjobsLimit {
		l.Logger.Errorf("CronJob配额不足，限制: %d个，更新后总分配: %d个", projectCluster.CronjobsLimit, newCronjobsTotalAllocated)
		return fmt.Errorf("CronJob配额不足，限制: %d个，更新后总分配: %d个", projectCluster.CronjobsLimit, newCronjobsTotalAllocated)
	}

	// DaemonSets检查 - 使用 daemonsets_limit
	newDaemonsetsTotalAllocated := projectCluster.DaemonsetsAllocated - workspace.DaemonsetsAllocated + in.DaemonsetsAllocated
	if newDaemonsetsTotalAllocated > projectCluster.DaemonsetsLimit {
		l.Logger.Errorf("DaemonSet配额不足，限制: %d个，更新后总分配: %d个", projectCluster.DaemonsetsLimit, newDaemonsetsTotalAllocated)
		return fmt.Errorf("DaemonSet配额不足，限制: %d个，更新后总分配: %d个", projectCluster.DaemonsetsLimit, newDaemonsetsTotalAllocated)
	}

	// StatefulSets检查 - 使用 statefulsets_limit
	newStatefulsetsTotalAllocated := projectCluster.StatefulsetsAllocated - workspace.StatefulsetsAllocated + in.StatefulsetsAllocated
	if newStatefulsetsTotalAllocated > projectCluster.StatefulsetsLimit {
		l.Logger.Errorf("StatefulSet配额不足，限制: %d个，更新后总分配: %d个", projectCluster.StatefulsetsLimit, newStatefulsetsTotalAllocated)
		return fmt.Errorf("StatefulSet配额不足，限制: %d个，更新后总分配: %d个", projectCluster.StatefulsetsLimit, newStatefulsetsTotalAllocated)
	}

	// Ingresses检查 - 使用 ingresses_limit
	newIngressesTotalAllocated := projectCluster.IngressesAllocated - workspace.IngressesAllocated + in.IngressesAllocated
	if newIngressesTotalAllocated > projectCluster.IngressesLimit {
		l.Logger.Errorf("Ingress配额不足，限制: %d个，更新后总分配: %d个", projectCluster.IngressesLimit, newIngressesTotalAllocated)
		return fmt.Errorf("Ingress配额不足，限制: %d个，更新后总分配: %d个", projectCluster.IngressesLimit, newIngressesTotalAllocated)
	}

	return nil
}
