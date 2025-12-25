package managerservicelogic

import (
	"context"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type MigrateWorkspaceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMigrateWorkspaceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MigrateWorkspaceLogic {
	return &MigrateWorkspaceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// MigrateWorkspace 迁移工作空间到新项目
func (l *MigrateWorkspaceLogic) MigrateWorkspace(in *pb.MigrateWorkspaceReq) (*pb.MigrateWorkspaceResp, error) {
	l.Infof("开始迁移工作空间，workspaceId: %d, newProjectId: %d", in.WorkspaceId, in.NewProjectId)

	// 1. 查询要迁移的 workspace
	workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, in.WorkspaceId)
	if err != nil {
		l.Errorf("查询工作空间失败，ID: %d, 错误: %v", in.WorkspaceId, err)
		return nil, errorx.Msg("工作空间不存在")
	}

	// 2. 查询旧的 project_cluster
	oldProjectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, workspace.ProjectClusterId)
	if err != nil {
		l.Errorf("查询旧项目集群失败，ID: %d, 错误: %v", workspace.ProjectClusterId, err)
		return nil, errorx.Msg("旧项目集群不存在")
	}

	// 3. 检查是否是同一个项目（不需要迁移）
	if oldProjectCluster.ProjectId == in.NewProjectId {
		l.Errorf("工作空间已在目标项目中，无需迁移")
		return nil, errorx.Msg("工作空间已在目标项目中，无需迁移")
	}

	// 4. 查询新的 project 是否存在
	_, err = l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.NewProjectId)
	if err != nil {
		l.Errorf("查询新项目失败，ID: %d, 错误: %v", in.NewProjectId, err)
		return nil, errorx.Msg("目标项目不存在")
	}

	// 5. 查询新项目在同一个集群上的 project_cluster
	newProjectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOneByClusterUuidProjectId(
		l.ctx, workspace.ClusterUuid, in.NewProjectId)
	if err != nil {
		l.Errorf("目标项目在集群 %s 上没有配额配置", workspace.ClusterUuid)
		return nil, errorx.Msg("目标项目在此集群上没有配额配置，请先为目标项目在集群上分配资源")
	}

	// 6. 检查新项目集群的资源是否充足
	if err := l.checkResourceAvailable(newProjectCluster, workspace); err != nil {
		l.Errorf("目标项目资源不足: %v", err)
		return nil, errorx.Msg(err.Error())
	}

	// 7. 使用事务进行迁移
	err = l.svcCtx.OnecProjectClusterModel.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 7.1 转换资源量
		cpuAllocated, err := utils.CPUToCores(workspace.CpuAllocated)
		if err != nil {
			return fmt.Errorf("CPU转换失败: %v", err)
		}

		memAllocated, err := utils.MemoryToGiB(workspace.MemAllocated)
		if err != nil {
			return fmt.Errorf("内存转换失败: %v", err)
		}

		storageAllocated, err := utils.MemoryToGiB(workspace.StorageAllocated)
		if err != nil {
			return fmt.Errorf("存储转换失败: %v", err)
		}

		gpuAllocated, err := utils.GPUToCount(workspace.GpuAllocated)
		if err != nil {
			return fmt.Errorf("GPU转换失败: %v", err)
		}

		ephemeralStorageAllocated, err := utils.MemoryToGiB(workspace.EphemeralStorageAllocated)
		if err != nil {
			return fmt.Errorf("临时存储转换失败: %v", err)
		}

		// 7.2 更新旧 project_cluster：减少 allocated 资源
		updateOldProjectClusterSQL := `UPDATE onec_project_cluster SET
			cpu_allocated = cpu_allocated - ?,
			mem_allocated = mem_allocated - ?,
			storage_allocated = storage_allocated - ?,
			gpu_allocated = gpu_allocated - ?,
			pods_allocated = pods_allocated - ?,
			configmap_allocated = configmap_allocated - ?,
			secret_allocated = secret_allocated - ?,
			pvc_allocated = pvc_allocated - ?,
			ephemeral_storage_allocated = ephemeral_storage_allocated - ?,
			service_allocated = service_allocated - ?,
			loadbalancers_allocated = loadbalancers_allocated - ?,
			nodeports_allocated = nodeports_allocated - ?,
			deployments_allocated = deployments_allocated - ?,
			jobs_allocated = jobs_allocated - ?,
			cronjobs_allocated = cronjobs_allocated - ?,
			daemonsets_allocated = daemonsets_allocated - ?,
			statefulsets_allocated = statefulsets_allocated - ?,
			ingresses_allocated = ingresses_allocated - ?,
			updated_at = ?
		WHERE id = ? AND is_deleted = 0`

		_, err = l.svcCtx.OnecProjectClusterModel.TransOnSql(ctx, session, oldProjectCluster.Id,
			updateOldProjectClusterSQL,
			cpuAllocated,
			memAllocated,
			storageAllocated,
			gpuAllocated,
			workspace.PodsAllocated,
			workspace.ConfigmapAllocated,
			workspace.SecretAllocated,
			workspace.PvcAllocated,
			ephemeralStorageAllocated,
			workspace.ServiceAllocated,
			workspace.LoadbalancersAllocated,
			workspace.NodeportsAllocated,
			workspace.DeploymentsAllocated,
			workspace.JobsAllocated,
			workspace.CronjobsAllocated,
			workspace.DaemonsetsAllocated,
			workspace.StatefulsetsAllocated,
			workspace.IngressesAllocated,
			time.Now(),
			oldProjectCluster.Id,
		)
		if err != nil {
			l.Errorf("更新旧项目集群配额失败: %v", err)
			return fmt.Errorf("更新旧项目集群配额失败: %v", err)
		}

		// 7.3 更新新 project_cluster：增加 allocated 资源
		updateNewProjectClusterSQL := `UPDATE onec_project_cluster SET
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

		_, err = l.svcCtx.OnecProjectClusterModel.TransOnSql(ctx, session, newProjectCluster.Id,
			updateNewProjectClusterSQL,
			cpuAllocated,
			memAllocated,
			storageAllocated,
			gpuAllocated,
			workspace.PodsAllocated,
			workspace.ConfigmapAllocated,
			workspace.SecretAllocated,
			workspace.PvcAllocated,
			ephemeralStorageAllocated,
			workspace.ServiceAllocated,
			workspace.LoadbalancersAllocated,
			workspace.NodeportsAllocated,
			workspace.DeploymentsAllocated,
			workspace.JobsAllocated,
			workspace.CronjobsAllocated,
			workspace.DaemonsetsAllocated,
			workspace.StatefulsetsAllocated,
			workspace.IngressesAllocated,
			time.Now(),
			newProjectCluster.Id,
		)
		if err != nil {
			l.Errorf("更新新项目集群配额失败: %v", err)
			return fmt.Errorf("更新新项目集群配额失败: %v", err)
		}

		// 7.4 更新 workspace 的 project_cluster_id
		updateWorkspaceSQL := `UPDATE onec_project_workspace SET
			project_cluster_id = ?,
			updated_at = ?
		WHERE id = ? AND is_deleted = 0`

		_, err = l.svcCtx.OnecProjectWorkspaceModel.TransOnSql(ctx, session, workspace.Id,
			updateWorkspaceSQL,
			newProjectCluster.Id,
			time.Now(),
			workspace.Id,
		)
		if err != nil {
			l.Errorf("更新工作空间所属项目失败: %v", err)
			return fmt.Errorf("更新工作空间所属项目失败: %v", err)
		}

		return nil
	})

	if err != nil {
		l.Errorf("迁移工作空间事务执行失败: %v", err)
		return nil, errorx.Msg("迁移工作空间失败")
	}

	l.Infof("迁移工作空间成功，workspaceId: %d, 从项目 %d 迁移到项目 %d",
		in.WorkspaceId, oldProjectCluster.ProjectId, in.NewProjectId)

	return &pb.MigrateWorkspaceResp{}, nil
}

// checkResourceAvailable 检查目标项目集群的资源是否充足
func (l *MigrateWorkspaceLogic) checkResourceAvailable(projectCluster *model.OnecProjectCluster, workspace *model.OnecProjectWorkspace) error {
	// CPU检查 - 使用 cpu_capacity（超分后容量）
	wsCpu, err := utils.CPUToCores(workspace.CpuAllocated)
	if err != nil {
		return fmt.Errorf("CPU转换错误: %v", err)
	}
	pcCpuAllocated, err := utils.CPUToCores(projectCluster.CpuAllocated)
	if err != nil {
		return fmt.Errorf("CPU转换错误: %v", err)
	}
	pcCpuCapacity, err := utils.CPUToCores(projectCluster.CpuCapacity)
	if err != nil {
		return fmt.Errorf("CPU转换错误: %v", err)
	}
	newCpuTotalAllocated := pcCpuAllocated + wsCpu
	if newCpuTotalAllocated > pcCpuCapacity {
		l.Errorf("CPU资源不足，容量: %v，迁移后总分配: %.2f核", projectCluster.CpuCapacity, newCpuTotalAllocated)
		return fmt.Errorf("CPU资源不足，容量: %v，迁移后总分配: %.2f核", projectCluster.CpuCapacity, newCpuTotalAllocated)
	}

	// 内存检查 - 使用 mem_capacity（超分后容量）
	wsMem, err := utils.MemoryToGiB(workspace.MemAllocated)
	if err != nil {
		return fmt.Errorf("内存转换错误: %v", err)
	}
	pcMemAllocated, err := utils.MemoryToGiB(projectCluster.MemAllocated)
	if err != nil {
		return fmt.Errorf("内存转换错误: %v", err)
	}
	pcMemCapacity, err := utils.MemoryToGiB(projectCluster.MemCapacity)
	if err != nil {
		return fmt.Errorf("内存转换错误: %v", err)
	}
	newMemTotalAllocated := pcMemAllocated + wsMem
	if newMemTotalAllocated > pcMemCapacity {
		l.Errorf("内存资源不足，容量: %v，迁移后总分配: %.2f GiB", projectCluster.MemCapacity, newMemTotalAllocated)
		return fmt.Errorf("内存资源不足，容量: %v，迁移后总分配: %.2f GiB", projectCluster.MemCapacity, newMemTotalAllocated)
	}

	// 存储检查 - 使用 storage_limit（不支持超分）
	wsStorage, err := utils.MemoryToGiB(workspace.StorageAllocated)
	if err != nil {
		return fmt.Errorf("存储转换错误: %v", err)
	}
	pcStorageAllocated, err := utils.MemoryToGiB(projectCluster.StorageAllocated)
	if err != nil {
		return fmt.Errorf("存储转换错误: %v", err)
	}
	pcStorageLimit, err := utils.MemoryToGiB(projectCluster.StorageLimit)
	if err != nil {
		return fmt.Errorf("存储转换错误: %v", err)
	}
	newStorageTotalAllocated := pcStorageAllocated + wsStorage
	if newStorageTotalAllocated > pcStorageLimit {
		l.Errorf("存储资源不足，限制: %v，迁移后总分配: %.2f GiB", projectCluster.StorageLimit, newStorageTotalAllocated)
		return fmt.Errorf("存储资源不足，限制: %v，迁移后总分配: %.2f GiB", projectCluster.StorageLimit, newStorageTotalAllocated)
	}

	// GPU检查 - 使用 gpu_capacity（超分后容量）
	wsGpu, err := utils.GPUToCount(workspace.GpuAllocated)
	if err != nil {
		return fmt.Errorf("GPU转换失败: %v", err)
	}
	pcGpuAllocated, err := utils.GPUToCount(projectCluster.GpuAllocated)
	if err != nil {
		return fmt.Errorf("GPU转换失败: %v", err)
	}
	pcGpuCapacity, err := utils.GPUToCount(projectCluster.GpuCapacity)
	if err != nil {
		return fmt.Errorf("GPU转换失败: %v", err)
	}
	newGpuTotalAllocated := pcGpuAllocated + wsGpu
	if newGpuTotalAllocated > pcGpuCapacity {
		l.Errorf("GPU资源不足，容量: %v个，迁移后总分配: %.2f个", projectCluster.GpuCapacity, newGpuTotalAllocated)
		return fmt.Errorf("GPU资源不足，容量: %v个，迁移后总分配: %.2f个", projectCluster.GpuCapacity, newGpuTotalAllocated)
	}

	// Pod资源检查 - 使用 pods_limit
	newPodsTotalAllocated := projectCluster.PodsAllocated + workspace.PodsAllocated
	if newPodsTotalAllocated > projectCluster.PodsLimit {
		l.Errorf("Pod配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.PodsLimit, newPodsTotalAllocated)
		return fmt.Errorf("Pod配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.PodsLimit, newPodsTotalAllocated)
	}

	// ConfigMap检查 - 使用 configmap_limit
	newConfigmapTotalAllocated := projectCluster.ConfigmapAllocated + workspace.ConfigmapAllocated
	if newConfigmapTotalAllocated > projectCluster.ConfigmapLimit {
		l.Errorf("ConfigMap配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.ConfigmapLimit, newConfigmapTotalAllocated)
		return fmt.Errorf("ConfigMap配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.ConfigmapLimit, newConfigmapTotalAllocated)
	}

	// Secret检查 - 使用 secret_limit
	newSecretTotalAllocated := projectCluster.SecretAllocated + workspace.SecretAllocated
	if newSecretTotalAllocated > projectCluster.SecretLimit {
		l.Errorf("Secret配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.SecretLimit, newSecretTotalAllocated)
		return fmt.Errorf("Secret配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.SecretLimit, newSecretTotalAllocated)
	}

	// PVC检查 - 使用 pvc_limit
	newPvcTotalAllocated := projectCluster.PvcAllocated + workspace.PvcAllocated
	if newPvcTotalAllocated > projectCluster.PvcLimit {
		l.Errorf("PVC配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.PvcLimit, newPvcTotalAllocated)
		return fmt.Errorf("PVC配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.PvcLimit, newPvcTotalAllocated)
	}

	// EphemeralStorage检查 - 使用 ephemeral_storage_limit
	wsEphStorage, err := utils.MemoryToGiB(workspace.EphemeralStorageAllocated)
	if err != nil {
		return fmt.Errorf("临时存储转换错误: %v", err)
	}
	pcEphStorageAllocated, err := utils.MemoryToGiB(projectCluster.EphemeralStorageAllocated)
	if err != nil {
		return fmt.Errorf("临时存储转换错误: %v", err)
	}
	pcEphStorageLimit, err := utils.MemoryToGiB(projectCluster.EphemeralStorageLimit)
	if err != nil {
		return fmt.Errorf("临时存储转换错误: %v", err)
	}
	newEphemeralStorageTotalAllocated := pcEphStorageAllocated + wsEphStorage
	if newEphemeralStorageTotalAllocated > pcEphStorageLimit {
		l.Errorf("临时存储配额不足，限制: %v，迁移后总分配: %.2f GiB", projectCluster.EphemeralStorageLimit, newEphemeralStorageTotalAllocated)
		return fmt.Errorf("临时存储配额不足，限制: %v，迁移后总分配: %.2f GiB", projectCluster.EphemeralStorageLimit, newEphemeralStorageTotalAllocated)
	}

	// Service检查 - 使用 service_limit
	newServiceTotalAllocated := projectCluster.ServiceAllocated + workspace.ServiceAllocated
	if newServiceTotalAllocated > projectCluster.ServiceLimit {
		l.Errorf("Service配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.ServiceLimit, newServiceTotalAllocated)
		return fmt.Errorf("Service配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.ServiceLimit, newServiceTotalAllocated)
	}

	// LoadBalancer检查 - 使用 loadbalancers_limit
	newLoadbalancersTotalAllocated := projectCluster.LoadbalancersAllocated + workspace.LoadbalancersAllocated
	if newLoadbalancersTotalAllocated > projectCluster.LoadbalancersLimit {
		l.Errorf("LoadBalancer配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.LoadbalancersLimit, newLoadbalancersTotalAllocated)
		return fmt.Errorf("LoadBalancer配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.LoadbalancersLimit, newLoadbalancersTotalAllocated)
	}

	// NodePort检查 - 使用 nodeports_limit
	newNodeportsTotalAllocated := projectCluster.NodeportsAllocated + workspace.NodeportsAllocated
	if newNodeportsTotalAllocated > projectCluster.NodeportsLimit {
		l.Errorf("NodePort配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.NodeportsLimit, newNodeportsTotalAllocated)
		return fmt.Errorf("NodePort配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.NodeportsLimit, newNodeportsTotalAllocated)
	}

	// Deployment检查 - 使用 deployments_limit
	newDeploymentsTotalAllocated := projectCluster.DeploymentsAllocated + workspace.DeploymentsAllocated
	if newDeploymentsTotalAllocated > projectCluster.DeploymentsLimit {
		l.Errorf("Deployment配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.DeploymentsLimit, newDeploymentsTotalAllocated)
		return fmt.Errorf("Deployment配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.DeploymentsLimit, newDeploymentsTotalAllocated)
	}

	// Jobs检查 - 使用 jobs_limit
	newJobsTotalAllocated := projectCluster.JobsAllocated + workspace.JobsAllocated
	if newJobsTotalAllocated > projectCluster.JobsLimit {
		l.Errorf("Job配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.JobsLimit, newJobsTotalAllocated)
		return fmt.Errorf("Job配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.JobsLimit, newJobsTotalAllocated)
	}

	// CronJobs检查 - 使用 cronjobs_limit
	newCronjobsTotalAllocated := projectCluster.CronjobsAllocated + workspace.CronjobsAllocated
	if newCronjobsTotalAllocated > projectCluster.CronjobsLimit {
		l.Errorf("CronJob配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.CronjobsLimit, newCronjobsTotalAllocated)
		return fmt.Errorf("CronJob配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.CronjobsLimit, newCronjobsTotalAllocated)
	}

	// DaemonSets检查 - 使用 daemonsets_limit
	newDaemonsetsTotalAllocated := projectCluster.DaemonsetsAllocated + workspace.DaemonsetsAllocated
	if newDaemonsetsTotalAllocated > projectCluster.DaemonsetsLimit {
		l.Errorf("DaemonSet配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.DaemonsetsLimit, newDaemonsetsTotalAllocated)
		return fmt.Errorf("DaemonSet配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.DaemonsetsLimit, newDaemonsetsTotalAllocated)
	}

	// StatefulSets检查 - 使用 statefulsets_limit
	newStatefulsetsTotalAllocated := projectCluster.StatefulsetsAllocated + workspace.StatefulsetsAllocated
	if newStatefulsetsTotalAllocated > projectCluster.StatefulsetsLimit {
		l.Errorf("StatefulSet配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.StatefulsetsLimit, newStatefulsetsTotalAllocated)
		return fmt.Errorf("StatefulSet配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.StatefulsetsLimit, newStatefulsetsTotalAllocated)
	}

	// Ingresses检查 - 使用 ingresses_limit
	newIngressesTotalAllocated := projectCluster.IngressesAllocated + workspace.IngressesAllocated
	if newIngressesTotalAllocated > projectCluster.IngressesLimit {
		l.Errorf("Ingress配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.IngressesLimit, newIngressesTotalAllocated)
		return fmt.Errorf("Ingress配额不足，限制: %d个，迁移后总分配: %d个", projectCluster.IngressesLimit, newIngressesTotalAllocated)
	}

	return nil
}
