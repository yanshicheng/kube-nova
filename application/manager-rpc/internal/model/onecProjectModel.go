// 文件：./onecProjectModel.go

package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecProjectModel = (*customOnecProjectModel)(nil)

// ProjectStatistics 项目统计信息
type ProjectStatistics struct {
	AdminCount          int64 `db:"admin_count" json:"admin_count"`                     // 管理员总数
	ProjectClusterCount int64 `db:"project_cluster_count" json:"project_cluster_count"` // 项目集群总数
}

// ProjectResourceSummary 项目资源汇总统计
type ProjectResourceSummary struct {
	// CPU资源汇总
	CpuLimitTotal     float64 `json:"cpu_limit_total"`     // CPU总配额（所有集群的limit之和）
	CpuCapacityTotal  float64 `json:"cpu_capacity_total"`  // CPU总容量（所有集群的capacity之和）
	CpuAllocatedTotal float64 `json:"cpu_allocated_total"` // CPU总分配（所有集群的allocated之和）

	// 内存资源汇总
	MemLimitTotal     float64 `json:"mem_limit_total"`     // 内存总配额（GiB）
	MemCapacityTotal  float64 `json:"mem_capacity_total"`  // 内存总容量（GiB）
	MemAllocatedTotal float64 `json:"mem_allocated_total"` // 内存总分配（GiB）

	// 存储资源汇总
	StorageLimitTotal     float64 `json:"storage_limit_total"`     // 存储总配额（GiB）
	StorageAllocatedTotal float64 `json:"storage_allocated_total"` // 存储总分配（GiB）

	// GPU资源汇总
	GpuLimitTotal     float64 `json:"gpu_limit_total"`     // GPU总配额
	GpuCapacityTotal  float64 `json:"gpu_capacity_total"`  // GPU总容量
	GpuAllocatedTotal float64 `json:"gpu_allocated_total"` // GPU总分配

	// Pod资源汇总
	PodsLimitTotal     int64 `json:"pods_limit_total"`     // Pod总配额
	PodsAllocatedTotal int64 `json:"pods_allocated_total"` // Pod总分配

	// 其他资源汇总
	ConfigmapAllocatedTotal        int64   `json:"configmap_allocated_total"`         // ConfigMap总分配
	SecretAllocatedTotal           int64   `json:"secret_allocated_total"`            // Secret总分配
	PvcAllocatedTotal              int64   `json:"pvc_allocated_total"`               // PVC总分配
	EphemeralStorageAllocatedTotal float64 `json:"ephemeral_storage_allocated_total"` // 临时存储总分配（GiB）
	ServiceAllocatedTotal          int64   `json:"service_allocated_total"`           // Service总分配
	LoadbalancersAllocatedTotal    int64   `json:"loadbalancers_allocated_total"`     // LoadBalancer总分配
	NodeportsAllocatedTotal        int64   `json:"nodeports_allocated_total"`         // NodePort总分配
	DeploymentsAllocatedTotal      int64   `json:"deployments_allocated_total"`       // Deployment总分配
	JobsAllocatedTotal             int64   `json:"jobs_allocated_total"`              // Job总分配
	CronjobsAllocatedTotal         int64   `json:"cronjobs_allocated_total"`          // CronJob总分配
	DaemonsetsAllocatedTotal       int64   `json:"daemonsets_allocated_total"`        // DaemonSet总分配
	StatefulsetsAllocatedTotal     int64   `json:"statefulsets_allocated_total"`      // StatefulSet总分配
	IngressesAllocatedTotal        int64   `json:"ingresses_allocated_total"`         // Ingress总分配
}

// WorkspaceResourceStats 工作空间资源统计结构体（项目集群维度）
type WorkspaceResourceStats struct {
	CpuAllocated              float64 `json:"cpu_allocated"`               // CPU已分配总量（核）
	MemAllocated              float64 `json:"mem_allocated"`               // 内存已分配总量（GiB）
	StorageAllocated          float64 `json:"storage_allocated"`           // 存储已分配总量（GiB）
	GpuAllocated              float64 `json:"gpu_allocated"`               // GPU已分配总量（个）
	PodsAllocated             int64   `json:"pods_allocated"`              // Pod已分配总量
	ConfigmapAllocated        int64   `json:"configmap_allocated"`         // ConfigMap已分配总量
	SecretAllocated           int64   `json:"secret_allocated"`            // Secret已分配总量
	PvcAllocated              int64   `json:"pvc_allocated"`               // PVC已分配总量
	EphemeralStorageAllocated float64 `json:"ephemeral_storage_allocated"` // 临时存储已分配总量（GiB）
	ServiceAllocated          int64   `json:"service_allocated"`           // Service已分配总量
	LoadbalancersAllocated    int64   `json:"loadbalancers_allocated"`     // LoadBalancer已分配总量
	NodeportsAllocated        int64   `json:"nodeports_allocated"`         // NodePort已分配总量
	DeploymentsAllocated      int64   `json:"deployments_allocated"`       // Deployment已分配总量
	JobsAllocated             int64   `json:"jobs_allocated"`              // Job已分配总量
	CronjobsAllocated         int64   `json:"cronjobs_allocated"`          // CronJob已分配总量
	DaemonsetsAllocated       int64   `json:"daemonsets_allocated"`        // DaemonSet已分配总量
	StatefulsetsAllocated     int64   `json:"statefulsets_allocated"`      // StatefulSet已分配总量
	IngressesAllocated        int64   `json:"ingresses_allocated"`         // Ingress已分配总量
}

type (
	OnecProjectModel interface {
		onecProjectModel
		// ========== 项目基础统计 ==========
		// GetProjectStatistics 获取项目统计信息（管理员数、集群数）
		GetProjectStatistics(ctx context.Context, projectId uint64) (*ProjectStatistics, error)

		// ========== 项目集群资源管理（原 OnecProjectClusterModel 的功能） ==========
		// GetWorkspaceResourceStatsByProjectCluster 获取指定项目集群下所有工作空间的资源分配统计
		GetWorkspaceResourceStatsByProjectCluster(ctx context.Context, projectClusterId uint64) (*WorkspaceResourceStats, error)
		// SyncProjectClusterResourceAllocation 同步工作空间资源分配到项目集群表
		SyncProjectClusterResourceAllocation(ctx context.Context, projectClusterId uint64) error

		// ========== 项目层面资源管理 ==========
		// SyncAllProjectClusters 同步项目下所有集群的资源分配
		SyncAllProjectClusters(ctx context.Context, projectId uint64) error
		// GetProjectResourceSummary 获取项目的资源汇总统计
		GetProjectResourceSummary(ctx context.Context, projectId uint64) (*ProjectResourceSummary, error)
	}

	customOnecProjectModel struct {
		*defaultOnecProjectModel
	}
)

// NewOnecProjectModel returns a model for the database table.
func NewOnecProjectModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecProjectModel {
	return &customOnecProjectModel{
		defaultOnecProjectModel: newOnecProjectModel(conn, c, opts...),
	}
}

// ========== 项目基础统计方法 ==========

// GetProjectStatistics 获取项目统计信息（管理员数量、项目集群数量）
func (m *customOnecProjectModel) GetProjectStatistics(ctx context.Context, projectId uint64) (*ProjectStatistics, error) {
	// 验证项目是否存在
	_, err := m.FindOne(ctx, projectId)
	if err != nil {
		return nil, fmt.Errorf("项目不存在: %v", err)
	}

	// 统计管理员数量和项目集群数量
	query := `
		SELECT
			(SELECT COUNT(*) FROM onec_project_admin
			 WHERE project_id = ? AND is_deleted = 0) as admin_count,
			(SELECT COUNT(*) FROM onec_project_cluster
			 WHERE project_id = ? AND is_deleted = 0) as project_cluster_count
	`

	var stats ProjectStatistics
	err = m.QueryRowNoCacheCtx(ctx, &stats, query, projectId, projectId)
	if err != nil {
		return nil, fmt.Errorf("查询项目统计信息失败: %v", err)
	}

	return &stats, nil
}

// ========== 项目集群资源管理方法（原 OnecProjectClusterModel 的功能） ==========

// GetWorkspaceResourceStatsByProjectCluster 获取指定项目集群下所有工作空间的资源分配统计
// 由于工作空间表的资源字段是VARCHAR字符串类型，需要在Go代码中进行转换和累加
func (m *customOnecProjectModel) GetWorkspaceResourceStatsByProjectCluster(ctx context.Context, projectClusterId uint64) (*WorkspaceResourceStats, error) {
	// 查询该项目集群下所有未删除的工作空间
	query := `
		SELECT
			cpu_allocated,
			mem_allocated,
			storage_allocated,
			gpu_allocated,
			pods_allocated,
			configmap_allocated,
			secret_allocated,
			pvc_allocated,
			ephemeral_storage_allocated,
			service_allocated,
			loadbalancers_allocated,
			nodeports_allocated,
			deployments_allocated,
			jobs_allocated,
			cronjobs_allocated,
			daemonsets_allocated,
			statefulsets_allocated,
			ingresses_allocated
		FROM onec_project_workspace
		WHERE project_cluster_id = ? AND is_deleted = 0
	`

	// 定义工作空间记录结构体（字符串类型）
	type WorkspaceRecord struct {
		CpuAllocated              string `db:"cpu_allocated"`
		MemAllocated              string `db:"mem_allocated"`
		StorageAllocated          string `db:"storage_allocated"`
		GpuAllocated              string `db:"gpu_allocated"`
		PodsAllocated             int64  `db:"pods_allocated"`
		ConfigmapAllocated        int64  `db:"configmap_allocated"`
		SecretAllocated           int64  `db:"secret_allocated"`
		PvcAllocated              int64  `db:"pvc_allocated"`
		EphemeralStorageAllocated string `db:"ephemeral_storage_allocated"`
		ServiceAllocated          int64  `db:"service_allocated"`
		LoadbalancersAllocated    int64  `db:"loadbalancers_allocated"`
		NodeportsAllocated        int64  `db:"nodeports_allocated"`
		DeploymentsAllocated      int64  `db:"deployments_allocated"`
		JobsAllocated             int64  `db:"jobs_allocated"`
		CronjobsAllocated         int64  `db:"cronjobs_allocated"`
		DaemonsetsAllocated       int64  `db:"daemonsets_allocated"`
		StatefulsetsAllocated     int64  `db:"statefulsets_allocated"`
		IngressesAllocated        int64  `db:"ingresses_allocated"`
	}

	var workspaces []*WorkspaceRecord
	err := m.QueryRowsNoCacheCtx(ctx, &workspaces, query, projectClusterId)
	if err != nil && err != sqlx.ErrNotFound {
		return nil, fmt.Errorf("查询工作空间列表失败: %v", err)
	}

	// 初始化统计结果
	stats := &WorkspaceResourceStats{}

	// 如果没有工作空间，直接返回零值统计
	if len(workspaces) == 0 {
		return stats, nil
	}

	// 遍历所有工作空间，累加资源
	for _, ws := range workspaces {
		// 转换并累加 CPU（核心数）
		cpu, err := utils.CPUToCores(ws.CpuAllocated)
		if err != nil {
			return nil, fmt.Errorf("CPU转换失败 [%s]: %v", ws.CpuAllocated, err)
		}
		stats.CpuAllocated += cpu

		// 转换并累加内存（GiB）
		mem, err := utils.MemoryToGiB(ws.MemAllocated)
		if err != nil {
			return nil, fmt.Errorf("内存转换失败 [%s]: %v", ws.MemAllocated, err)
		}
		stats.MemAllocated += mem

		// 转换并累加存储（GiB）
		storage, err := utils.MemoryToGiB(ws.StorageAllocated)
		if err != nil {
			return nil, fmt.Errorf("存储转换失败 [%s]: %v", ws.StorageAllocated, err)
		}
		stats.StorageAllocated += storage

		// 转换并累加 GPU（个数）
		gpu, err := utils.GPUToCount(ws.GpuAllocated)
		if err != nil {
			return nil, fmt.Errorf("GPU转换失败 [%s]: %v", ws.GpuAllocated, err)
		}
		stats.GpuAllocated += gpu

		// 转换并累加临时存储（GiB）
		ephStorage, err := utils.MemoryToGiB(ws.EphemeralStorageAllocated)
		if err != nil {
			return nil, fmt.Errorf("临时存储转换失败 [%s]: %v", ws.EphemeralStorageAllocated, err)
		}
		stats.EphemeralStorageAllocated += ephStorage

		// 累加整数类型的资源（不需要转换）
		stats.PodsAllocated += ws.PodsAllocated
		stats.ConfigmapAllocated += ws.ConfigmapAllocated
		stats.SecretAllocated += ws.SecretAllocated
		stats.PvcAllocated += ws.PvcAllocated
		stats.ServiceAllocated += ws.ServiceAllocated
		stats.LoadbalancersAllocated += ws.LoadbalancersAllocated
		stats.NodeportsAllocated += ws.NodeportsAllocated
		stats.DeploymentsAllocated += ws.DeploymentsAllocated
		stats.JobsAllocated += ws.JobsAllocated
		stats.CronjobsAllocated += ws.CronjobsAllocated
		stats.DaemonsetsAllocated += ws.DaemonsetsAllocated
		stats.StatefulsetsAllocated += ws.StatefulsetsAllocated
		stats.IngressesAllocated += ws.IngressesAllocated
	}

	return stats, nil
}

// SyncProjectClusterResourceAllocation 同步工作空间资源分配到项目集群表
// 统计该项目集群下所有工作空间的资源总和，更新到项目集群的 *_allocated 字段
func (m *customOnecProjectModel) SyncProjectClusterResourceAllocation(ctx context.Context, projectClusterId uint64) error {
	// 1. 查询项目集群记录，验证是否存在并获取缓存键所需信息
	projectClusterQuery := `
		SELECT pc.id, pc.cluster_uuid, pc.project_id
		FROM onec_project_cluster pc
		INNER JOIN onec_project p ON pc.project_id = p.id
		WHERE pc.id = ? AND pc.is_deleted = 0 AND p.is_deleted = 0
		LIMIT 1
	`
	type ProjectClusterInfo struct {
		Id          uint64 `db:"id"`
		ClusterUuid string `db:"cluster_uuid"`
		ProjectId   uint64 `db:"project_id"`
	}
	var projectCluster ProjectClusterInfo
	err := m.QueryRowNoCacheCtx(ctx, &projectCluster, projectClusterQuery, projectClusterId)
	if err != nil {
		return fmt.Errorf("查询项目集群记录失败: %v", err)
	}

	// 2. 获取工作空间资源统计
	stats, err := m.GetWorkspaceResourceStatsByProjectCluster(ctx, projectClusterId)
	if err != nil {
		return fmt.Errorf("获取工作空间资源统计失败: %v", err)
	}

	// 3. 构造更新SQL
	updateQuery := `
		UPDATE onec_project_cluster SET
			cpu_allocated = ?,
			mem_allocated = ?,
			storage_allocated = ?,
			gpu_allocated = ?,
			pods_allocated = ?,
			configmap_allocated = ?,
			secret_allocated = ?,
			pvc_allocated = ?,
			ephemeral_storage_allocated = ?,
			service_allocated = ?,
			loadbalancers_allocated = ?,
			nodeports_allocated = ?,
			deployments_allocated = ?,
			jobs_allocated = ?,
			cronjobs_allocated = ?,
			daemonsets_allocated = ?,
			statefulsets_allocated = ?,
			ingresses_allocated = ?,
			updated_at = NOW()
		WHERE id = ? AND is_deleted = 0
	`

	// 4. 生成缓存键（使用项目集群表的缓存前缀）
	cacheKeyClusterUuidProjectId := fmt.Sprintf("cache:ikubeops:onecProjectCluster:clusterUuid:projectId:%v:%v",
		projectCluster.ClusterUuid,
		projectCluster.ProjectId)
	cacheKeyId := fmt.Sprintf("cache:ikubeops:onecProjectCluster:id:%v",
		projectClusterId)

	// 5. 执行更新操作并清除缓存
	_, err = m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		return conn.ExecCtx(ctx, updateQuery,
			fmt.Sprintf("%.2f", stats.CpuAllocated),
			fmt.Sprintf("%.2fGi", stats.MemAllocated),
			fmt.Sprintf("%.2fGi", stats.StorageAllocated),
			fmt.Sprintf("%.0f", stats.GpuAllocated),
			stats.PodsAllocated,
			stats.ConfigmapAllocated,
			stats.SecretAllocated,
			stats.PvcAllocated,
			fmt.Sprintf("%.2fGi", stats.EphemeralStorageAllocated),
			stats.ServiceAllocated,
			stats.LoadbalancersAllocated,
			stats.NodeportsAllocated,
			stats.DeploymentsAllocated,
			stats.JobsAllocated,
			stats.CronjobsAllocated,
			stats.DaemonsetsAllocated,
			stats.StatefulsetsAllocated,
			stats.IngressesAllocated,
			projectClusterId,
		)
	}, cacheKeyClusterUuidProjectId, cacheKeyId)

	if err != nil {
		return fmt.Errorf("更新项目集群资源分配失败: %v", err)
	}

	return nil
}

// ========== 项目层面资源管理方法 ==========

// SyncAllProjectClusters 同步项目下所有集群的资源分配
// 遍历项目下所有的项目集群关系，逐个同步工作空间资源到项目集群表
func (m *customOnecProjectModel) SyncAllProjectClusters(ctx context.Context, projectId uint64) error {
	// 1. 验证项目是否存在
	_, err := m.FindOne(ctx, projectId)
	if err != nil {
		return fmt.Errorf("项目不存在: %v", err)
	}

	// 2. 查询该项目下所有的项目集群关系
	query := `
		SELECT id
		FROM onec_project_cluster
		WHERE project_id = ? AND is_deleted = 0
	`

	type ProjectClusterRecord struct {
		Id uint64 `db:"id"`
	}

	var projectClusters []*ProjectClusterRecord
	err = m.QueryRowsNoCacheCtx(ctx, &projectClusters, query, projectId)
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return fmt.Errorf("查询项目集群列表失败: %v", err)
	}

	// 3. 如果没有项目集群，直接返回
	if len(projectClusters) == 0 {
		return nil
	}

	// 4. 遍历每个项目集群，执行同步
	var syncErrors []string
	for _, pc := range projectClusters {
		err := m.SyncProjectClusterResourceAllocation(ctx, pc.Id)
		if err != nil {
			// 记录错误但继续处理其他集群
			syncErrors = append(syncErrors, fmt.Sprintf("同步项目集群[%d]失败: %v", pc.Id, err))
			continue
		}
	}

	// 5. 如果有同步错误，返回汇总错误
	if len(syncErrors) > 0 {
		return fmt.Errorf("部分项目集群同步失败: %v", syncErrors)
	}

	return nil
}

// GetProjectResourceSummary 获取项目的资源汇总统计
// 统计项目在所有集群上的资源配额和使用情况总和
func (m *customOnecProjectModel) GetProjectResourceSummary(ctx context.Context, projectId uint64) (*ProjectResourceSummary, error) {
	// 1. 验证项目是否存在
	_, err := m.FindOne(ctx, projectId)
	if err != nil {
		return nil, fmt.Errorf("项目不存在: %v", err)
	}

	// 2. 查询该项目下所有项目集群的资源（字符串类型，需要转换）
	query := `
		SELECT
			cpu_limit,
			cpu_capacity,
			cpu_allocated,
			mem_limit,
			mem_capacity,
			mem_allocated,
			storage_limit,
			storage_allocated,
			gpu_limit,
			gpu_capacity,
			gpu_allocated,
			pods_limit,
			pods_allocated,
			configmap_allocated,
			secret_allocated,
			pvc_allocated,
			ephemeral_storage_allocated,
			service_allocated,
			loadbalancers_allocated,
			nodeports_allocated,
			deployments_allocated,
			jobs_allocated,
			cronjobs_allocated,
			daemonsets_allocated,
			statefulsets_allocated,
			ingresses_allocated
		FROM onec_project_cluster
		WHERE project_id = ? AND is_deleted = 0
	`

	// 定义记录结构体
	type ProjectClusterRecord struct {
		CpuLimit                  string `db:"cpu_limit"`
		CpuCapacity               string `db:"cpu_capacity"`
		CpuAllocated              string `db:"cpu_allocated"`
		MemLimit                  string `db:"mem_limit"`
		MemCapacity               string `db:"mem_capacity"`
		MemAllocated              string `db:"mem_allocated"`
		StorageLimit              string `db:"storage_limit"`
		StorageAllocated          string `db:"storage_allocated"`
		GpuLimit                  string `db:"gpu_limit"`
		GpuCapacity               string `db:"gpu_capacity"`
		GpuAllocated              string `db:"gpu_allocated"`
		PodsLimit                 int64  `db:"pods_limit"`
		PodsAllocated             int64  `db:"pods_allocated"`
		ConfigmapAllocated        int64  `db:"configmap_allocated"`
		SecretAllocated           int64  `db:"secret_allocated"`
		PvcAllocated              int64  `db:"pvc_allocated"`
		EphemeralStorageAllocated string `db:"ephemeral_storage_allocated"`
		ServiceAllocated          int64  `db:"service_allocated"`
		LoadbalancersAllocated    int64  `db:"loadbalancers_allocated"`
		NodeportsAllocated        int64  `db:"nodeports_allocated"`
		DeploymentsAllocated      int64  `db:"deployments_allocated"`
		JobsAllocated             int64  `db:"jobs_allocated"`
		CronjobsAllocated         int64  `db:"cronjobs_allocated"`
		DaemonsetsAllocated       int64  `db:"daemonsets_allocated"`
		StatefulsetsAllocated     int64  `db:"statefulsets_allocated"`
		IngressesAllocated        int64  `db:"ingresses_allocated"`
	}

	var records []*ProjectClusterRecord
	err = m.QueryRowsNoCacheCtx(ctx, &records, query, projectId)
	if err != nil && err != sqlx.ErrNotFound {
		return nil, fmt.Errorf("查询项目集群资源失败: %v", err)
	}

	// 初始化统计结果
	summary := &ProjectResourceSummary{}

	// 如果没有记录，返回零值统计
	if len(records) == 0 {
		return summary, nil
	}

	// 遍历所有记录，转换并累加
	for _, record := range records {
		// CPU
		cpuLimit, _ := utils.CPUToCores(record.CpuLimit)
		summary.CpuLimitTotal += cpuLimit

		cpuCapacity, _ := utils.CPUToCores(record.CpuCapacity)
		summary.CpuCapacityTotal += cpuCapacity

		cpuAllocated, _ := utils.CPUToCores(record.CpuAllocated)
		summary.CpuAllocatedTotal += cpuAllocated

		// Memory
		memLimit, _ := utils.MemoryToGiB(record.MemLimit)
		summary.MemLimitTotal += memLimit

		memCapacity, _ := utils.MemoryToGiB(record.MemCapacity)
		summary.MemCapacityTotal += memCapacity

		memAllocated, _ := utils.MemoryToGiB(record.MemAllocated)
		summary.MemAllocatedTotal += memAllocated

		// Storage
		storageLimit, _ := utils.MemoryToGiB(record.StorageLimit)
		summary.StorageLimitTotal += storageLimit

		storageAllocated, _ := utils.MemoryToGiB(record.StorageAllocated)
		summary.StorageAllocatedTotal += storageAllocated

		// GPU
		gpuLimit, _ := utils.GPUToCount(record.GpuLimit)
		summary.GpuLimitTotal += gpuLimit

		gpuCapacity, _ := utils.GPUToCount(record.GpuCapacity)
		summary.GpuCapacityTotal += gpuCapacity

		gpuAllocated, _ := utils.GPUToCount(record.GpuAllocated)
		summary.GpuAllocatedTotal += gpuAllocated

		// Ephemeral Storage
		ephemeralStorage, _ := utils.MemoryToGiB(record.EphemeralStorageAllocated)
		summary.EphemeralStorageAllocatedTotal += ephemeralStorage

		// 整数类型直接累加
		summary.PodsLimitTotal += record.PodsLimit
		summary.PodsAllocatedTotal += record.PodsAllocated
		summary.ConfigmapAllocatedTotal += record.ConfigmapAllocated
		summary.SecretAllocatedTotal += record.SecretAllocated
		summary.PvcAllocatedTotal += record.PvcAllocated
		summary.ServiceAllocatedTotal += record.ServiceAllocated
		summary.LoadbalancersAllocatedTotal += record.LoadbalancersAllocated
		summary.NodeportsAllocatedTotal += record.NodeportsAllocated
		summary.DeploymentsAllocatedTotal += record.DeploymentsAllocated
		summary.JobsAllocatedTotal += record.JobsAllocated
		summary.CronjobsAllocatedTotal += record.CronjobsAllocated
		summary.DaemonsetsAllocatedTotal += record.DaemonsetsAllocated
		summary.StatefulsetsAllocatedTotal += record.StatefulsetsAllocated
		summary.IngressesAllocatedTotal += record.IngressesAllocated
	}
	return summary, nil
}
