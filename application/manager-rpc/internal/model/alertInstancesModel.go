package model

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertInstancesModel = (*customAlertInstancesModel)(nil)

// AlertOverviewStats 告警总览统计结构体
type AlertOverviewStats struct {
	TotalCount         int64   `db:"total_count"`
	FiringCount        int64   `db:"firing_count"`
	ResolvedCount      int64   `db:"resolved_count"`
	TodayNewCount      int64   `db:"today_new_count"`
	TodayResolvedCount int64   `db:"today_resolved_count"`
	AvgDuration        float64 `db:"avg_duration"`
	YesterdayCount     int64   `db:"yesterday_count"`
}

// SeverityStats 级别统计结构体
type SeverityStats struct {
	Severity      string `db:"severity"`
	TotalCount    int64  `db:"total_count"`
	FiringCount   int64  `db:"firing_count"`
	ResolvedCount int64  `db:"resolved_count"`
}

// TrendDataPoint 趋势数据点结构体
type TrendDataPoint struct {
	Date          string `db:"date"`
	NewCount      int64  `db:"new_count"`
	ResolvedCount int64  `db:"resolved_count"`
	FiringCount   int64  `db:"firing_count"`
}

// DimensionStats 维度统计结构体
type DimensionStats struct {
	DimensionId   string `db:"dimension_id"`
	DimensionName string `db:"dimension_name"`
	TotalCount    int64  `db:"total_count"`
	FiringCount   int64  `db:"firing_count"`
	ResolvedCount int64  `db:"resolved_count"`
	CriticalCount int64  `db:"critical_count"`
	WarningCount  int64  `db:"warning_count"`
	InfoCount     int64  `db:"info_count"`
}

// RankingStats 排行统计结构体
type RankingStats struct {
	ItemId        string `db:"item_id"`
	ItemName      string `db:"item_name"`
	AlertCount    int64  `db:"alert_count"`
	FiringCount   int64  `db:"firing_count"`
	CriticalCount int64  `db:"critical_count"`
	WarningCount  int64  `db:"warning_count"`
	ExtraInfo     string `db:"extra_info"`
}

// RealtimeStats 实时状态统计结构体
type RealtimeStats struct {
	FiringCount         int64 `db:"firing_count"`
	CriticalFiringCount int64 `db:"critical_firing_count"`
	WarningFiringCount  int64 `db:"warning_firing_count"`
	InfoFiringCount     int64 `db:"info_firing_count"`
	Last5MinNewCount    int64 `db:"last_5min_new_count"`
	Last5MinResolved    int64 `db:"last_5min_resolved"`
}

type (
	AlertInstancesModel interface {
		alertInstancesModel
		// GetOverviewStats 获取告警总览统计
		GetOverviewStats(ctx context.Context, clusterUuid string, projectId, workspaceId uint64) (*AlertOverviewStats, error)
		// GetSeverityStats 获取级别统计
		GetSeverityStats(ctx context.Context, clusterUuid string, projectId, workspaceId uint64) ([]*SeverityStats, error)
		// GetTrendStats 获取趋势统计
		GetTrendStats(ctx context.Context, clusterUuid string, projectId, workspaceId uint64, days int) ([]*TrendDataPoint, error)
		// GetDimensionStats 获取维度统计
		GetDimensionStats(ctx context.Context, dimension string, clusterUuid string, projectId, workspaceId uint64, page, pageSize uint64) ([]*DimensionStats, int64, error)
		// GetTopRanking 获取排行榜
		GetTopRanking(ctx context.Context, rankingType string, clusterUuid string, projectId, workspaceId uint64, topN int, severity, status string) ([]*RankingStats, int64, error)
		// GetRealtimeStats 获取实时状态统计
		GetRealtimeStats(ctx context.Context, clusterUuid string, projectId, workspaceId uint64) (*RealtimeStats, error)
	}

	customAlertInstancesModel struct {
		*defaultAlertInstancesModel
	}
)

// NewAlertInstancesModel 创建告警实例模型
func NewAlertInstancesModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AlertInstancesModel {
	return &customAlertInstancesModel{
		defaultAlertInstancesModel: newAlertInstancesModel(conn, c, opts...),
	}
}

// buildWhereClause 构建通用的WHERE条件
func (m *customAlertInstancesModel) buildWhereClause(clusterUuid string, projectId, workspaceId uint64) (string, []any) {
	where := "`is_deleted` = 0"
	args := []any{}

	if clusterUuid != "" {
		where += " AND `cluster_uuid` = ?"
		args = append(args, clusterUuid)
	}
	if projectId > 0 {
		where += " AND `project_id` = ?"
		args = append(args, projectId)
	}
	if workspaceId > 0 {
		where += " AND `workspace_id` = ?"
		args = append(args, workspaceId)
	}

	return where, args
}

// GetOverviewStats 获取告警总览统计
func (m *customAlertInstancesModel) GetOverviewStats(ctx context.Context, clusterUuid string, projectId, workspaceId uint64) (*AlertOverviewStats, error) {
	whereClause, args := m.buildWhereClause(clusterUuid, projectId, workspaceId)

	// 获取今天和昨天的时间范围
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterdayStart := todayStart.AddDate(0, 0, -1)

	// 主查询获取基本统计
	query := fmt.Sprintf(`
		SELECT 
			COUNT(*) as total_count,
			COALESCE(SUM(CASE WHEN status = 'firing' THEN 1 ELSE 0 END), 0) as firing_count,
			COALESCE(SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END), 0) as resolved_count,
			COALESCE(SUM(CASE WHEN created_at >= ? THEN 1 ELSE 0 END), 0) as today_new_count,
			COALESCE(SUM(CASE WHEN status = 'resolved' AND resolved_at >= ? THEN 1 ELSE 0 END), 0) as today_resolved_count,
			COALESCE(AVG(CASE WHEN duration > 0 THEN duration ELSE NULL END), 0) as avg_duration
		FROM %s
		WHERE %s
	`, m.table, whereClause)

	// 构建查询参数
	queryArgs := []any{todayStart, todayStart}
	queryArgs = append(queryArgs, args...)

	// 使用临时结构体接收主查询结果
	var mainStats struct {
		TotalCount         int64   `db:"total_count"`
		FiringCount        int64   `db:"firing_count"`
		ResolvedCount      int64   `db:"resolved_count"`
		TodayNewCount      int64   `db:"today_new_count"`
		TodayResolvedCount int64   `db:"today_resolved_count"`
		AvgDuration        float64 `db:"avg_duration"`
	}

	err := m.QueryRowNoCacheCtx(ctx, &mainStats, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("查询告警总览统计失败: %v", err)
	}

	// 单独查询昨日数量
	yesterdayQuery := fmt.Sprintf(`
		SELECT COALESCE(COUNT(*), 0) as cnt
		FROM %s
		WHERE %s AND created_at >= ? AND created_at < ?
	`, m.table, whereClause)

	yesterdayArgs := append(args, yesterdayStart, todayStart)

	var yesterdayCount int64
	err = m.QueryRowNoCacheCtx(ctx, &yesterdayCount, yesterdayQuery, yesterdayArgs...)
	if err != nil {
		return nil, fmt.Errorf("查询昨日告警数量失败: %v", err)
	}

	return &AlertOverviewStats{
		TotalCount:         mainStats.TotalCount,
		FiringCount:        mainStats.FiringCount,
		ResolvedCount:      mainStats.ResolvedCount,
		TodayNewCount:      mainStats.TodayNewCount,
		TodayResolvedCount: mainStats.TodayResolvedCount,
		AvgDuration:        mainStats.AvgDuration,
		YesterdayCount:     yesterdayCount,
	}, nil
}

// GetSeverityStats 获取级别统计
func (m *customAlertInstancesModel) GetSeverityStats(ctx context.Context, clusterUuid string, projectId, workspaceId uint64) ([]*SeverityStats, error) {
	whereClause, args := m.buildWhereClause(clusterUuid, projectId, workspaceId)

	// 添加排序字段到 SELECT 中，然后按该字段排序
	query := fmt.Sprintf(`
		SELECT 
			COALESCE(NULLIF(severity, ''), 'none') as severity,
			COUNT(*) as total_count,
			COALESCE(SUM(CASE WHEN status = 'firing' THEN 1 ELSE 0 END), 0) as firing_count,
			COALESCE(SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END), 0) as resolved_count
		FROM %s
		WHERE %s
		GROUP BY COALESCE(NULLIF(severity, ''), 'none')
	`, m.table, whereClause)

	var stats []*SeverityStats
	err := m.QueryRowsNoCacheCtx(ctx, &stats, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询级别统计失败: %v", err)
	}

	// 在 Go 代码中排序
	severityOrder := map[string]int{
		"critical": 1,
		"warning":  2,
		"info":     3,
		"none":     4,
	}

	sort.Slice(stats, func(i, j int) bool {
		orderI, okI := severityOrder[stats[i].Severity]
		orderJ, okJ := severityOrder[stats[j].Severity]
		if !okI {
			orderI = 5
		}
		if !okJ {
			orderJ = 5
		}
		return orderI < orderJ
	})

	return stats, nil
}

// GetTrendStats 获取趋势统计
func (m *customAlertInstancesModel) GetTrendStats(ctx context.Context, clusterUuid string, projectId, workspaceId uint64, days int) ([]*TrendDataPoint, error) {
	whereClause, args := m.buildWhereClause(clusterUuid, projectId, workspaceId)

	// 计算开始日期
	startDate := time.Now().AddDate(0, 0, -days+1)
	startDateStr := startDate.Format("2006-01-02")

	// 查询每日新增和恢复数量
	query := fmt.Sprintf(`
		SELECT 
			DATE_FORMAT(starts_at, '%%Y-%%m-%%d') as date,
			COUNT(*) as new_count,
			COALESCE(SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END), 0) as resolved_count,
			COALESCE(SUM(CASE WHEN status = 'firing' THEN 1 ELSE 0 END), 0) as firing_count
		FROM %s
		WHERE %s AND DATE(starts_at) >= ?
		GROUP BY DATE_FORMAT(starts_at, '%%Y-%%m-%%d')
		ORDER BY date ASC
	`, m.table, whereClause)

	queryArgs := append(args, startDateStr)

	var stats []*TrendDataPoint
	err := m.QueryRowsNoCacheCtx(ctx, &stats, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("查询趋势统计失败: %v", err)
	}

	return stats, nil
}

// GetDimensionStats 获取维度统计
func (m *customAlertInstancesModel) GetDimensionStats(ctx context.Context, dimension string, clusterUuid string, projectId, workspaceId uint64, page, pageSize uint64) ([]*DimensionStats, int64, error) {
	whereClause, args := m.buildWhereClause(clusterUuid, projectId, workspaceId)

	// 根据维度确定分组字段
	var groupField, nameField string
	switch dimension {
	case "cluster":
		groupField = "cluster_uuid"
		nameField = "cluster_name"
	case "project":
		groupField = "project_id"
		nameField = "project_name"
	case "workspace":
		groupField = "workspace_id"
		nameField = "workspace_name"
	default:
		return nil, 0, fmt.Errorf("不支持的维度类型: %s", dimension)
	}

	// 查询总数
	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT %s) FROM %s WHERE %s
	`, groupField, m.table, whereClause)

	var total int64
	err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询维度统计总数失败: %v", err)
	}

	if total == 0 {
		return []*DimensionStats{}, 0, nil
	}

	// 查询数据
	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf(`
		SELECT 
			CAST(%s AS CHAR) as dimension_id,
			MAX(%s) as dimension_name,
			COUNT(*) as total_count,
			COALESCE(SUM(CASE WHEN status = 'firing' THEN 1 ELSE 0 END), 0) as firing_count,
			COALESCE(SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END), 0) as resolved_count,
			COALESCE(SUM(CASE WHEN severity = 'critical' THEN 1 ELSE 0 END), 0) as critical_count,
			COALESCE(SUM(CASE WHEN severity = 'warning' THEN 1 ELSE 0 END), 0) as warning_count,
			COALESCE(SUM(CASE WHEN severity = 'info' THEN 1 ELSE 0 END), 0) as info_count
		FROM %s
		WHERE %s
		GROUP BY %s
		ORDER BY total_count DESC
		LIMIT ?, ?
	`, groupField, nameField, m.table, whereClause, groupField)

	queryArgs := append(args, offset, pageSize)

	var stats []*DimensionStats
	err = m.QueryRowsNoCacheCtx(ctx, &stats, dataQuery, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询维度统计数据失败: %v", err)
	}

	return stats, total, nil
}

// GetTopRanking 获取排行榜
func (m *customAlertInstancesModel) GetTopRanking(ctx context.Context, rankingType string, clusterUuid string, projectId, workspaceId uint64, topN int, severity, status string) ([]*RankingStats, int64, error) {
	whereClause, args := m.buildWhereClause(clusterUuid, projectId, workspaceId)

	// 添加可选的级别和状态过滤
	if severity != "" {
		whereClause += " AND `severity` = ?"
		args = append(args, severity)
	}
	if status != "" {
		whereClause += " AND `status` = ?"
		args = append(args, status)
	}

	// 根据排行类型确定分组字段
	var groupField, nameField, extraField string
	switch rankingType {
	case "cluster":
		groupField = "cluster_uuid"
		nameField = "cluster_name"
		extraField = "''"
	case "project":
		groupField = "project_id"
		nameField = "project_name"
		extraField = "MAX(cluster_name)"
	case "workspace":
		groupField = "workspace_id"
		nameField = "workspace_name"
		extraField = "CONCAT(MAX(cluster_name), '/', MAX(project_name))"
	case "rule":
		groupField = "alert_name"
		nameField = "alert_name"
		extraField = "''"
	case "instance":
		groupField = "instance"
		nameField = "instance"
		extraField = "MAX(alert_name)"
	default:
		return nil, 0, fmt.Errorf("不支持的排行类型: %s", rankingType)
	}

	// 查询总告警数
	totalQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, m.table, whereClause)
	var totalAlerts int64
	err := m.QueryRowNoCacheCtx(ctx, &totalAlerts, totalQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询告警总数失败: %v", err)
	}

	if totalAlerts == 0 {
		return []*RankingStats{}, 0, nil
	}

	// 查询排行数据
	rankQuery := fmt.Sprintf(`
		SELECT 
			CAST(%s AS CHAR) as item_id,
			MAX(%s) as item_name,
			COUNT(*) as alert_count,
			COALESCE(SUM(CASE WHEN status = 'firing' THEN 1 ELSE 0 END), 0) as firing_count,
			COALESCE(SUM(CASE WHEN severity = 'critical' THEN 1 ELSE 0 END), 0) as critical_count,
			COALESCE(SUM(CASE WHEN severity = 'warning' THEN 1 ELSE 0 END), 0) as warning_count,
			%s as extra_info
		FROM %s
		WHERE %s
		GROUP BY %s
		ORDER BY alert_count DESC
		LIMIT ?
	`, groupField, nameField, extraField, m.table, whereClause, groupField)

	queryArgs := append(args, topN)

	var stats []*RankingStats
	err = m.QueryRowsNoCacheCtx(ctx, &stats, rankQuery, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询排行数据失败: %v", err)
	}

	return stats, totalAlerts, nil
}

// GetRealtimeStats 获取实时状态统计
func (m *customAlertInstancesModel) GetRealtimeStats(ctx context.Context, clusterUuid string, projectId, workspaceId uint64) (*RealtimeStats, error) {
	whereClause, args := m.buildWhereClause(clusterUuid, projectId, workspaceId)

	// 计算5分钟前的时间
	fiveMinAgo := time.Now().Add(-5 * time.Minute)

	query := fmt.Sprintf(`
		SELECT 
			COALESCE(SUM(CASE WHEN status = 'firing' THEN 1 ELSE 0 END), 0) as firing_count,
			COALESCE(SUM(CASE WHEN status = 'firing' AND severity = 'critical' THEN 1 ELSE 0 END), 0) as critical_firing_count,
			COALESCE(SUM(CASE WHEN status = 'firing' AND severity = 'warning' THEN 1 ELSE 0 END), 0) as warning_firing_count,
			COALESCE(SUM(CASE WHEN status = 'firing' AND severity = 'info' THEN 1 ELSE 0 END), 0) as info_firing_count,
			COALESCE(SUM(CASE WHEN created_at >= ? THEN 1 ELSE 0 END), 0) as last_5min_new_count,
			COALESCE(SUM(CASE WHEN status = 'resolved' AND resolved_at >= ? THEN 1 ELSE 0 END), 0) as last_5min_resolved
		FROM %s
		WHERE %s
	`, m.table, whereClause)

	queryArgs := []any{fiveMinAgo, fiveMinAgo}
	queryArgs = append(queryArgs, args...)

	var stats RealtimeStats
	err := m.QueryRowNoCacheCtx(ctx, &stats, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("查询实时状态统计失败: %v", err)
	}

	return &stats, nil
}
