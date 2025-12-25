package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	// Redis Key 前缀
	AlertBufferKeyPrefix = "portal:alert:buffer"
	// 扫描间隔
	ScanInterval = 5 * time.Second
	// 缓冲数据 TTL（窗口最大值 + 余量）
	BufferTTL = 10 * time.Minute
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
		Critical: 0,                // 立即发送
		Warning:  1 * time.Minute,  // 1分钟
		Info:     2 * time.Minute,  // 2分钟
		Default:  10 * time.Second, // 10秒
	}
}

// AggregatorConfig 聚合器配置
type AggregatorConfig struct {
	// 是否启用聚合
	Enabled bool
	// 按级别配置窗口
	SeverityWindows SeverityWindowConfig
	// 最大缓冲大小（防止内存泄漏）
	MaxBufferSize int
}

// DefaultAggregatorConfig 默认聚合器配置
func DefaultAggregatorConfig() AggregatorConfig {
	return AggregatorConfig{
		Enabled:         true,
		SeverityWindows: DefaultSeverityWindows(),
		MaxBufferSize:   1000,
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
	// 统计信息
	stats struct {
		mu              sync.RWMutex
		totalBuffered   int64
		totalSent       int64
		totalFailed     int64
		immediatelySent int64
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

	agg := &AlertAggregator{
		redis:           rdb,
		manager:         manager,
		config:          config,
		logger:          logx.WithContext(context.Background()),
		stopChan:        make(chan struct{}),
		processInterval: ScanInterval,
	}

	if config.Enabled {
		agg.wg.Add(1)
		go agg.processLoop()
		agg.logger.Infof("[告警聚合] 已启动 | Critical=%v | Warning=%v | Info=%v | Default=%v",
			config.SeverityWindows.Critical,
			config.SeverityWindows.Warning,
			config.SeverityWindows.Info,
			config.SeverityWindows.Default,
		)
	}

	return agg
}

// AddAlert 添加告警
func (a *AlertAggregator) AddAlert(ctx context.Context, alerts []*AlertInstance) error {
	if !a.config.Enabled {
		return a.manager.SendAlertsDirectly(ctx, alerts)
	}

	if len(alerts) == 0 {
		return nil
	}

	// 按 ProjectID + Severity 分组
	groups := a.groupAlerts(alerts)

	var immediateAlerts []*AlertInstance
	var bufferedGroups []struct {
		key    string
		alerts []*AlertInstance
	}

	// 分离立即发送和缓冲的告警
	for key, groupAlerts := range groups {
		severity := a.extractSeverity(key)
		window := a.getWindowBySeverity(severity)

		if window == 0 {
			// 立即发送
			immediateAlerts = append(immediateAlerts, groupAlerts...)
			a.stats.mu.Lock()
			a.stats.immediatelySent += int64(len(groupAlerts))
			a.stats.mu.Unlock()
			a.logger.Infof("[告警聚合] 级别=%s 立即发送: %d条", severity, len(groupAlerts))
		} else {
			// 缓冲
			bufferedGroups = append(bufferedGroups, struct {
				key    string
				alerts []*AlertInstance
			}{key, groupAlerts})
		}
	}

	// 立即发送 critical
	if len(immediateAlerts) > 0 {
		if err := a.manager.SendAlertsDirectly(ctx, immediateAlerts); err != nil {
			a.logger.Errorf("[告警聚合] 立即发送失败: %v", err)
			a.stats.mu.Lock()
			a.stats.totalFailed++
			a.stats.mu.Unlock()
		}
	}

	// 缓冲其他告警
	for _, group := range bufferedGroups {
		if err := a.bufferAlerts(ctx, group.key, group.alerts); err != nil {
			a.logger.Errorf("[告警聚合] 缓冲失败: %v", err)
			// 降级为直接发送
			if err := a.manager.SendAlertsDirectly(ctx, group.alerts); err != nil {
				a.logger.Errorf("[告警聚合] 降级发送失败: %v", err)
			}
		}
	}

	return nil
}

// groupAlerts 分组
func (a *AlertAggregator) groupAlerts(alerts []*AlertInstance) map[string][]*AlertInstance {
	groups := make(map[string][]*AlertInstance)
	for _, alert := range alerts {
		key := fmt.Sprintf("%d:%s", alert.ProjectID, strings.ToLower(alert.Severity))
		groups[key] = append(groups[key], alert)
	}
	return groups
}

// extractSeverity 提取级别
func (a *AlertAggregator) extractSeverity(key string) string {
	parts := strings.Split(key, ":")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
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
	severity := a.extractSeverity(key)
	window := a.getWindowBySeverity(severity)

	if len(alerts) > a.config.MaxBufferSize {
		a.logger.Errorf("[告警聚合] 超过限制: %d", len(alerts))
		alerts = alerts[:a.config.MaxBufferSize]
	}

	// Lua 脚本 - 修改为使用毫秒时间戳
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
			local totalSize = #buffered.alerts + #newAlerts
			if totalSize > maxSize then
				return redis.error_reply("buffer size exceeded")
			end
			for i, alert in ipairs(newAlerts) do
				table.insert(buffered.alerts, alert)
			end
			buffered.lastTime = nowMs
			buffered.version = buffered.version + 1
		else
			local expireMs = nowMs + (expireSeconds * 1000)

			buffered = {
				key = key,
				alerts = newAlerts,
				firstTime = nowMs,
				lastTime = nowMs,
				expireTime = expireMs,
				version = 1,
				projectId = newAlerts[1].projectId or 0,
				severity = newAlerts[1].severity or "unknown"
			}
		end

		redis.call('SET', key, cjson.encode(buffered), 'EX', expireSeconds + 60)
		return 'OK'
	`

	alertsJSON, err := json.Marshal(alerts)
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
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
		return fmt.Errorf("Redis缓冲失败: %w", err)
	}

	a.stats.mu.Lock()
	a.stats.totalBuffered += int64(len(alerts))
	a.stats.mu.Unlock()

	a.logger.Infof("[告警聚合] 缓冲成功: key=%s, count=%d, window=%v", key, len(alerts), window)
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

// processExpiredBuffers 处理过期缓冲
func (a *AlertAggregator) processExpiredBuffers(ctx context.Context) {
	pattern := fmt.Sprintf("%s:*", AlertBufferKeyPrefix)
	keys, err := a.redis.KeysCtx(ctx, pattern)
	if err != nil {
		a.logger.Errorf("[告警聚合] 扫描失败: %v", err)
		return
	}

	if len(keys) == 0 {
		return
	}

	now := time.Now()
	processed := 0

	for _, key := range keys {
		if a.processBuffer(ctx, key, now) {
			processed++
		}
	}

	if processed > 0 {
		a.logger.Infof("[告警聚合] 处理完成: 扫描=%d, 已发送=%d", len(keys), processed)
	}
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

	var buffered bufferedAlert
	if err := json.Unmarshal([]byte(data), &buffered); err != nil {
		a.logger.Errorf("[告警聚合] 解析失败: %v | 数据: %s", err, data)
		return false
	}

	// 使用辅助方法获取 time.Time
	expireTime := buffered.ExpireTime()
	firstTime := buffered.FirstTime()

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

	// 发送
	duration := now.Sub(firstTime)
	a.logger.Infof("[告警聚合] 到期: key=%s, count=%d, duration=%v", buffered.Key, len(buffered.Alerts), duration)

	if err := a.manager.SendAlertsDirectly(ctx, buffered.Alerts); err != nil {
		a.logger.Errorf("[告警聚合] 发送失败: %v", err)
		a.stats.mu.Lock()
		a.stats.totalFailed++
		a.stats.mu.Unlock()
		return false
	}

	a.stats.mu.Lock()
	a.stats.totalSent += int64(len(buffered.Alerts))
	a.stats.mu.Unlock()

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
func (a *AlertAggregator) FlushAll(ctx context.Context) {
	pattern := fmt.Sprintf("%s:*", AlertBufferKeyPrefix)
	keys, err := a.redis.KeysCtx(ctx, pattern)
	if err != nil {
		a.logger.Errorf("[告警聚合] 刷新失败: %v", err)
		return
	}

	flushed := 0
	futureTime := time.Now().Add(24 * time.Hour)
	for _, key := range keys {
		if a.processBuffer(ctx, key, futureTime) {
			flushed++
		}
	}

	a.logger.Infof("[告警聚合] 刷新完成: 总数=%d, 已发送=%d", len(keys), flushed)
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
	}
}
