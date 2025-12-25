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

type ProjectClusterUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectClusterUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectClusterUpdateLogic {
	return &ProjectClusterUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectClusterUpdate 更新项目集群配额
func (l *ProjectClusterUpdateLogic) ProjectClusterUpdate(in *pb.UpdateOnecProjectClusterReq) (*pb.UpdateOnecProjectClusterResp, error) {
	l.Infof("开始更新项目集群配额，ID: %d", in.Id)

	// 查询项目集群配额是否存在
	projectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查询项目集群配额失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("项目集群配额不存在")
	}

	// 查询集群资源情况，判断资源是否充足
	clusterResource, err := l.svcCtx.OnecClusterResourceModel.FindOneByClusterUuid(l.ctx, projectCluster.ClusterUuid)
	if err != nil {
		l.Errorf("查询集群资源失败，集群UUID: %s, 错误: %v", projectCluster.ClusterUuid, err)
		return nil, errorx.Msg("查询集群资源失败")
	}

	// 验证资源配额是否可行
	if err := l.validateResourceQuota(clusterResource, projectCluster, in); err != nil {
		l.Errorf("资源配额验证失败: %v", err)
		return nil, errorx.Msg(err.Error())
	}

	// 使用事务更新两个表
	err = l.svcCtx.OnecProjectClusterModel.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 1. 计算资源变化量（使用 limit 进行计算）
		newCpuLimit, err := utils.CPUToCores(in.CpuLimit)
		if err != nil {
			return fmt.Errorf("CPU转换失败: %v", err)
		}
		oldCpuLimit, err := utils.CPUToCores(projectCluster.CpuLimit)
		if err != nil {
			return fmt.Errorf("CPU转换失败: %v", err)
		}

		newCpuCapacity, err := utils.CPUToCores(in.CpuCapacity)
		if err != nil {
			return fmt.Errorf("CPU容量转换失败: %v", err)
		}
		oldCpuCapacity, err := utils.CPUToCores(projectCluster.CpuCapacity)
		if err != nil {
			return fmt.Errorf("CPU容量转换失败: %v", err)
		}

		newMemLimit, err := utils.MemoryToGiB(in.MemLimit)
		if err != nil {
			return fmt.Errorf("内存转换失败: %v", err)
		}
		oldMemLimit, err := utils.MemoryToGiB(projectCluster.MemLimit)
		if err != nil {
			return fmt.Errorf("内存转换失败: %v", err)
		}

		newMemCapacity, err := utils.MemoryToGiB(in.MemCapacity)
		if err != nil {
			return fmt.Errorf("内存容量转换失败: %v", err)
		}
		oldMemCapacity, err := utils.MemoryToGiB(projectCluster.MemCapacity)
		if err != nil {
			return fmt.Errorf("内存容量转换失败: %v", err)
		}

		newStorageLimit, err := utils.MemoryToGiB(in.StorageLimit)
		if err != nil {
			return fmt.Errorf("存储转换失败: %v", err)
		}
		oldStorageLimit, err := utils.MemoryToGiB(projectCluster.StorageLimit)
		if err != nil {
			return fmt.Errorf("存储转换失败: %v", err)
		}

		newGpuLimit, err := utils.GPUToCount(in.GpuLimit)
		if err != nil {
			return fmt.Errorf("GPU转换失败: %v", err)
		}
		oldGpuLimit, err := utils.GPUToCount(projectCluster.GpuLimit)
		if err != nil {
			return fmt.Errorf("GPU转换失败: %v", err)
		}

		newGpuCapacity, err := utils.GPUToCount(in.GpuCapacity)
		if err != nil {
			return fmt.Errorf("GPU容量转换失败: %v", err)
		}
		oldGpuCapacity, err := utils.GPUToCount(projectCluster.GpuCapacity)
		if err != nil {
			return fmt.Errorf("GPU容量转换失败: %v", err)
		}

		// 计算 delta（变化量）
		cpuLimitDelta := newCpuLimit - oldCpuLimit
		cpuCapacityDelta := newCpuCapacity - oldCpuCapacity
		memLimitDelta := newMemLimit - oldMemLimit
		memCapacityDelta := newMemCapacity - oldMemCapacity
		storageLimitDelta := newStorageLimit - oldStorageLimit
		gpuLimitDelta := newGpuLimit - oldGpuLimit
		gpuCapacityDelta := newGpuCapacity - oldGpuCapacity
		podsLimitDelta := in.PodsLimit - projectCluster.PodsLimit

		// 2. 更新项目集群配额
		updateProjectClusterSQL := `UPDATE onec_project_cluster SET
			cpu_limit = ?, cpu_overcommit_ratio = ?, cpu_capacity = ?,
			mem_limit = ?, mem_overcommit_ratio = ?, mem_capacity = ?,
			storage_limit = ?,
			gpu_limit = ?, gpu_overcommit_ratio = ?, gpu_capacity = ?,
			pods_limit = ?, configmap_limit = ?, secret_limit = ?, pvc_limit = ?,
			ephemeral_storage_limit = ?, service_limit = ?, loadbalancers_limit = ?,
			nodeports_limit = ?, deployments_limit = ?, jobs_limit = ?,
			cronjobs_limit = ?, daemonsets_limit = ?, statefulsets_limit = ?, ingresses_limit = ?,
			updated_by = ?, updated_at = ?
		WHERE id = ? AND is_deleted = 0`

		_, err = l.svcCtx.OnecProjectClusterModel.TransOnSql(ctx, session, projectCluster.Id, updateProjectClusterSQL,
			in.CpuLimit, in.CpuOvercommitRatio, in.CpuCapacity,
			in.MemLimit, in.MemOvercommitRatio, in.MemCapacity,
			in.StorageLimit,
			in.GpuLimit, in.GpuOvercommitRatio, in.GpuCapacity,
			in.PodsLimit, in.ConfigmapLimit, in.SecretLimit, in.PvcLimit,
			in.EphemeralStorageLimit, in.ServiceLimit, in.LoadbalancersLimit,
			in.NodeportsLimit, in.DeploymentsLimit, in.JobsLimit,
			in.CronjobsLimit, in.DaemonsetsLimit, in.StatefulsetsLimit, in.IngressesLimit,
			in.UpdatedBy, time.Now(),
			projectCluster.Id,
		)
		if err != nil {
			l.Errorf("更新项目集群配额失败: %v", err)
			return fmt.Errorf("更新项目集群配额失败: %v", err)
		}

		// 3. 更新集群资源统计（只有变化量不为0时才需要更新）
		if cpuLimitDelta != 0 || cpuCapacityDelta != 0 || memLimitDelta != 0 || memCapacityDelta != 0 ||
			storageLimitDelta != 0 || gpuLimitDelta != 0 || gpuCapacityDelta != 0 || podsLimitDelta != 0 {

			updateClusterResourceSQL := `UPDATE onec_cluster_resource SET
				cpu_allocated_total = cpu_allocated_total + ?,
				cpu_capacity_total = cpu_capacity_total + ?,
				mem_allocated_total = mem_allocated_total + ?,
				mem_capacity_total = mem_capacity_total + ?,
				storage_allocated_total = storage_allocated_total + ?,
				gpu_allocated_total = gpu_allocated_total + ?,
				gpu_capacity_total = gpu_capacity_total + ?,
				pods_allocated_total = pods_allocated_total + ?
			WHERE cluster_uuid = ? AND is_deleted = 0`

			_, err = l.svcCtx.OnecClusterResourceModel.TransOnSql(ctx, session, clusterResource.Id, updateClusterResourceSQL,
				cpuLimitDelta,     // CPU实际分配总量变化
				cpuCapacityDelta,  // CPU超分后总容量变化
				memLimitDelta,     // 内存实际分配总量变化
				memCapacityDelta,  // 内存超分后总容量变化
				storageLimitDelta, // 存储分配总量变化
				gpuLimitDelta,     // GPU实际分配总量变化
				gpuCapacityDelta,  // GPU超分后总容量变化
				podsLimitDelta,    // Pod分配总量变化
				projectCluster.ClusterUuid,
			)
			if err != nil {
				l.Errorf("更新集群资源统计失败: %v", err)
				return fmt.Errorf("更新集群资源统计失败: %v", err)
			}
		}

		return nil
	})

	if err != nil {
		l.Errorf("事务执行失败: %v", err)
		return nil, errorx.Msg("更新项目集群配额失败")
	}

	// ============ 事务成功后手动删除所有相关缓存 ============
	l.Infof("事务执行成功，开始清理缓存")

	// 1. 删除 ProjectCluster 的缓存
	if err := l.svcCtx.OnecProjectClusterModel.DeleteCache(l.ctx, projectCluster.Id); err != nil {
		// 缓存删除失败只记录日志，不影响主流程
		l.Errorf("删除项目集群缓存失败 [id=%d]: %v", projectCluster.Id, err)
	} else {
		l.Infof("成功删除项目集群缓存 [id=%d]", projectCluster.Id)
	}

	// 2. 删除 ClusterResource 的缓存
	if err := l.svcCtx.OnecClusterResourceModel.DeleteCache(l.ctx, clusterResource.Id); err != nil {
		l.Errorf("删除集群资源缓存失败 [id=%d, clusterUuid=%s]: %v",
			clusterResource.Id, clusterResource.ClusterUuid, err)
	} else {
		l.Infof("成功删除集群资源缓存 [id=%d, clusterUuid=%s]",
			clusterResource.Id, clusterResource.ClusterUuid)
	}

	l.Infof("缓存清理完成")
	// ========================================================

	l.Infof("更新项目集群配额成功，ID: %d", in.Id)
	return &pb.UpdateOnecProjectClusterResp{}, nil
}

// validateResourceQuota 验证资源配额是否可行
// 校验规则：
// 1. 新的 limit（实际分配）不能超过集群物理容量
// 2. 新的 capacity（超分后容量）不能小于已分配给工作空间的 allocated 总和
func (l *ProjectClusterUpdateLogic) validateResourceQuota(clusterResource *model.OnecClusterResource,
	projectCluster *model.OnecProjectCluster, in *pb.UpdateOnecProjectClusterReq) error {

	// ============ CPU检查 ============
	// 1. 检查新的 cpu_limit 是否超出集群物理容量
	newCpuLimit, err := utils.CPUToCores(in.CpuLimit)
	if err != nil {
		return fmt.Errorf("CPU转换错误: %v", err)
	}
	oldCpuLimit, err := utils.CPUToCores(projectCluster.CpuLimit)
	if err != nil {
		return fmt.Errorf("CPU转换错误: %v", err)
	}

	cpuLimitDelta := newCpuLimit - oldCpuLimit
	newCpuAllocatedTotal := clusterResource.CpuAllocatedTotal + cpuLimitDelta
	if newCpuAllocatedTotal > clusterResource.CpuPhysicalCapacity {
		l.Errorf("CPU配额超出物理容量，物理容量: %.2f核，更新后总分配: %.2f核",
			clusterResource.CpuPhysicalCapacity, newCpuAllocatedTotal)
		return fmt.Errorf("CPU配额超出物理容量，物理容量: %.2f核，更新后总分配: %.2f核",
			clusterResource.CpuPhysicalCapacity, newCpuAllocatedTotal)
	}

	// 2. 检查新的 cpu_capacity 是否能满足已分配给工作空间的资源
	newCpuCapacity, err := utils.CPUToCores(in.CpuCapacity)
	if err != nil {
		return fmt.Errorf("CPU容量转换错误: %v", err)
	}
	oldCpuAllocated, err := utils.CPUToCores(projectCluster.CpuAllocated)
	if err != nil {
		return fmt.Errorf("CPU allocated转换错误: %v", err)
	}
	if oldCpuAllocated > newCpuCapacity {
		l.Errorf("CPU新容量不足，工作空间已分配: %.2f核，新容量: %.2f核",
			oldCpuAllocated, newCpuCapacity)
		return fmt.Errorf("CPU新容量不足，工作空间已分配: %.2f核，新容量: %.2f核",
			oldCpuAllocated, newCpuCapacity)
	}

	// ============ 内存检查 ============
	// 1. 检查新的 mem_limit 是否超出集群物理容量
	newMemLimit, err := utils.MemoryToGiB(in.MemLimit)
	if err != nil {
		return fmt.Errorf("内存转换错误: %v", err)
	}
	oldMemLimit, err := utils.MemoryToGiB(projectCluster.MemLimit)
	if err != nil {
		return fmt.Errorf("内存转换错误: %v", err)
	}

	memLimitDelta := newMemLimit - oldMemLimit
	newMemAllocatedTotal := clusterResource.MemAllocatedTotal + memLimitDelta
	if newMemAllocatedTotal > clusterResource.MemPhysicalCapacity {
		l.Errorf("内存配额超出物理容量，物理容量: %.2f GiB，更新后总分配: %.2f GiB",
			clusterResource.MemPhysicalCapacity, newMemAllocatedTotal)
		return fmt.Errorf("内存配额超出物理容量，物理容量: %.2f GiB，更新后总分配: %.2f GiB",
			clusterResource.MemPhysicalCapacity, newMemAllocatedTotal)
	}

	// 2. 检查新的 mem_capacity 是否能满足已分配给工作空间的资源
	newMemCapacity, err := utils.MemoryToGiB(in.MemCapacity)
	if err != nil {
		return fmt.Errorf("内存容量转换错误: %v", err)
	}
	oldMemAllocated, err := utils.MemoryToGiB(projectCluster.MemAllocated)
	if err != nil {
		return fmt.Errorf("内存allocated转换错误: %v", err)
	}
	if oldMemAllocated > newMemCapacity {
		l.Errorf("内存新容量不足，工作空间已分配: %.2f GiB，新容量: %.2f GiB",
			oldMemAllocated, newMemCapacity)
		return fmt.Errorf("内存新容量不足，工作空间已分配: %.2f GiB，新容量: %.2f GiB",
			oldMemAllocated, newMemCapacity)
	}

	// ============ 存储检查 ============
	newStorageLimit, err := utils.MemoryToGiB(in.StorageLimit)
	if err != nil {
		return fmt.Errorf("存储转换错误: %v", err)
	}
	oldStorageLimit, err := utils.MemoryToGiB(projectCluster.StorageLimit)
	if err != nil {
		return fmt.Errorf("存储转换错误: %v", err)
	}

	storageLimitDelta := newStorageLimit - oldStorageLimit
	newStorageAllocatedTotal := clusterResource.StorageAllocatedTotal + storageLimitDelta
	if newStorageAllocatedTotal > clusterResource.StoragePhysicalCapacity {
		l.Errorf("存储配额超出物理容量，物理容量: %.2f GiB，更新后总分配: %.2f GiB",
			clusterResource.StoragePhysicalCapacity, newStorageAllocatedTotal)
		return fmt.Errorf("存储配额超出物理容量，物理容量: %.2f GiB，更新后总分配: %.2f GiB",
			clusterResource.StoragePhysicalCapacity, newStorageAllocatedTotal)
	}

	oldStorageAllocated, err := utils.MemoryToGiB(projectCluster.StorageAllocated)
	if err != nil {
		return fmt.Errorf("存储allocated转换错误: %v", err)
	}
	if oldStorageAllocated > newStorageLimit {
		l.Errorf("存储新限制不足，工作空间已分配: %.2f GiB，新限制: %.2f GiB",
			oldStorageAllocated, newStorageLimit)
		return fmt.Errorf("存储新限制不足，工作空间已分配: %.2f GiB，新限制: %.2f GiB",
			oldStorageAllocated, newStorageLimit)
	}

	// ============ GPU检查 ============
	newGpuLimit, err := utils.GPUToCount(in.GpuLimit)
	if err != nil {
		return fmt.Errorf("GPU转换错误: %v", err)
	}
	oldGpuLimit, err := utils.GPUToCount(projectCluster.GpuLimit)
	if err != nil {
		return fmt.Errorf("GPU转换错误: %v", err)
	}

	gpuLimitDelta := newGpuLimit - oldGpuLimit
	newGpuAllocatedTotal := clusterResource.GpuAllocatedTotal + gpuLimitDelta
	if newGpuAllocatedTotal > clusterResource.GpuPhysicalCapacity {
		l.Errorf("GPU配额超出物理容量，物理容量: %.2f个，更新后总分配: %.2f个",
			clusterResource.GpuPhysicalCapacity, newGpuAllocatedTotal)
		return fmt.Errorf("GPU配额超出物理容量，物理容量: %.2f个，更新后总分配: %.2f个",
			clusterResource.GpuPhysicalCapacity, newGpuAllocatedTotal)
	}

	newGpuCapacity, err := utils.GPUToCount(in.GpuCapacity)
	if err != nil {
		return fmt.Errorf("GPU容量转换错误: %v", err)
	}
	oldGpuAllocated, err := utils.GPUToCount(projectCluster.GpuAllocated)
	if err != nil {
		return fmt.Errorf("GPU allocated转换错误: %v", err)
	}
	if oldGpuAllocated > newGpuCapacity {
		l.Errorf("GPU新容量不足，工作空间已分配: %.2f个，新容量: %.2f个",
			oldGpuAllocated, newGpuCapacity)
		return fmt.Errorf("GPU新容量不足，工作空间已分配: %.2f个，新容量: %.2f个",
			oldGpuAllocated, newGpuCapacity)
	}

	// ============ Pod检查 ============
	podsLimitDelta := in.PodsLimit - projectCluster.PodsLimit
	newPodsAllocatedTotal := clusterResource.PodsAllocatedTotal + podsLimitDelta
	if newPodsAllocatedTotal > clusterResource.PodsPhysicalCapacity {
		l.Errorf("Pod配额超出物理容量，物理容量: %d个，更新后总分配: %d个",
			clusterResource.PodsPhysicalCapacity, newPodsAllocatedTotal)
		return fmt.Errorf("Pod配额超出物理容量，物理容量: %d个，更新后总分配: %d个",
			clusterResource.PodsPhysicalCapacity, newPodsAllocatedTotal)
	}

	if projectCluster.PodsAllocated > in.PodsLimit {
		l.Errorf("Pod新限制不足，工作空间已分配: %d个，新限制: %d个",
			projectCluster.PodsAllocated, in.PodsLimit)
		return fmt.Errorf("Pod新限制不足，工作空间已分配: %d个，新限制: %d个",
			projectCluster.PodsAllocated, in.PodsLimit)
	}

	// ============ 其他资源检查（ConfigMap, Secret, PVC等）============
	if in.ConfigmapLimit > 0 && projectCluster.ConfigmapAllocated > in.ConfigmapLimit {
		return fmt.Errorf("ConfigMap新限制不足，工作空间已分配: %d个，新限制: %d个",
			projectCluster.ConfigmapAllocated, in.ConfigmapLimit)
	}

	if in.SecretLimit > 0 && projectCluster.SecretAllocated > in.SecretLimit {
		return fmt.Errorf("Secret新限制不足，工作空间已分配: %d个，新限制: %d个",
			projectCluster.SecretAllocated, in.SecretLimit)
	}

	if in.PvcLimit > 0 && projectCluster.PvcAllocated > in.PvcLimit {
		return fmt.Errorf("PVC新限制不足，工作空间已分配: %d个，新限制: %d个",
			projectCluster.PvcAllocated, in.PvcLimit)
	}

	newEphStorageLimit, err := utils.MemoryToGiB(in.EphemeralStorageLimit)
	if err != nil {
		return fmt.Errorf("临时存储转换错误: %v", err)
	}
	oldEphStorageAllocated, err := utils.MemoryToGiB(projectCluster.EphemeralStorageAllocated)
	if err != nil {
		return fmt.Errorf("临时存储allocated转换错误: %v", err)
	}
	if newEphStorageLimit > 0 && oldEphStorageAllocated > newEphStorageLimit {
		return fmt.Errorf("临时存储新限制不足，工作空间已分配: %.2f GiB，新限制: %.2f GiB",
			oldEphStorageAllocated, newEphStorageLimit)
	}

	if in.ServiceLimit > 0 && projectCluster.ServiceAllocated > in.ServiceLimit {
		return fmt.Errorf("Service新限制不足，工作空间已分配: %d个，新限制: %d个",
			projectCluster.ServiceAllocated, in.ServiceLimit)
	}

	if in.LoadbalancersLimit > 0 && projectCluster.LoadbalancersAllocated > in.LoadbalancersLimit {
		return fmt.Errorf("LoadBalancer新限制不足，工作空间已分配: %d个，新限制: %d个",
			projectCluster.LoadbalancersAllocated, in.LoadbalancersLimit)
	}

	if in.NodeportsLimit > 0 && projectCluster.NodeportsAllocated > in.NodeportsLimit {
		return fmt.Errorf("NodePort新限制不足，工作空间已分配: %d个，新限制: %d个",
			projectCluster.NodeportsAllocated, in.NodeportsLimit)
	}

	if in.DeploymentsLimit > 0 && projectCluster.DeploymentsAllocated > in.DeploymentsLimit {
		return fmt.Errorf("deployment新限制不足，工作空间已分配: %d个，新限制: %d个",
			projectCluster.DeploymentsAllocated, in.DeploymentsLimit)
	}

	if in.JobsLimit > 0 && projectCluster.JobsAllocated > in.JobsLimit {
		return fmt.Errorf("Job新限制不足，工作空间已分配: %d个，新限制: %d个",
			projectCluster.JobsAllocated, in.JobsLimit)
	}

	if in.CronjobsLimit > 0 && projectCluster.CronjobsAllocated > in.CronjobsLimit {
		return fmt.Errorf("CronJob新限制不足，工作空间已分配: %d个，新限制: %d个",
			projectCluster.CronjobsAllocated, in.CronjobsLimit)
	}

	if in.DaemonsetsLimit > 0 && projectCluster.DaemonsetsAllocated > in.DaemonsetsLimit {
		return fmt.Errorf("DaemonSet新限制不足，工作空间已分配: %d个，新限制: %d个",
			projectCluster.DaemonsetsAllocated, in.DaemonsetsLimit)
	}

	if in.StatefulsetsLimit > 0 && projectCluster.StatefulsetsAllocated > in.StatefulsetsLimit {
		return fmt.Errorf("StatefulSet新限制不足，工作空间已分配: %d个，新限制: %d个",
			projectCluster.StatefulsetsAllocated, in.StatefulsetsLimit)
	}

	if in.IngressesLimit > 0 && projectCluster.IngressesAllocated > in.IngressesLimit {
		return fmt.Errorf("ingress新限制不足，工作空间已分配: %d个，新限制: %d个",
			projectCluster.IngressesAllocated, in.IngressesLimit)
	}

	return nil
}
