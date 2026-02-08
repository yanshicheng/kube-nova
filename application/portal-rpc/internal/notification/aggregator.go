package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	// AlertBufferKeyPrefix Redis Key 前缀
	AlertBufferKeyPrefix = "portal:alert:buffer"
	// ScanInterval 扫描间隔
	ScanInterval = 5 * time.Second
	// BufferTTL 缓冲数据 TTL（窗口最大值 + 余量）
	BufferTTL = 10 * time.Minute
	// ScanCount SCAN 命令每次扫描的数量建议值
	ScanCount = 100

	// LeaderElectionKey 分布式 Leader 选举的 Redis Key
	LeaderElectionKey = "portal:alert:aggregator:leader"
	// LeaderTTL Leader 租约时间，超过此时间未续约则自动释放
	LeaderTTL = 15 * time.Second
	// LeaderRenewInterval Leader 续约间隔，应该小于 LeaderTTL
	LeaderRenewInterval = 5 * time.Second
)

// SeverityWindowConfig 告警级别窗口配置
type SeverityWindowConfig struct {
	Critical time.Duration // critical 级别窗口（0=立即发送）
	Warning  time.Duration // warning 级别窗口
	Info     time.Duration // info 级别窗口
	Default  time.Duration // 默认窗口（其他级别）
}

// DefaultSeverityWindows 默认级别窗口配置
func DefaultSeverityWindows() SeverityWindowConfig {
	return SeverityWindowConfig{
		Critical: 0,               // 立即发送
		Warning:  1 * time.Minute, // 1分钟
		Info:     1 * time.Minute, // 1分钟（与 Warning 保持一致）
		Default:  1 * time.Minute, // 1分钟（与 Warning 保持一致）
	}
}

// AggregatorConfig 聚合器配置
type AggregatorConfig struct {
	// Enabled 是否启用聚合
	Enabled bool
	// SeverityWindows 按级别配置窗口
	SeverityWindows SeverityWindowConfig
	// MaxBufferSize 最大缓冲大小（防止内存泄漏）
	MaxBufferSize int
	// GlobalBufferWindow 全局缓冲窗口，用于跨调用聚合（默认30秒）
	GlobalBufferWindow time.Duration
}

// DefaultAggregatorConfig 默认聚合器配置
func DefaultAggregatorConfig() AggregatorConfig {
	return AggregatorConfig{
		Enabled:            true,
		SeverityWindows:    DefaultSeverityWindows(),
		MaxBufferSize:      1000,
		GlobalBufferWindow: 30 * time.Second, // 30秒全局缓冲窗口
	}
}

// InternalManager 内部管理器接口（用于避免循环调用）
type InternalManager interface {
	// SendAlertsDirectly 直接发送告警（不经过聚合器）
	SendAlertsDirectly(ctx context.Context, alerts []*AlertInstance) error
}

// AlertAggregator 告警聚合器
type AlertAggregator struct {
	redis           *redis.Redis
	manager         InternalManager
	config          AggregatorConfig
	logger          logx.Logger
	stopChan        chan struct{}
	wg              sync.WaitGroup
	processInterval time.Duration
	// 分布式 Leader 选举相关
	leaderID      string        // 当前实例的 Leader ID
	isLeader      bool          // 当前是否是 Leader
	leaderStopCh  chan struct{} // 停止 Leader 选举循环
	leaderRenewCh chan struct{} // Leader 续约信号
	// 健康监控
	lastLeadershipCheck time.Time // 上次检查 Leader 身份的时间
	leadershipFailures  int       // 连续 Leader 操作失败次数
	maxLeaderFailures   int       // 最大允许的连续失败次数
	// stats 统计信息
	stats struct {
		mu              sync.RWMutex
		totalBuffered   int64
		totalSent       int64
		totalFailed     int64
		immediatelySent int64
		leaderElections int64 // Leader 选举次数
		leaderRenewals  int64 // Leader 续约次数
	}
}

// bufferedAlert 缓冲的告警
type bufferedAlert struct {
	Key          string           `json:"key"`
	ProjectID    uint64           `json:"projectId"`
	Severity     string           `json:"severity"`
	Alerts       []*AlertInstance `json:"alerts"`
	FirstTimeMs  int64            `json:"firstTime"`  // Unix 毫秒时间戳
	LastTimeMs   int64            `json:"lastTime"`   // Unix 毫秒时间戳
	ExpireTimeMs int64            `json:"expireTime"` // Unix 毫秒时间戳
	Version      int64            `json:"version"`
}

// FirstTime 获取首次时间
func (b *bufferedAlert) FirstTime() time.Time {
	return time.UnixMilli(b.FirstTimeMs)
}

// LastTime 获取最后时间
func (b *bufferedAlert) LastTime() time.Time {
	return time.UnixMilli(b.LastTimeMs)
}

// ExpireTime 获取过期时间
func (b *bufferedAlert) ExpireTime() time.Time {
	return time.UnixMilli(b.ExpireTimeMs)
}

// NewAlertAggregatorWithManager 创建告警聚合器
func NewAlertAggregatorWithManager(rdb *redis.Redis, manager InternalManager, config AggregatorConfig) *AlertAggregator {
	if rdb == nil {
		logx.Error("[告警聚合] Redis 连接为空")
		return nil
	}
	if manager == nil {
		logx.Error("[告警聚合] Manager 为空")
		return nil
	}

	if config.MaxBufferSize == 0 {
		config.MaxBufferSize = 1000
	}

	// 生成唯一的 Leader ID（使用 hostname 和 timestamp 组合）
	leaderID := fmt.Sprintf("agg-%d", time.Now().UnixNano())

	agg := &AlertAggregator{
		redis:               rdb,
		manager:             manager,
		config:              config,
		logger:              logx.WithContext(context.Background()),
		stopChan:            make(chan struct{}),
		processInterval:     ScanInterval,
		leaderID:            leaderID,
		isLeader:            false,
		leaderStopCh:        make(chan struct{}),
		leaderRenewCh:       make(chan struct{}, 1),
		lastLeadershipCheck: time.Now(),
		maxLeaderFailures:   3, // 最大允许3次连续失败
	}

	if config.Enabled {
		// 启动 Leader 选举循环
		agg.wg.Add(1)
		go agg.leaderElectionLoop()
		agg.logger.Infof("[告警聚合] Leader 选举已启动 | LeaderID=%s", leaderID)
		agg.logger.Infof("[告警聚合] 窗口配置 | Critical=%v | Warning=%v | Info=%v | Default=%v",
			config.SeverityWindows.Critical,
			config.SeverityWindows.Warning,
			config.SeverityWindows.Info,
			config.SeverityWindows.Default,
		)
	}

	return agg
}

// AddAlert 添加告警 - 统一告警处理入口（支持跨调用聚合）
func (a *AlertAggregator) AddAlert(ctx context.Context, alerts []*AlertInstance) error {
	if len(alerts) == 0 {
		return nil
	}

	// 记录接收到的告警
	a.logger.Infof("[告警聚合] 接收到 %d 条告警", len(alerts))

	// 如果聚合功能未启用，直接发送但仍通过统一通道
	if !a.config.Enabled {
		a.logger.Info("[告警聚合] 聚合功能未启用，直接发送")
		return a.manager.SendAlertsDirectly(ctx, alerts)
	}

	// 实现跨调用聚合：将所有告警先缓冲到全局缓冲区
	return a.addToGlobalBuffer(ctx, alerts)
}

// groupAlerts 分组
// 按项目ID分组（不区分级别），支持跨级别聚合
func (a *AlertAggregator) groupAlerts(alerts []*AlertInstance) map[string][]*AlertInstance {
	groups := make(map[string][]*AlertInstance)
	for _, alert := range alerts {
		// 只按 ProjectID 分组，不再区分 Severity
		key := fmt.Sprintf("%d", alert.ProjectID)
		groups[key] = append(groups[key], alert)
	}
	return groups
}

// extractSeverity 提取级别（已废弃，保留用于兼容）
// 由于现在按 ProjectID 分组，不再从 key 中提取级别
func (a *AlertAggregator) extractSeverity(key string) string {
	// 保留此方法用于向后兼容，但不再使用
	parts := strings.Split(key, ":")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// getMaxWindowForAlerts 获取告警组中的最大窗口时间
// 用于确定该组告警应该使用的聚合窗口
func (a *AlertAggregator) getMaxWindowForAlerts(alerts []*AlertInstance) time.Duration {
	maxWindow := time.Duration(0)
	for _, alert := range alerts {
		window := a.getWindowBySeverity(alert.Severity)
		if window > maxWindow {
			maxWindow = window
		}
	}
	return maxWindow
}

// getWindowBySeverity 获取窗口
func (a *AlertAggregator) getWindowBySeverity(severity string) time.Duration {
	switch strings.ToLower(severity) {
	case "critical":
		return a.config.SeverityWindows.Critical
	case "warning":
		return a.config.SeverityWindows.Warning
	case "info":
		return a.config.SeverityWindows.Info
	default:
		return a.config.SeverityWindows.Default
	}
}

// bufferAlerts 缓冲到 Redis
func (a *AlertAggregator) bufferAlerts(ctx context.Context, key string, alerts []*AlertInstance) error {
	if len(alerts) == 0 {
		return nil
	}

	redisKey := fmt.Sprintf("%s:%s", AlertBufferKeyPrefix, key)

	// 使用最大窗口时间作为过期时间
	window := a.getMaxWindowForAlerts(alerts)

	if len(alerts) > a.config.MaxBufferSize {
		a.logger.Errorf("[告警聚合] 超过限制: %d", len(alerts))
		alerts = alerts[:a.config.MaxBufferSize]
	}

	// Lua 脚本 - 支持按级别分组存储
	script := `
		local key = KEYS[1]
		local alertsJson = ARGV[1]
		local expireSeconds = tonumber(ARGV[2])
		local maxSize = tonumber(ARGV[3])
		local nowMs = tonumber(ARGV[4])

		local existingData = redis.call('GET', key)
		local buffered
		local newAlerts = cjson.decode(alertsJson)

		if existingData then
			buffered = cjson.decode(existingData)

			-- 按级别分组合并告警
			for i, alert in ipairs(newAlerts) do
				local severity = string.upper(alert.severity or "INFO")
				if not buffered.alertsBySeverity then
					buffered.alertsBySeverity = {}
				end
				if not buffered.alertsBySeverity[severity] then
					buffered.alertsBySeverity[severity] = {cjson.null}
				end

				-- 检查是否是已恢复的告警
				if alert.status == "resolved" then
					if not buffered.resolvedAlerts then
						buffered.resolvedAlerts = {cjson.null}
					end
					-- 检查 fingerprint 是否已存在
					local found = false
					for j, existing in ipairs(buffered.resolvedAlerts) do
						if existing.fingerprint == alert.fingerprint then
							-- 更新已存在的告警
							buffered.resolvedAlerts[j] = alert
							found = true
							break
						end
					end
					if not found then
						table.insert(buffered.resolvedAlerts, alert)
					end
				else
					-- 检查 fingerprint 是否已存在
					local found = false
					for j, existing in ipairs(buffered.alertsBySeverity[severity]) do
						if existing.fingerprint == alert.fingerprint then
							-- 更新已存在的告警（重复次数可能增加了）
							buffered.alertsBySeverity[severity][j] = alert
							found = true
							break
						end
					end
					if not found then
						table.insert(buffered.alertsBySeverity[severity], alert)
					end
				end
			end

			buffered.lastTime = nowMs
			buffered.version = buffered.version + 1
		else
			local expireMs = nowMs + (expireSeconds * 1000)

			-- 初始化按级别分组的结构
			local alertsBySeverity = {}
			local resolvedAlerts = {cjson.null}

			for i, alert in ipairs(newAlerts) do
				local severity = string.upper(alert.severity or "INFO")
				if not alertsBySeverity[severity] then
					alertsBySeverity[severity] = {cjson.null}
				end

				if alert.status == "resolved" then
					table.insert(resolvedAlerts, alert)
				else
					table.insert(alertsBySeverity[severity], alert)
				end
			end

			buffered = {
				key = key,
				alertsBySeverity = alertsBySeverity,
				resolvedAlerts = resolvedAlerts,
				firstTime = nowMs,
				lastTime = nowMs,
				expireTime = expireMs,
				version = 1,
				projectId = newAlerts[1].projectId or 0,
				projectName = newAlerts[1].projectName or ""
			}
		end

		redis.call('SET', key, cjson.encode(buffered), 'EX', expireSeconds + 60)
		return 'OK'
	`

	alertsJSON, err := json.Marshal(alerts)
	if err != nil {
		return errorx.Msg(fmt.Sprintf("序列化失败: %v", err))
	}

	nowMs := time.Now().UnixMilli()
	expireSeconds := int(window.Seconds())

	_, err = a.redis.EvalCtx(ctx, script,
		[]string{redisKey},
		string(alertsJSON),
		expireSeconds,
		a.config.MaxBufferSize,
		nowMs,
	)

	if err != nil {
		a.stats.mu.Lock()
		a.stats.totalFailed++
		a.stats.mu.Unlock()
		return errorx.Msg(fmt.Sprintf("Redis缓冲失败: %v", err))
	}

	a.stats.mu.Lock()
	a.stats.totalBuffered += int64(len(alerts))
	a.stats.mu.Unlock()

	// 记录 Prometheus 指标
	for _, alert := range alerts {
		GlobalMetricsCollector.RecordAlertBuffered(alert.ProjectName, alert.Severity, 1)
	}

	a.logger.Infof("[告警聚合] 缓冲成功: key=%s, count=%d, window=%v", key, len(alerts), window)
	return nil
}

// cleanupCriticalAlertsOnly 只清理 Critical 级别的告警，保留其他级别
func (a *AlertAggregator) cleanupCriticalAlertsOnly(ctx context.Context, redisKey string) error {
	// Lua 脚本：只删除 CRITICAL 级别的告警，保留其他级别
	script := `
		local key = KEYS[1]
		local data = redis.call('GET', key)
		if not data then
			return 'KEY_NOT_FOUND'
		end

		local buffered = cjson.decode(data)
		if not buffered.alertsBySeverity then
			return 'NO_SEVERITY_DATA'
		end

		-- 删除 CRITICAL 级别的告警
		buffered.alertsBySeverity['CRITICAL'] = nil

		-- 检查是否还有其他级别的告警
		local hasOtherAlerts = false
		for severity, alerts in pairs(buffered.alertsBySeverity) do
			if alerts and #alerts > 0 then
				hasOtherAlerts = true
				break
			end
		end

		-- 检查是否有已恢复的告警
		local hasResolvedAlerts = false
		if buffered.resolvedAlerts and #buffered.resolvedAlerts > 0 then
			hasResolvedAlerts = true
		end

		if hasOtherAlerts or hasResolvedAlerts then
			-- 还有其他告警，更新数据
			buffered.version = buffered.version + 1
			redis.call('SET', key, cjson.encode(buffered), 'KEEPTTL')
			return 'UPDATED'
		else
			-- 没有其他告警了，删除整个键
			redis.call('DEL', key)
			return 'DELETED'
		end
	`

	result, err := a.redis.EvalCtx(ctx, script, []string{redisKey})
	if err != nil {
		return errorx.Msg(fmt.Sprintf("清理Critical告警失败: %v", err))
	}

	resultStr, ok := result.(string)
	if !ok {
		return errorx.Msg("清理Critical告警返回值类型错误")
	}

	switch resultStr {
	case "KEY_NOT_FOUND":
		a.logger.Infof("[告警聚合] 清理Critical告警: 键不存在 %s", redisKey)
	case "NO_SEVERITY_DATA":
		a.logger.Infof("[告警聚合] 清理Critical告警: 无级别数据 %s", redisKey)
	case "UPDATED":
		a.logger.Infof("[告警聚合] 清理Critical告警: 已更新，保留其他级别 %s", redisKey)
	case "DELETED":
		a.logger.Infof("[告警聚合] 清理Critical告警: 已删除整个键 %s", redisKey)
	default:
		a.logger.Infof("[告警聚合] 清理Critical告警: 未知结果 %s -> %s", redisKey, resultStr)
	}

	return nil
}

// processLoop 后台处理
func (a *AlertAggregator) processLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(a.processInterval)
	defer ticker.Stop()

	a.logger.Info("[告警聚合] 处理循环已启动")

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			a.processExpiredBuffers(ctx)
			cancel()
		case <-a.stopChan:
			a.logger.Info("[告警聚合] 处理循环已停止")
			return
		}
	}
}

// processKeysWithCallback 使用回调函数流式处理 Redis 键
// 这种方式避免了将所有键累积到内存中，适合处理大量键的场景
// pattern: 匹配模式
// callback: 处理每个键的回调函数，返回 false 时停止处理
func (a *AlertAggregator) processKeysWithCallback(ctx context.Context, pattern string, callback func(key string) bool) error {
	var cursor uint64 = 0

	for {
		// 使用 SCAN 命令分批获取键，避免阻塞 Redis
		keys, nextCursor, err := a.redis.ScanCtx(ctx, cursor, pattern, ScanCount)
		if err != nil {
			return errorx.Msg(fmt.Sprintf("SCAN 失败: %v", err))
		}

		// 立即处理每批 key，不累积到内存
		for _, key := range keys {
			if !callback(key) {
				return nil // 回调返回 false 时停止处理
			}
		}

		cursor = nextCursor

		// 游标为 0 表示扫描完成
		if cursor == 0 {
			break
		}

		// 检查 context 是否已取消，避免长时间阻塞
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// 继续扫描
		}
	}

	return nil
}

// processExpiredBuffers 处理过期缓冲（包括全局缓冲区）
// 使用流式处理避免内存累积问题
func (a *AlertAggregator) processExpiredBuffers(ctx context.Context) {
	// 处理原有的缓冲区
	pattern := fmt.Sprintf("%s:*", AlertBufferKeyPrefix)
	now := time.Now()
	processed := 0
	scanned := 0

	// 使用流式处理，边扫描边处理，避免内存累积
	err := a.processKeysWithCallback(ctx, pattern, func(key string) bool {
		scanned++
		if a.processBuffer(ctx, key, now) {
			processed++
		}
		return true // 继续处理
	})

	if err != nil {
		a.logger.Errorf("[告警聚合] 扫描失败: %v", err)
		return
	}

	if processed > 0 {
		a.logger.Infof("[告警聚合] 处理完成: 扫描=%d, 已发送=%d", scanned, processed)
	}

	// 处理全局缓冲区
	a.processExpiredGlobalBuffers(ctx)
}

// processBuffer 处理单个缓冲
func (a *AlertAggregator) processBuffer(ctx context.Context, redisKey string, now time.Time) bool {
	script := `
		local key = KEYS[1]
		local data = redis.call('GET', key)
		if data then
			redis.call('DEL', key)
			return data
		end
		return nil
	`

	result, err := a.redis.EvalCtx(ctx, script, []string{redisKey})
	if err != nil || result == nil {
		return false
	}

	data, ok := result.(string)
	if !ok || data == "" {
		return false
	}

	// 解析新的按级别分组的缓冲结构
	var buffered struct {
		Key              string                      `json:"key"`
		ProjectID        uint64                      `json:"projectId"`
		ProjectName      string                      `json:"projectName"`
		AlertsBySeverity map[string][]*AlertInstance `json:"alertsBySeverity"`
		ResolvedAlerts   []*AlertInstance            `json:"resolvedAlerts"`
		FirstTimeMs      int64                       `json:"firstTime"`
		LastTimeMs       int64                       `json:"lastTime"`
		ExpireTimeMs     int64                       `json:"expireTime"`
		Version          int64                       `json:"version"`
	}

	if err := json.Unmarshal([]byte(data), &buffered); err != nil {
		a.logger.Errorf("[告警聚合] 解析失败: %v | 数据: %s", err, data)
		return false
	}

	// 计算时间
	expireTime := time.UnixMilli(buffered.ExpireTimeMs)
	firstTime := time.UnixMilli(buffered.FirstTimeMs)

	// 检查是否到期
	if now.Before(expireTime) {
		ttl := int(expireTime.Sub(now).Seconds()) + 60
		if ttl > 0 {
			if err := a.redis.SetexCtx(ctx, redisKey, data, ttl); err != nil {
				a.logger.Errorf("[告警聚合] 放回失败: %v", err)
			}
		}
		return false
	}

	// 合并所有告警
	var allAlerts []*AlertInstance
	totalCount := 0

	// 过滤并合并按级别分组的告警
	for _, alerts := range buffered.AlertsBySeverity {
		for _, alert := range alerts {
			if alert != nil { // 过滤掉 null 值
				allAlerts = append(allAlerts, alert)
				totalCount++
			}
		}
	}

	// 过滤并合并已恢复的告警
	for _, alert := range buffered.ResolvedAlerts {
		if alert != nil { // 过滤掉 null 值
			allAlerts = append(allAlerts, alert)
			totalCount++
		}
	}

	// 发送
	duration := now.Sub(firstTime)
	a.logger.Infof("[告警聚合] 到期: key=%s, count=%d, duration=%v", buffered.Key, totalCount, duration)

	if err := a.manager.SendAlertsDirectly(ctx, allAlerts); err != nil {
		a.logger.Errorf("[告警聚合] 发送失败: %v", err)
		a.stats.mu.Lock()
		a.stats.totalFailed++
		a.stats.mu.Unlock()
		return false
	}

	a.stats.mu.Lock()
	a.stats.totalSent += int64(totalCount)
	a.stats.mu.Unlock()

	// 记录 Prometheus 指标
	GlobalMetricsCollector.RecordAggregatedAlertsExpired(buffered.ProjectName, totalCount)

	a.logger.Infof("[告警聚合] 发送成功: key=%s", buffered.Key)
	return true
}

// Stop 停止
func (a *AlertAggregator) Stop() {
	if !a.config.Enabled {
		return
	}

	close(a.stopChan)
	a.wg.Wait()

	a.stats.mu.RLock()
	a.logger.Infof("[告警聚合] 统计: 缓冲=%d, 已发送=%d, 失败=%d, 立即发送=%d",
		a.stats.totalBuffered, a.stats.totalSent, a.stats.totalFailed, a.stats.immediatelySent)
	a.stats.mu.RUnlock()

	a.logger.Info("[告警聚合] 已停止")
}

// FlushAll 刷新所有缓冲
// 使用流式处理避免内存累积问题
func (a *AlertAggregator) FlushAll(ctx context.Context) {
	pattern := fmt.Sprintf("%s:*", AlertBufferKeyPrefix)
	futureTime := time.Now().Add(24 * time.Hour)
	flushed := 0
	total := 0

	// 使用流式处理，边扫描边处理
	err := a.processKeysWithCallback(ctx, pattern, func(key string) bool {
		total++
		if a.processBuffer(ctx, key, futureTime) {
			flushed++
		}
		return true // 继续处理
	})

	if err != nil {
		a.logger.Errorf("[告警聚合] 刷新失败: %v", err)
		return
	}

	a.logger.Infof("[告警聚合] 刷新完成: 总数=%d, 已发送=%d", total, flushed)
}

// GetStats 获取统计
func (a *AlertAggregator) GetStats() map[string]int64 {
	a.stats.mu.RLock()
	defer a.stats.mu.RUnlock()
	return map[string]int64{
		"totalBuffered":   a.stats.totalBuffered,
		"totalSent":       a.stats.totalSent,
		"totalFailed":     a.stats.totalFailed,
		"immediatelySent": a.stats.immediatelySent,
		"leaderElections": a.stats.leaderElections,
		"leaderRenewals":  a.stats.leaderRenewals,
		"leaderFailures":  int64(a.leadershipFailures),
		"isLeader": func() int64 {
			if a.isLeader {
				return 1
			} else {
				return 0
			}
		}(),
	}
}

// GetHealthInfo 获取健康信息
func (a *AlertAggregator) GetHealthInfo() map[string]interface{} {
	a.stats.mu.RLock()
	defer a.stats.mu.RUnlock()

	return map[string]interface{}{
		"enabled":             a.config.Enabled,
		"isLeader":            a.isLeader,
		"leaderID":            a.leaderID,
		"lastLeadershipCheck": a.lastLeadershipCheck.Format(time.RFC3339),
		"leadershipFailures":  a.leadershipFailures,
		"maxLeaderFailures":   a.maxLeaderFailures,
		"leaderElections":     a.stats.leaderElections,
		"leaderRenewals":      a.stats.leaderRenewals,
		"processInterval":     a.processInterval.String(),
		"severityWindows": map[string]string{
			"critical": a.config.SeverityWindows.Critical.String(),
			"warning":  a.config.SeverityWindows.Warning.String(),
			"info":     a.config.SeverityWindows.Info.String(),
			"default":  a.config.SeverityWindows.Default.String(),
		},
		"stats": map[string]int64{
			"totalBuffered":   a.stats.totalBuffered,
			"totalSent":       a.stats.totalSent,
			"totalFailed":     a.stats.totalFailed,
			"immediatelySent": a.stats.immediatelySent,
		},
	}
}

// ==================== 分布式 Leader 选举 ====================

// leaderElectionLoop Leader 选举循环
// 使用 Redis 实现分布式 Leader 选举，确保只有一个副本运行聚合器
func (a *AlertAggregator) leaderElectionLoop() {
	defer a.wg.Done()

	a.logger.Info("[Leader选举] 选举循环已启动")

	// 启动续约循环
	renewTicker := time.NewTicker(LeaderRenewInterval)
	defer renewTicker.Stop()

	// 启动处理循环（只有 Leader 才会处理）
	var processStopCh chan struct{}

	for {
		select {
		case <-renewTicker.C:
			// 定期尝试续约
			a.tryRenewLeadership()

		case <-a.leaderRenewCh:
			// 被动续约信号（当成功竞选 Leader 后触发）
			a.tryRenewLeadership()

		case <-a.stopChan:
			// 停止选举循环
			a.logger.Info("[Leader选举] 选举循环已停止")
			if a.isLeader {
				a.resignLeadership()
			}
			return

		case <-a.leaderStopCh:
			// Leader 被剥夺
			if processStopCh != nil {
				close(processStopCh)
				processStopCh = nil
			}
			a.isLeader = false
			a.logger.Errorf("[Leader选举] 已失去 Leader 身份")
		}

		// 检查是否是 Leader，如果是则启动处理循环
		wasLeader := a.isLeader
		a.checkLeadership()

		// 如果刚刚成为 Leader，启动处理循环
		if !wasLeader && a.isLeader {
			a.logger.Info("[Leader选举] 成为 Leader，启动聚合处理循环")
			processStopCh = make(chan struct{})
			go a.processLoopWithStop(processStopCh)
		} else if wasLeader && !a.isLeader {
			// 刚刚失去 Leader 身份
			a.logger.Errorf("[Leader选举] 已失去 Leader 身份")
		}
	}
}

// tryRenewLeadership 尝试续约 Leader 身份
func (a *AlertAggregator) tryRenewLeadership() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // 增加超时时间
	defer cancel()

	a.lastLeadershipCheck = time.Now()

	// 先检查当前 Key 的值
	currentLeader, err := a.redis.GetCtx(ctx, LeaderElectionKey)
	if err != nil {
		// Key 不存在或 Redis 连接失败，尝试竞选
		if err.Error() == "redis: nil" {
			// Key 不存在，尝试竞选
			success, err := a.redis.SetnxExCtx(ctx, LeaderElectionKey, a.leaderID, int(LeaderTTL.Seconds()))
			if err != nil {
				a.logger.Errorf("[Leader选举] 竞选失败: %v", err)
				a.leadershipFailures++
				return
			}
			if success {
				if !a.isLeader {
					a.isLeader = true
					a.leadershipFailures = 0 // 重置失败计数
					a.stats.mu.Lock()
					a.stats.leaderElections++
					a.stats.mu.Unlock()
					// 记录 Prometheus 指标
					GlobalMetricsCollector.RecordLeaderElection(a.leaderID, "redis")
					GlobalMetricsCollector.RecordAggregatorStatus(a.leaderID, a.config.Enabled, true, "redis")
					a.logger.Infof("[Leader选举] 竞选成为 Leader | LeaderID=%s", a.leaderID)
				}
			}
		} else {
			// Redis 连接失败
			a.logger.Errorf("[Leader选举] Redis 连接失败: %v", err)
			a.leadershipFailures++
			if a.isLeader {
				a.logger.Infof("[Leader选举] 由于 Redis 连接失败，暂时保持 Leader 状态")
			}

			// 如果连续失败次数过多，考虑放弃 Leader 身份
			if a.leadershipFailures >= a.maxLeaderFailures {
				a.logger.Errorf("[Leader选举] 连续失败 %d 次，放弃 Leader 身份", a.leadershipFailures)
				a.isLeader = false
				a.leadershipFailures = 0
			}
		}
		return
	}

	if currentLeader == a.leaderID {
		// Key 存在且是自己的值，直接续约（使用 SETEX）
		err := a.redis.SetexCtx(ctx, LeaderElectionKey, a.leaderID, int(LeaderTTL.Seconds()))
		if err != nil {
			a.logger.Errorf("[Leader选举] 续约失败: %v", err)
			a.leadershipFailures++
			// 续约失败，但不立即放弃 Leader 身份，等待下次重试
			if a.leadershipFailures >= a.maxLeaderFailures {
				a.logger.Errorf("[Leader选举] 续约连续失败 %d 次，放弃 Leader 身份", a.leadershipFailures)
				a.isLeader = false
				a.leadershipFailures = 0
			}
			return
		}

		// 续约成功
		a.leadershipFailures = 0 // 重置失败计数
		a.stats.mu.Lock()
		a.stats.leaderRenewals++
		a.stats.mu.Unlock()
		// 记录 Prometheus 指标
		GlobalMetricsCollector.RecordLeaderRenewal(a.leaderID, "redis")

		if !a.isLeader {
			a.isLeader = true
			GlobalMetricsCollector.RecordAggregatorStatus(a.leaderID, a.config.Enabled, true, "redis")
			a.logger.Infof("[Leader选举] 恢复 Leader 身份 | LeaderID=%s", a.leaderID)
		}
		return
	}

	// Key 存在但是别人的值，说明当前不是 Leader
	if a.isLeader {
		a.isLeader = false
		a.leadershipFailures = 0 // 重置失败计数
		a.logger.Infof("[Leader选举] 失去 Leader 身份 | 当前Leader=%s", currentLeader)
	}
}

// checkLeadership 检查 Leader 身份是否仍然有效
func (a *AlertAggregator) checkLeadership() {
	if !a.isLeader {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	currentLeader, err := a.redis.GetCtx(ctx, LeaderElectionKey)
	if err != nil || currentLeader != a.leaderID {
		// 不再是 Leader
		if a.isLeader {
			a.isLeader = false
			a.logger.Errorf("[Leader选举] 失去 Leader 身份 | 当前=%s, 期望=%s", currentLeader, a.leaderID)
		}
	}
}

// resignLeadership 主动放弃 Leader 身份
func (a *AlertAggregator) resignLeadership() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	currentLeader, err := a.redis.GetCtx(ctx, LeaderElectionKey)
	if err == nil && currentLeader == a.leaderID {
		a.redis.DelCtx(ctx, LeaderElectionKey)
		a.logger.Info("[Leader选举] 已主动释放 Leader 身份")
	}
}

// processLoopWithStop 带停止信号的聚合处理循环
// 只有 Leader 副本才会执行此循环
func (a *AlertAggregator) processLoopWithStop(stopCh chan struct{}) {
	defer a.logger.Info("[告警聚合] 处理循环已停止")

	ticker := time.NewTicker(a.processInterval)
	defer ticker.Stop()

	a.logger.Info("[告警聚合] Leader 处理循环已启动")

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			a.processExpiredBuffers(ctx)
			cancel()

		case <-stopCh:
			a.logger.Info("[告警聚合] 收到停止信号，退出处理循环")
			return

		case <-a.stopChan:
			a.logger.Info("[告警聚合] 收到全局停止信号，退出处理循环")
			return
		}
	}
}

// addToGlobalBuffer 将告警添加到全局缓冲区，实现跨调用聚合
func (a *AlertAggregator) addToGlobalBuffer(ctx context.Context, alerts []*AlertInstance) error {
	// 按 ProjectID 分组
	groups := a.groupAlerts(alerts)

	var immediateAlerts []*AlertInstance

	for key, groupAlerts := range groups {
		var criticalAlerts []*AlertInstance
		var nonCriticalAlerts []*AlertInstance

		// 按级别分离告警
		for _, alert := range groupAlerts {
			if strings.ToLower(alert.Severity) == "critical" {
				criticalAlerts = append(criticalAlerts, alert)
			} else {
				nonCriticalAlerts = append(nonCriticalAlerts, alert)
			}
		}

		// Critical 告警立即发送（不聚合）
		if len(criticalAlerts) > 0 {
			immediateAlerts = append(immediateAlerts, criticalAlerts...)
			a.logger.Infof("[跨调用聚合] 项目=%s CRITICAL级别立即发送: %d条", key, len(criticalAlerts))
		}

		// 非 Critical 告警添加到全局缓冲区
		if len(nonCriticalAlerts) > 0 {
			if err := a.addToGlobalBufferByProject(ctx, key, nonCriticalAlerts); err != nil {
				a.logger.Errorf("[跨调用聚合] 项目=%s 缓冲失败: %v", key, err)
				// 降级为直接发送
				immediateAlerts = append(immediateAlerts, nonCriticalAlerts...)
			} else {
				a.logger.Infof("[跨调用聚合] 项目=%s 已缓冲 %d 条非Critical告警", key, len(nonCriticalAlerts))
			}
		}
	}

	// 立即发送 Critical 告警和降级的告警
	if len(immediateAlerts) > 0 {
		return a.manager.SendAlertsDirectly(ctx, immediateAlerts)
	}

	return nil
}

// addToGlobalBufferByProject 将告警添加到指定项目的全局缓冲区
func (a *AlertAggregator) addToGlobalBufferByProject(ctx context.Context, projectKey string, alerts []*AlertInstance) error {
	globalBufferKey := fmt.Sprintf("portal:alert:global_buffer:%s", projectKey)

	// 获取当前缓冲区的告警
	existingData, err := a.redis.GetCtx(ctx, globalBufferKey)
	var existingAlerts []*AlertInstance

	if err == nil && existingData != "" {
		if err := json.Unmarshal([]byte(existingData), &existingAlerts); err != nil {
			a.logger.Errorf("[跨调用聚合] 解析现有缓冲区数据失败: %v", err)
			existingAlerts = nil // 重置为空，避免数据污染
		}
	}

	// 合并新告警到现有告警中（去重）
	mergedAlerts := a.mergeAlerts(existingAlerts, alerts)

	// 检查是否达到发送条件
	shouldSend, reason := a.shouldSendGlobalBuffer(mergedAlerts)

	if shouldSend {
		a.logger.Infof("[跨调用聚合] 项目=%s 触发发送条件: %s, 共%d条告警", projectKey, reason, len(mergedAlerts))

		// 删除缓冲区
		a.redis.DelCtx(ctx, globalBufferKey)

		// 发送告警
		return a.manager.SendAlertsDirectly(ctx, mergedAlerts)
	} else {
		// 更新缓冲区
		mergedData, err := json.Marshal(mergedAlerts)
		if err != nil {
			return fmt.Errorf("序列化合并后的告警失败: %w", err)
		}

		// 设置过期时间为全局缓冲窗口
		expireSeconds := int(a.config.GlobalBufferWindow.Seconds())
		if err := a.redis.SetexCtx(ctx, globalBufferKey, string(mergedData), expireSeconds); err != nil {
			return fmt.Errorf("更新全局缓冲区失败: %w", err)
		}

		a.logger.Infof("[跨调用聚合] 项目=%s 已更新缓冲区: %d条告警, 过期时间=%ds",
			projectKey, len(mergedAlerts), expireSeconds)

		return nil
	}
}

// mergeAlerts 合并告警列表，去重（基于 Fingerprint）
func (a *AlertAggregator) mergeAlerts(existing []*AlertInstance, new []*AlertInstance) []*AlertInstance {
	fingerprintMap := make(map[string]*AlertInstance)

	// 先添加现有告警
	for _, alert := range existing {
		fingerprintMap[alert.Fingerprint] = alert
	}

	// 添加新告警（会覆盖相同 Fingerprint 的旧告警）
	for _, alert := range new {
		fingerprintMap[alert.Fingerprint] = alert
	}

	// 转换回切片
	result := make([]*AlertInstance, 0, len(fingerprintMap))
	for _, alert := range fingerprintMap {
		result = append(result, alert)
	}

	return result
}

// shouldSendGlobalBuffer 判断是否应该发送全局缓冲区的告警
func (a *AlertAggregator) shouldSendGlobalBuffer(alerts []*AlertInstance) (bool, string) {
	if len(alerts) == 0 {
		return false, "无告警"
	}

	// 条件1: 告警数量达到阈值（比如5条）
	if len(alerts) >= 5 {
		return true, fmt.Sprintf("告警数量达到阈值(%d条)", len(alerts))
	}

	// 条件2: 包含多个不同的告警类型
	alertNames := make(map[string]bool)
	for _, alert := range alerts {
		alertNames[alert.AlertName] = true
	}
	if len(alertNames) >= 3 {
		return true, fmt.Sprintf("包含多种告警类型(%d种)", len(alertNames))
	}

	// 条件3: 包含不同级别的告警
	severities := make(map[string]bool)
	for _, alert := range alerts {
		severities[strings.ToLower(alert.Severity)] = true
	}
	if len(severities) >= 2 {
		return true, fmt.Sprintf("包含多种级别(%d种)", len(severities))
	}

	// 其他情况等待时间窗口到期
	return false, "等待更多告警或时间窗口到期"
}

// processExpiredGlobalBuffers 处理过期的全局缓冲区
func (a *AlertAggregator) processExpiredGlobalBuffers(ctx context.Context) {
	pattern := "portal:alert:global_buffer:*"
	processed := 0
	scanned := 0

	err := a.processKeysWithCallback(ctx, pattern, func(key string) bool {
		scanned++
		if a.processGlobalBuffer(ctx, key) {
			processed++
		}
		return true // 继续处理
	})

	if err != nil {
		a.logger.Errorf("[跨调用聚合] 扫描全局缓冲区失败: %v", err)
	} else if processed > 0 {
		a.logger.Infof("[跨调用聚合] 全局缓冲区处理完成: 扫描=%d, 已发送=%d", scanned, processed)
	}
}

// processGlobalBuffer 处理单个全局缓冲区
func (a *AlertAggregator) processGlobalBuffer(ctx context.Context, redisKey string) bool {
	// 获取并删除缓冲区数据
	data, err := a.redis.GetCtx(ctx, redisKey)
	if err != nil || data == "" {
		return false
	}

	// 删除缓冲区
	a.redis.DelCtx(ctx, redisKey)

	// 解析告警数据
	var alerts []*AlertInstance
	if err := json.Unmarshal([]byte(data), &alerts); err != nil {
		a.logger.Errorf("[跨调用聚合] 解析全局缓冲区数据失败: key=%s, error=%v", redisKey, err)
		return false
	}

	if len(alerts) == 0 {
		return false
	}

	// 提取项目信息
	projectKey := strings.TrimPrefix(redisKey, "portal:alert:global_buffer:")
	a.logger.Infof("[跨调用聚合] 全局缓冲区到期，发送告警: 项目=%s, 数量=%d", projectKey, len(alerts))

	// 发送告警
	if err := a.manager.SendAlertsDirectly(ctx, alerts); err != nil {
		a.logger.Errorf("[跨调用聚合] 发送全局缓冲区告警失败: 项目=%s, error=%v", projectKey, err)
		return false
	}

	return true
}
