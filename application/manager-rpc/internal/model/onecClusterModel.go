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

var _ OnecClusterModel = (*customOnecClusterModel)(nil)

// ProjectClusterStats 项目集群资源统计（集群维度）
type ProjectClusterStats struct {
	CpuLimitTotal     float64 `json:"cpu_limit_total"`     // CPU配额总和
	CpuCapacityTotal  float64 `json:"cpu_capacity_total"`  // CPU容量总和
	MemLimitTotal     float64 `json:"mem_limit_total"`     // 内存配额总和
	MemCapacityTotal  float64 `json:"mem_capacity_total"`  // 内存容量总和
	StorageLimitTotal float64 `json:"storage_limit_total"` // 存储配额总和
	GpuLimitTotal     float64 `json:"gpu_limit_total"`     // GPU配额总和
	GpuCapacityTotal  float64 `json:"gpu_capacity_total"`  // GPU容量总和
	PodsLimitTotal    int64   `json:"pods_limit_total"`    // Pod配额总和
}

type (
	OnecClusterModel interface {
		onecClusterModel
		// ========== 集群列表查询 ==========
		// GetAllClusters 获取所有集群列表（不分页）
		GetAllClusters(ctx context.Context) ([]*OnecCluster, error)

		// ========== 集群资源统计（原 OnecClusterResourceModel 的功能） ==========
		// GetProjectClusterStatsByClusterResource 获取集群资源下所有项目的资源统计（通过clusterResourceId）
		GetProjectClusterStatsByClusterResource(ctx context.Context, clusterResourceId uint64) (*ProjectClusterStats, error)
		// GetProjectClusterStatsByClusterUuid 根据集群UUID获取所有项目的资源统计
		GetProjectClusterStatsByClusterUuid(ctx context.Context, clusterUuid string) (*ProjectClusterStats, error)
		// SyncClusterResourceByResourceId 同步项目集群统计数据到集群资源表（通过clusterResourceId）
		SyncClusterResourceByResourceId(ctx context.Context, clusterResourceId uint64) error
		// SyncClusterResourceByUuid 同步项目集群统计数据到集群资源表（通过clusterUuid）
		SyncClusterResourceByUuid(ctx context.Context, clusterUuid string) error
		// SyncClusterResourceByClusterId 同步项目集群统计数据到集群资源表（通过clusterId）
		SyncClusterResourceByClusterId(ctx context.Context, clusterId uint64) error
		// SyncAllClusters 同步所有集群的资源统计
		SyncAllClusters(ctx context.Context) error
	}

	customOnecClusterModel struct {
		*defaultOnecClusterModel
	}
)

// NewOnecClusterModel returns a model for the database table.
func NewOnecClusterModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecClusterModel {
	return &customOnecClusterModel{
		defaultOnecClusterModel: newOnecClusterModel(conn, c, opts...),
	}
}

// ========== 集群列表查询方法 ==========

// GetAllClusters 获取所有集群列表（不分页）
func (m *customOnecClusterModel) GetAllClusters(ctx context.Context) ([]*OnecCluster, error) {
	query := `
		SELECT * FROM onec_cluster
		WHERE is_deleted = 0
		ORDER BY id ASC
	`

	var clusters []*OnecCluster
	err := m.QueryRowsNoCacheCtx(ctx, &clusters, query)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return []*OnecCluster{}, nil
		}
		return nil, fmt.Errorf("查询所有集群失败: %v", err)
	}

	return clusters, nil
}

// ========== 集群资源统计方法（原 OnecClusterResourceModel 的功能） ==========

// GetProjectClusterStatsByClusterResource 获取集群资源下所有项目的资源统计（通过clusterResourceId）
// 统计该集群下所有项目集群关系的资源配额和使用情况
func (m *customOnecClusterModel) GetProjectClusterStatsByClusterResource(ctx context.Context, clusterResourceId uint64) (*ProjectClusterStats, error) {
	// 1. 查询 onec_cluster_resource 记录，获取 cluster_uuid
	clusterResourceQuery := `
		SELECT cluster_uuid
		FROM onec_cluster_resource
		WHERE id = ? AND is_deleted = 0
		LIMIT 1
	`
	var clusterUuid string
	err := m.QueryRowNoCacheCtx(ctx, &clusterUuid, clusterResourceQuery, clusterResourceId)
	if err != nil {
		return nil, fmt.Errorf("查询集群资源记录失败: %v", err)
	}

	// 2. 根据 cluster_uuid 查询统计
	return m.GetProjectClusterStatsByClusterUuid(ctx, clusterUuid)
}

// GetProjectClusterStatsByClusterUuid 根据集群UUID获取所有项目的资源统计
// 统计该集群下所有项目集群关系的资源配额和使用情况
func (m *customOnecClusterModel) GetProjectClusterStatsByClusterUuid(ctx context.Context, clusterUuid string) (*ProjectClusterStats, error) {
	// 查询 onec_project_cluster 表，同时关联 onec_project
	// 注意：字段是字符串类型，需要在 Go 代码中转换
	query := `
		SELECT
			pc.cpu_limit,
			pc.cpu_capacity,
			pc.mem_limit,
			pc.mem_capacity,
			pc.storage_limit,
			pc.gpu_limit,
			pc.gpu_capacity,
			pc.pods_limit
		FROM onec_project_cluster pc
		INNER JOIN onec_project p ON pc.project_id = p.id
		WHERE pc.cluster_uuid = ?
			AND pc.is_deleted = 0
			AND p.is_deleted = 0
	`

	// 定义记录结构体（字符串类型）
	type ProjectClusterRecord struct {
		CpuLimit     string `db:"cpu_limit"`
		CpuCapacity  string `db:"cpu_capacity"`
		MemLimit     string `db:"mem_limit"`
		MemCapacity  string `db:"mem_capacity"`
		StorageLimit string `db:"storage_limit"`
		GpuLimit     string `db:"gpu_limit"`
		GpuCapacity  string `db:"gpu_capacity"`
		PodsLimit    int64  `db:"pods_limit"`
	}

	var records []*ProjectClusterRecord
	err := m.QueryRowsNoCacheCtx(ctx, &records, query, clusterUuid)
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return nil, fmt.Errorf("查询项目集群记录失败: %v", err)
	}

	// 初始化统计结果
	stats := &ProjectClusterStats{}

	// 如果没有记录，直接返回零值统计
	if len(records) == 0 {
		return stats, nil
	}

	// 遍历所有记录，转换并累加资源
	for _, record := range records {
		// CPU Limit (对应 cpu_allocated_total)
		cpuLimit, err := utils.CPUToCores(record.CpuLimit)
		if err != nil {
			return nil, fmt.Errorf("CPU Limit转换失败 [%s]: %v", record.CpuLimit, err)
		}
		stats.CpuLimitTotal += cpuLimit

		// CPU Capacity (对应 cpu_capacity_total)
		cpuCapacity, err := utils.CPUToCores(record.CpuCapacity)
		if err != nil {
			return nil, fmt.Errorf("CPU Capacity转换失败 [%s]: %v", record.CpuCapacity, err)
		}
		stats.CpuCapacityTotal += cpuCapacity

		// Memory Limit (对应 mem_allocated_total)
		memLimit, err := utils.MemoryToGiB(record.MemLimit)
		if err != nil {
			return nil, fmt.Errorf("Memory Limit转换失败 [%s]: %v", record.MemLimit, err)
		}
		stats.MemLimitTotal += memLimit

		// Memory Capacity (对应 mem_capacity_total)
		memCapacity, err := utils.MemoryToGiB(record.MemCapacity)
		if err != nil {
			return nil, fmt.Errorf("Memory Capacity转换失败 [%s]: %v", record.MemCapacity, err)
		}
		stats.MemCapacityTotal += memCapacity

		// Storage Limit (对应 storage_allocated_total)
		storageLimit, err := utils.MemoryToGiB(record.StorageLimit)
		if err != nil {
			return nil, fmt.Errorf("Storage Limit转换失败 [%s]: %v", record.StorageLimit, err)
		}
		stats.StorageLimitTotal += storageLimit

		// GPU Limit (对应 gpu_allocated_total)
		gpuLimit, err := utils.GPUToCount(record.GpuLimit)
		if err != nil {
			return nil, fmt.Errorf("GPU Limit转换失败 [%s]: %v", record.GpuLimit, err)
		}
		stats.GpuLimitTotal += gpuLimit

		// GPU Capacity (对应 gpu_capacity_total)
		gpuCapacity, err := utils.GPUToCount(record.GpuCapacity)
		if err != nil {
			return nil, fmt.Errorf("GPU Capacity转换失败 [%s]: %v", record.GpuCapacity, err)
		}
		stats.GpuCapacityTotal += gpuCapacity

		// Pods Limit (对应 pods_allocated_total，整数类型，直接累加)
		stats.PodsLimitTotal += record.PodsLimit
	}

	return stats, nil
}

// SyncClusterResourceByResourceId 同步项目集群统计数据到集群资源表（通过clusterResourceId）
// 统计集群下所有项目的资源使用情况，更新到集群资源表
func (m *customOnecClusterModel) SyncClusterResourceByResourceId(ctx context.Context, clusterResourceId uint64) error {
	// 1. 获取项目集群统计数据
	stats, err := m.GetProjectClusterStatsByClusterResource(ctx, clusterResourceId)
	if err != nil {
		return fmt.Errorf("获取项目集群统计失败: %v", err)
	}

	// 2. 查找集群资源记录，验证是否存在并获取cluster_uuid
	clusterResourceQuery := `
		SELECT id, cluster_uuid
		FROM onec_cluster_resource
		WHERE id = ? AND is_deleted = 0
		LIMIT 1
	`
	type ClusterResourceInfo struct {
		Id          uint64 `db:"id"`
		ClusterUuid string `db:"cluster_uuid"`
	}
	var clusterResource ClusterResourceInfo
	err = m.QueryRowNoCacheCtx(ctx, &clusterResource, clusterResourceQuery, clusterResourceId)
	if err != nil {
		return fmt.Errorf("查询集群资源记录失败: %v", err)
	}

	updateQuery := `
		UPDATE onec_cluster_resource SET
			cpu_allocated_total = ?,
			cpu_capacity_total = ?,
			mem_allocated_total = ?,
			mem_capacity_total = ?,
			storage_allocated_total = ?,
			gpu_allocated_total = ?,
			gpu_capacity_total = ?,
			pods_allocated_total = ?
		WHERE id = ? AND is_deleted = 0
	`

	// 4. 生成缓存键（使用集群资源表的缓存前缀）
	cacheKeyClusterUuid := fmt.Sprintf("cache:ikubeops:onecClusterResource:clusterUuid:%v",
		clusterResource.ClusterUuid)
	cacheKeyId := fmt.Sprintf("cache:ikubeops:onecClusterResource:id:%v",
		clusterResourceId)

	_, err = m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		return conn.ExecCtx(ctx, updateQuery,
			stats.CpuLimitTotal,     // cpu_allocated_total = 所有项目 cpu_limit 总和
			stats.CpuCapacityTotal,  // cpu_capacity_total = 所有项目 cpu_capacity 总和
			stats.MemLimitTotal,     // mem_allocated_total = 所有项目 mem_limit 总和
			stats.MemCapacityTotal,  // mem_capacity_total = 所有项目 mem_capacity 总和
			stats.StorageLimitTotal, // storage_allocated_total = 所有项目 storage_limit 总和
			stats.GpuLimitTotal,     // gpu_allocated_total = 所有项目 gpu_limit 总和
			stats.GpuCapacityTotal,  // gpu_capacity_total = 所有项目 gpu_capacity 总和
			stats.PodsLimitTotal,    // pods_allocated_total = 所有项目 pods_limit 总和
			clusterResourceId,
		)
	}, cacheKeyClusterUuid, cacheKeyId)

	if err != nil {
		return fmt.Errorf("更新集群资源统计失败: %v", err)
	}

	return nil
}

// SyncClusterResourceByUuid 同步项目集群统计数据到集群资源表（通过clusterUuid）
// 根据集群UUID统计并更新集群资源表
func (m *customOnecClusterModel) SyncClusterResourceByUuid(ctx context.Context, clusterUuid string) error {
	// 1. 根据 cluster_uuid 查询集群资源记录
	clusterResourceQuery := `
		SELECT id
		FROM onec_cluster_resource
		WHERE cluster_uuid = ? AND is_deleted = 0
		LIMIT 1
	`
	var clusterResourceId uint64
	err := m.QueryRowNoCacheCtx(ctx, &clusterResourceId, clusterResourceQuery, clusterUuid)
	if err != nil {
		return fmt.Errorf("查询集群资源记录失败: %v", err)
	}

	// 2. 调用基于ID的同步方法
	return m.SyncClusterResourceByResourceId(ctx, clusterResourceId)
}

// SyncClusterResourceByClusterId 同步项目集群统计数据到集群资源表（通过clusterId）
// 根据集群ID统计并更新集群资源表
func (m *customOnecClusterModel) SyncClusterResourceByClusterId(ctx context.Context, clusterId uint64) error {
	// 1. 根据集群ID查询集群UUID
	cluster, err := m.FindOne(ctx, clusterId)
	if err != nil {
		return fmt.Errorf("查询集群失败: %v", err)
	}

	// 2. 调用基于UUID的同步方法
	return m.SyncClusterResourceByUuid(ctx, cluster.Uuid)
}

// SyncAllClusters 同步所有集群的资源统计
// 遍历所有集群，逐个同步项目集群统计数据到集群资源表
func (m *customOnecClusterModel) SyncAllClusters(ctx context.Context) error {
	// 1. 获取所有集群
	clusters, err := m.GetAllClusters(ctx)
	if err != nil {
		return fmt.Errorf("获取所有集群失败: %v", err)
	}

	// 2. 如果没有集群，直接返回
	if len(clusters) == 0 {
		return nil
	}

	// 3. 遍历每个集群，执行同步
	var syncErrors []string
	for _, cluster := range clusters {
		err := m.SyncClusterResourceByUuid(ctx, cluster.Uuid)
		if err != nil {
			// 记录错误但继续处理其他集群
			syncErrors = append(syncErrors, fmt.Sprintf("同步集群[%s]失败: %v", cluster.Name, err))
			continue
		}
	}

	// 4. 如果有同步错误，返回汇总错误
	if len(syncErrors) > 0 {
		return fmt.Errorf("部分集群同步失败: %v", syncErrors)
	}

	return nil
}
