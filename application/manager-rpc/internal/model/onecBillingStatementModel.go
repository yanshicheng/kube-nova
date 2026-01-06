package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OnecBillingStatementModel = (*customOnecBillingStatementModel)(nil)

type (
	// 月度统计结果
	MonthlyStats struct {
		TotalCost      float64
		ResourceCost   float64
		ManagementCost float64
		StatementCount int64
		ProjectCount   int64
	}

	// 费用构成项
	CostCompositionItem struct {
		CostName   string
		CostAmount float64
	}

	// 月度趋势项
	MonthlyTrendItem struct {
		Month          string
		TotalCost      float64
		ResourceCost   float64
		ManagementCost float64
	}

	// 项目费用统计
	ProjectCostStats struct {
		ProjectId   uint64
		ProjectName string
		ProjectUuid string
		TotalCost   float64
	}

	// 集群费用统计
	ClusterCostStats struct {
		ClusterUuid  string
		ClusterName  string
		ProjectCount int64
		TotalCost    float64
	}

	// 账单汇总
	StatementSummary struct {
		TotalCount    int64
		TotalAmount   float64
		CpuCost       float64
		MemoryCost    float64
		StorageCost   float64
		GpuCost       float64
		PodCost       float64
		ManagementFee float64
	}

	OnecBillingStatementModel interface {
		onecBillingStatementModel
		// 原有方法（按月查询）- 保留向后兼容
		GetMonthlyStats(ctx context.Context, month string, clusterUuid string, projectId uint64) (*MonthlyStats, error)
		GetCostComposition(ctx context.Context, month string, clusterUuid string, projectId uint64) ([]*CostCompositionItem, error)
		GetMonthlyTrend(ctx context.Context, months int, clusterUuid string, projectId uint64) ([]*MonthlyTrendItem, error)
		GetProjectTop(ctx context.Context, month string, clusterUuid string, topN int) ([]*ProjectCostStats, error)
		GetClusterTop(ctx context.Context, month string, topN int) ([]*ClusterCostStats, error)

		// 新增方法（按时间区间查询）
		GetStatsByTimeRange(ctx context.Context, startTime, endTime int64, clusterUuid string, projectId uint64) (*MonthlyStats, error)
		GetCostCompositionByTimeRange(ctx context.Context, startTime, endTime int64, clusterUuid string, projectId uint64) ([]*CostCompositionItem, error)
		GetProjectTopByTimeRange(ctx context.Context, startTime, endTime int64, clusterUuid string, topN int) ([]*ProjectCostStats, error)
		GetClusterTopByTimeRange(ctx context.Context, startTime, endTime int64, topN int) ([]*ClusterCostStats, error)

		// 账单搜索
		SearchWithSummary(ctx context.Context, startTime, endTime int64, clusterUuid string, projectId uint64, statementType string, page, pageSize uint64, orderField string, isAsc bool) (*StatementSummary, []*OnecBillingStatement, int64, error)
		DeleteByCondition(ctx context.Context, beforeTime int64, clusterUuid string, projectId uint64) (int64, error)
		SoftDelete(ctx context.Context, id uint64) error
	}

	customOnecBillingStatementModel struct {
		*defaultOnecBillingStatementModel
	}
)

func NewOnecBillingStatementModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OnecBillingStatementModel {
	return &customOnecBillingStatementModel{
		defaultOnecBillingStatementModel: newOnecBillingStatementModel(conn, c, opts...),
	}
}

// ===================== 按月查询方法（原有，保留向后兼容）=====================

// GetMonthlyStats 获取月度统计数据（按 billing_end_time 月份匹配）
func (m *customOnecBillingStatementModel) GetMonthlyStats(ctx context.Context, month string, clusterUuid string, projectId uint64) (*MonthlyStats, error) {
	query := `select 
		COALESCE(SUM(total_amount), 0) as total_cost,
		COALESCE(SUM(resource_cost_total), 0) as resource_cost,
		COALESCE(SUM(management_fee), 0) as management_cost,
		COUNT(*) as statement_count,
		COUNT(DISTINCT project_id) as project_count
		from ` + m.table + ` where is_deleted = 0 and DATE_FORMAT(billing_end_time, '%Y-%m') = ?`

	args := []interface{}{month}

	if clusterUuid != "" {
		query += " and cluster_uuid = ?"
		args = append(args, clusterUuid)
	}
	if projectId > 0 {
		query += " and project_id = ?"
		args = append(args, projectId)
	}

	var stats MonthlyStats
	err := m.CachedConn.QueryRowNoCacheCtx(ctx, &stats, query, args...)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetCostComposition 获取费用构成（按 billing_end_time 月份匹配）
func (m *customOnecBillingStatementModel) GetCostComposition(ctx context.Context, month string, clusterUuid string, projectId uint64) ([]*CostCompositionItem, error) {
	query := `select 
		COALESCE(SUM(cpu_cost), 0) as cpu_cost,
		COALESCE(SUM(memory_cost), 0) as memory_cost,
		COALESCE(SUM(storage_cost), 0) as storage_cost,
		COALESCE(SUM(gpu_cost), 0) as gpu_cost,
		COALESCE(SUM(pod_cost), 0) as pod_cost,
		COALESCE(SUM(management_fee), 0) as management_fee
		from ` + m.table + ` where is_deleted = 0 and DATE_FORMAT(billing_end_time, '%Y-%m') = ?`

	args := []interface{}{month}

	if clusterUuid != "" {
		query += " and cluster_uuid = ?"
		args = append(args, clusterUuid)
	}
	if projectId > 0 {
		query += " and project_id = ?"
		args = append(args, projectId)
	}

	var result struct {
		CpuCost       float64 `db:"cpu_cost"`
		MemoryCost    float64 `db:"memory_cost"`
		StorageCost   float64 `db:"storage_cost"`
		GpuCost       float64 `db:"gpu_cost"`
		PodCost       float64 `db:"pod_cost"`
		ManagementFee float64 `db:"management_fee"`
	}

	err := m.CachedConn.QueryRowNoCacheCtx(ctx, &result, query, args...)
	if err != nil {
		return nil, err
	}

	items := []*CostCompositionItem{
		{CostName: "CPU费用", CostAmount: result.CpuCost},
		{CostName: "内存费用", CostAmount: result.MemoryCost},
		{CostName: "存储费用", CostAmount: result.StorageCost},
		{CostName: "GPU费用", CostAmount: result.GpuCost},
		{CostName: "Pod费用", CostAmount: result.PodCost},
		{CostName: "管理费", CostAmount: result.ManagementFee},
	}
	return items, nil
}

// GetMonthlyTrend 获取月度趋势（按 billing_end_time 分组）
func (m *customOnecBillingStatementModel) GetMonthlyTrend(ctx context.Context, months int, clusterUuid string, projectId uint64) ([]*MonthlyTrendItem, error) {
	query := `select 
		DATE_FORMAT(billing_end_time, '%Y-%m') as month,
		COALESCE(SUM(total_amount), 0) as total_cost,
		COALESCE(SUM(resource_cost_total), 0) as resource_cost,
		COALESCE(SUM(management_fee), 0) as management_cost
		from ` + m.table + ` where is_deleted = 0 
		and billing_end_time >= DATE_SUB(CURDATE(), INTERVAL ? MONTH)`

	args := []interface{}{months}

	if clusterUuid != "" {
		query += " and cluster_uuid = ?"
		args = append(args, clusterUuid)
	}
	if projectId > 0 {
		query += " and project_id = ?"
		args = append(args, projectId)
	}

	query += " group by DATE_FORMAT(billing_end_time, '%Y-%m') order by month asc"

	var items []*MonthlyTrendItem
	err := m.CachedConn.QueryRowsNoCacheCtx(ctx, &items, query, args...)
	if err != nil {
		return nil, err
	}
	return items, nil
}

// GetProjectTop 获取项目费用排行（按 billing_end_time 月份匹配）
func (m *customOnecBillingStatementModel) GetProjectTop(ctx context.Context, month string, clusterUuid string, topN int) ([]*ProjectCostStats, error) {
	query := `select 
		project_id,
		project_name,
		project_uuid,
		COALESCE(SUM(total_amount), 0) as total_cost
		from ` + m.table + ` where is_deleted = 0 and DATE_FORMAT(billing_end_time, '%Y-%m') = ?`

	args := []interface{}{month}

	if clusterUuid != "" {
		query += " and cluster_uuid = ?"
		args = append(args, clusterUuid)
	}

	query += " group by project_id, project_name, project_uuid order by total_cost desc limit ?"
	args = append(args, topN)

	var items []*ProjectCostStats
	err := m.CachedConn.QueryRowsNoCacheCtx(ctx, &items, query, args...)
	if err != nil {
		return nil, err
	}
	return items, nil
}

// GetClusterTop 获取集群费用排行（按 billing_end_time 月份匹配）
func (m *customOnecBillingStatementModel) GetClusterTop(ctx context.Context, month string, topN int) ([]*ClusterCostStats, error) {
	query := `select 
		cluster_uuid,
		cluster_name,
		COUNT(DISTINCT project_id) as project_count,
		COALESCE(SUM(total_amount), 0) as total_cost
		from ` + m.table + ` where is_deleted = 0 and DATE_FORMAT(billing_end_time, '%Y-%m') = ?
		group by cluster_uuid, cluster_name 
		order by total_cost desc 
		limit ?`

	var items []*ClusterCostStats
	err := m.CachedConn.QueryRowsNoCacheCtx(ctx, &items, query, month, topN)
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ===================== 按时间区间查询方法（新增）=====================

// GetStatsByTimeRange 按时间区间获取统计数据
func (m *customOnecBillingStatementModel) GetStatsByTimeRange(ctx context.Context, startTime, endTime int64, clusterUuid string, projectId uint64) (*MonthlyStats, error) {
	query := `select 
		COALESCE(SUM(total_amount), 0) as total_cost,
		COALESCE(SUM(resource_cost_total), 0) as resource_cost,
		COALESCE(SUM(management_fee), 0) as management_cost,
		COUNT(*) as statement_count,
		COUNT(DISTINCT project_id) as project_count
		from ` + m.table + ` where is_deleted = 0`

	args := make([]interface{}, 0)

	// 时间区间条件：账单结束时间在区间内
	if startTime > 0 {
		query += " and billing_end_time >= ?"
		args = append(args, time.Unix(startTime, 0))
	}
	if endTime > 0 {
		query += " and billing_end_time <= ?"
		args = append(args, time.Unix(endTime, 0))
	}

	if clusterUuid != "" {
		query += " and cluster_uuid = ?"
		args = append(args, clusterUuid)
	}
	if projectId > 0 {
		query += " and project_id = ?"
		args = append(args, projectId)
	}

	var stats MonthlyStats
	err := m.CachedConn.QueryRowNoCacheCtx(ctx, &stats, query, args...)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetCostCompositionByTimeRange 按时间区间获取费用构成
func (m *customOnecBillingStatementModel) GetCostCompositionByTimeRange(ctx context.Context, startTime, endTime int64, clusterUuid string, projectId uint64) ([]*CostCompositionItem, error) {
	query := `select 
		COALESCE(SUM(cpu_cost), 0) as cpu_cost,
		COALESCE(SUM(memory_cost), 0) as memory_cost,
		COALESCE(SUM(storage_cost), 0) as storage_cost,
		COALESCE(SUM(gpu_cost), 0) as gpu_cost,
		COALESCE(SUM(pod_cost), 0) as pod_cost,
		COALESCE(SUM(management_fee), 0) as management_fee
		from ` + m.table + ` where is_deleted = 0`

	args := make([]interface{}, 0)

	if startTime > 0 {
		query += " and billing_end_time >= ?"
		args = append(args, time.Unix(startTime, 0))
	}
	if endTime > 0 {
		query += " and billing_end_time <= ?"
		args = append(args, time.Unix(endTime, 0))
	}

	if clusterUuid != "" {
		query += " and cluster_uuid = ?"
		args = append(args, clusterUuid)
	}
	if projectId > 0 {
		query += " and project_id = ?"
		args = append(args, projectId)
	}

	var result struct {
		CpuCost       float64 `db:"cpu_cost"`
		MemoryCost    float64 `db:"memory_cost"`
		StorageCost   float64 `db:"storage_cost"`
		GpuCost       float64 `db:"gpu_cost"`
		PodCost       float64 `db:"pod_cost"`
		ManagementFee float64 `db:"management_fee"`
	}

	err := m.CachedConn.QueryRowNoCacheCtx(ctx, &result, query, args...)
	if err != nil {
		return nil, err
	}

	items := []*CostCompositionItem{
		{CostName: "CPU费用", CostAmount: result.CpuCost},
		{CostName: "内存费用", CostAmount: result.MemoryCost},
		{CostName: "存储费用", CostAmount: result.StorageCost},
		{CostName: "GPU费用", CostAmount: result.GpuCost},
		{CostName: "Pod费用", CostAmount: result.PodCost},
		{CostName: "管理费", CostAmount: result.ManagementFee},
	}
	return items, nil
}

// GetProjectTopByTimeRange 按时间区间获取项目费用排行
func (m *customOnecBillingStatementModel) GetProjectTopByTimeRange(ctx context.Context, startTime, endTime int64, clusterUuid string, topN int) ([]*ProjectCostStats, error) {
	query := `select 
		project_id,
		project_name,
		project_uuid,
		COALESCE(SUM(total_amount), 0) as total_cost
		from ` + m.table + ` where is_deleted = 0`

	args := make([]interface{}, 0)

	if startTime > 0 {
		query += " and billing_end_time >= ?"
		args = append(args, time.Unix(startTime, 0))
	}
	if endTime > 0 {
		query += " and billing_end_time <= ?"
		args = append(args, time.Unix(endTime, 0))
	}

	if clusterUuid != "" {
		query += " and cluster_uuid = ?"
		args = append(args, clusterUuid)
	}

	query += " group by project_id, project_name, project_uuid order by total_cost desc limit ?"
	args = append(args, topN)

	var items []*ProjectCostStats
	err := m.CachedConn.QueryRowsNoCacheCtx(ctx, &items, query, args...)
	if err != nil {
		return nil, err
	}
	return items, nil
}

// GetClusterTopByTimeRange 按时间区间获取集群费用排行
func (m *customOnecBillingStatementModel) GetClusterTopByTimeRange(ctx context.Context, startTime, endTime int64, topN int) ([]*ClusterCostStats, error) {
	query := `select 
		cluster_uuid,
		cluster_name,
		COUNT(DISTINCT project_id) as project_count,
		COALESCE(SUM(total_amount), 0) as total_cost
		from ` + m.table + ` where is_deleted = 0`

	args := make([]interface{}, 0)

	if startTime > 0 {
		query += " and billing_end_time >= ?"
		args = append(args, time.Unix(startTime, 0))
	}
	if endTime > 0 {
		query += " and billing_end_time <= ?"
		args = append(args, time.Unix(endTime, 0))
	}

	query += " group by cluster_uuid, cluster_name order by total_cost desc limit ?"
	args = append(args, topN)

	var items []*ClusterCostStats
	err := m.CachedConn.QueryRowsNoCacheCtx(ctx, &items, query, args...)
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ===================== 账单搜索相关方法 =====================

// 排序字段映射：驼峰 -> 下划线
var statementOrderFieldMap = map[string]string{
	"id":                "id",
	"statementNo":       "statement_no",
	"statementType":     "statement_type",
	"billingStartTime":  "billing_start_time",
	"billingEndTime":    "billing_end_time",
	"billingHours":      "billing_hours",
	"clusterUuid":       "cluster_uuid",
	"clusterName":       "cluster_name",
	"projectId":         "project_id",
	"projectName":       "project_name",
	"cpuCost":           "cpu_cost",
	"memoryCost":        "memory_cost",
	"storageCost":       "storage_cost",
	"gpuCost":           "gpu_cost",
	"podCost":           "pod_cost",
	"managementFee":     "management_fee",
	"resourceCostTotal": "resource_cost_total",
	"totalAmount":       "total_amount",
	"createdAt":         "created_at",
	"updatedAt":         "updated_at",
}

// SearchWithSummary 搜索账单并返回汇总
func (m *customOnecBillingStatementModel) SearchWithSummary(ctx context.Context, startTime, endTime int64, clusterUuid string, projectId uint64, statementType string, page, pageSize uint64, orderField string, isAsc bool) (*StatementSummary, []*OnecBillingStatement, int64, error) {
	whereClause := "is_deleted = 0"
	args := make([]interface{}, 0)

	if startTime > 0 {
		whereClause += " and billing_end_time >= ?"
		args = append(args, time.Unix(startTime, 0))
	}
	if endTime > 0 {
		whereClause += " and billing_end_time <= ?"
		args = append(args, time.Unix(endTime, 0))
	}
	if clusterUuid != "" {
		whereClause += " and cluster_uuid = ?"
		args = append(args, clusterUuid)
	}
	if projectId > 0 {
		whereClause += " and project_id = ?"
		args = append(args, projectId)
	}
	if statementType != "" {
		whereClause += " and statement_type = ?"
		args = append(args, statementType)
	}

	// 查询汇总数据
	summaryQuery := fmt.Sprintf(`select 
		COUNT(*) as total_count,
		COALESCE(SUM(total_amount), 0) as total_amount,
		COALESCE(SUM(cpu_cost), 0) as cpu_cost,
		COALESCE(SUM(memory_cost), 0) as memory_cost,
		COALESCE(SUM(storage_cost), 0) as storage_cost,
		COALESCE(SUM(gpu_cost), 0) as gpu_cost,
		COALESCE(SUM(pod_cost), 0) as pod_cost,
		COALESCE(SUM(management_fee), 0) as management_fee
		from %s where %s`, m.table, whereClause)

	var summary StatementSummary
	err := m.CachedConn.QueryRowNoCacheCtx(ctx, &summary, summaryQuery, args...)
	if err != nil {
		return nil, nil, 0, err
	}

	// 查询总数
	total := summary.TotalCount

	// 处理排序字段：驼峰转下划线
	orderDir := "desc"
	if isAsc {
		orderDir = "asc"
	}
	dbOrderField := "billing_end_time" // 默认按账单结束时间排序
	if orderField != "" {
		if mapped, ok := statementOrderFieldMap[orderField]; ok {
			dbOrderField = mapped
		} else {
			// 如果不在映射表中，直接使用（可能已经是下划线格式）
			dbOrderField = orderField
		}
	}

	offset := (page - 1) * pageSize
	listQuery := fmt.Sprintf("select %s from %s where %s order by `%s` %s limit ?, ?",
		onecBillingStatementRows, m.table, whereClause, dbOrderField, orderDir)

	listArgs := append(args, offset, pageSize)
	var list []*OnecBillingStatement
	err = m.CachedConn.QueryRowsNoCacheCtx(ctx, &list, listQuery, listArgs...)
	if err != nil {
		return nil, nil, 0, err
	}

	return &summary, list, total, nil
}

// DeleteByCondition 按条件批量删除
func (m *customOnecBillingStatementModel) DeleteByCondition(ctx context.Context, beforeTime int64, clusterUuid string, projectId uint64) (int64, error) {
	whereClause := "is_deleted = 0 and billing_end_time < ?"
	args := []interface{}{time.Unix(beforeTime, 0)}

	if clusterUuid != "" {
		whereClause += " and cluster_uuid = ?"
		args = append(args, clusterUuid)
	}
	if projectId > 0 {
		whereClause += " and project_id = ?"
		args = append(args, projectId)
	}

	query := fmt.Sprintf("update %s set is_deleted = 1 where %s", m.table, whereClause)
	result, err := m.CachedConn.ExecNoCacheCtx(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}

// SoftDelete 软删除单条记录
func (m *customOnecBillingStatementModel) SoftDelete(ctx context.Context, id uint64) error {
	data, err := m.FindOne(ctx, id)
	if err != nil {
		return err
	}

	kubeNovaOnecBillingStatementIdKey := fmt.Sprintf("%s%v", cacheKubeNovaOnecBillingStatementIdPrefix, id)
	kubeNovaOnecBillingStatementStatementNoKey := fmt.Sprintf("%s%v", cacheKubeNovaOnecBillingStatementStatementNoPrefix, data.StatementNo)

	_, err = m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		query := fmt.Sprintf("update %s set is_deleted = 1 where id = ?", m.table)
		return conn.ExecCtx(ctx, query, id)
	}, kubeNovaOnecBillingStatementIdKey, kubeNovaOnecBillingStatementStatementNoKey)

	return err
}
