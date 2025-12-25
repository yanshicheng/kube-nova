package notification

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// RetryContext 重试上下文信息
type RetryContext struct {
	ChannelType string // 渠道类型：dingtalk, email, wechat, feishu, webhook, site_message
	ChannelUUID string // 渠道 UUID
	ChannelName string // 渠道名称
	Operation   string // 操作类型：send_alert, send_notification, test_connection
	TargetInfo  string // 目标信息：如收件人、webhook地址等
}

// baseClient 基础客户端实现
type baseClient struct {
	// 配置信息
	config Config

	// 日志和上下文
	l   logx.Logger
	ctx context.Context

	// 状态
	healthy     int32
	closed      int32
	healthMutex sync.Mutex

	// 健康检查信息
	lastHealthCheckTime  time.Time
	lastHealthCheckError error

	// 统计信息
	statsMu sync.RWMutex
	stats   Statistics

	// 速率限制
	rateLimiter *rateLimiter
}

// rateLimiter 简单的速率限制器
type rateLimiter struct {
	limit  int
	window time.Duration
	tokens []time.Time
	mu     sync.Mutex
}

// newRateLimiter 创建速率限制器
func newRateLimiter(limitPerMinute int) *rateLimiter {
	return &rateLimiter{
		limit:  limitPerMinute,
		window: time.Minute,
		tokens: make([]time.Time, 0),
	}
}

// Allow 检查是否允许通过
func (r *rateLimiter) Allow() bool {
	if r.limit <= 0 {
		return true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	// 清理过期的令牌
	validTokens := make([]time.Time, 0, len(r.tokens))
	for _, t := range r.tokens {
		if t.After(cutoff) {
			validTokens = append(validTokens, t)
		}
	}
	r.tokens = validTokens

	// 检查是否超过限制
	if len(r.tokens) >= r.limit {
		return false
	}

	// 添加新令牌
	r.tokens = append(r.tokens, now)
	return true
}

// newBaseClient 创建基础客户端
func newBaseClient(config Config) *baseClient {
	// 设置默认选项
	if config.Options.Timeout == 0 {
		config.Options.Timeout = 10 * time.Second
	}
	if config.Options.RetryCount == 0 {
		config.Options.RetryCount = 3
	}
	if config.Options.RetryInterval == 0 {
		config.Options.RetryInterval = 2 * time.Second
	}
	if config.Options.RateLimitPerMinute == 0 {
		config.Options.RateLimitPerMinute = 60
	}

	bc := &baseClient{
		config:      config,
		ctx:         context.Background(),
		l:           logx.WithContext(context.Background()),
		rateLimiter: newRateLimiter(config.Options.RateLimitPerMinute),
	}

	// 初始化状态
	atomic.StoreInt32(&bc.healthy, 1)
	atomic.StoreInt32(&bc.closed, 0)

	// 初始化统计信息
	bc.stats = Statistics{
		TotalSent:    0,
		TotalSuccess: 0,
		TotalFailed:  0,
	}

	return bc
}

// GetUUID 获取 UUID
func (bc *baseClient) GetUUID() string {
	return bc.config.UUID
}

// GetName 获取名称
func (bc *baseClient) GetName() string {
	if bc.config.Name != "" {
		return bc.config.Name
	}
	return bc.config.UUID
}

// GetType 获取类型
func (bc *baseClient) GetType() AlertType {
	return bc.config.Type
}

// GetConfig 获取配置
func (bc *baseClient) GetConfig() Config {
	return bc.config
}

// WithContext 设置上下文
func (bc *baseClient) WithContext(ctx context.Context) Client {
	bc.ctx = ctx
	bc.l = logx.WithContext(ctx)
	return bc
}

// IsHealthy 检查是否健康
func (bc *baseClient) IsHealthy() bool {
	if atomic.LoadInt32(&bc.closed) == 1 {
		return false
	}
	return atomic.LoadInt32(&bc.healthy) == 1
}

// GetStatistics 获取统计信息
func (bc *baseClient) GetStatistics() *Statistics {
	bc.statsMu.RLock()
	defer bc.statsMu.RUnlock()

	statsCopy := bc.stats
	return &statsCopy
}

// ResetStatistics 重置统计信息
func (bc *baseClient) ResetStatistics() {
	bc.statsMu.Lock()
	defer bc.statsMu.Unlock()

	bc.stats = Statistics{
		TotalSent:    0,
		TotalSuccess: 0,
		TotalFailed:  0,
	}
	bc.l.Infof("重置告警客户端 %s 统计信息", bc.config.UUID)
}

// Close 关闭客户端
func (bc *baseClient) Close() error {
	if !atomic.CompareAndSwapInt32(&bc.closed, 0, 1) {
		return nil
	}

	bc.l.Infof("关闭告警客户端: %s (%s)", bc.config.UUID, bc.config.Type)
	return nil
}

// recordSuccess 记录成功
func (bc *baseClient) recordSuccess(latency time.Duration) {
	if !bc.config.Options.EnableMetrics {
		return
	}

	bc.statsMu.Lock()
	defer bc.statsMu.Unlock()

	bc.stats.TotalSent++
	bc.stats.TotalSuccess++
	bc.stats.LastSentTime = time.Now()
	bc.stats.LastSuccessTime = time.Now()

	// 更新平均延迟
	if bc.stats.AverageLatency == 0 {
		bc.stats.AverageLatency = latency
	} else {
		bc.stats.AverageLatency = (bc.stats.AverageLatency + latency) / 2
	}
}

// recordFailure 记录失败
func (bc *baseClient) recordFailure() {
	if !bc.config.Options.EnableMetrics {
		return
	}

	bc.statsMu.Lock()
	defer bc.statsMu.Unlock()

	bc.stats.TotalSent++
	bc.stats.TotalFailed++
	bc.stats.LastSentTime = time.Now()
	bc.stats.LastFailedTime = time.Now()
}

// checkRateLimit 检查速率限制
func (bc *baseClient) checkRateLimit() error {
	if !bc.rateLimiter.Allow() {
		return fmt.Errorf("超过速率限制：每分钟最多 %d 条", bc.config.Options.RateLimitPerMinute)
	}
	return nil
}

// retryWithBackoff 重试机制（不带上下文信息，保持向后兼容）
func (bc *baseClient) retryWithBackoff(ctx context.Context, operation func() error) error {
	retryCtx := &RetryContext{
		ChannelType: string(bc.config.Type),
		ChannelUUID: bc.config.UUID,
		ChannelName: bc.config.Name,
		Operation:   "unknown",
	}
	return bc.retryWithBackoffContext(ctx, retryCtx, operation)
}

// retryWithBackoffContext 带上下文的重试机制
func (bc *baseClient) retryWithBackoffContext(ctx context.Context, retryCtx *RetryContext, operation func() error) error {
	var lastErr error

	for i := 0; i <= bc.config.Options.RetryCount; i++ {
		if i > 0 {
			// 详细的重试日志
			bc.l.Errorf("[重试] 第 %d/%d 次重试 | 渠道类型: %s | 渠道UUID: %s | 渠道名称: %s | 操作: %s | 目标: %s | 上次错误: %v",
				i,
				bc.config.Options.RetryCount,
				retryCtx.ChannelType,
				retryCtx.ChannelUUID,
				retryCtx.ChannelName,
				retryCtx.Operation,
				retryCtx.TargetInfo,
				lastErr,
			)

			select {
			case <-ctx.Done():
				bc.l.Errorf("[重试中断] 上下文取消 | 渠道类型: %s | 渠道UUID: %s | 错误: %v",
					retryCtx.ChannelType, retryCtx.ChannelUUID, ctx.Err())
				return ctx.Err()
			case <-time.After(bc.config.Options.RetryInterval):
			}
		}

		if err := operation(); err != nil {
			lastErr = err
			bc.l.Errorf("[操作失败] 渠道类型: %s | 渠道UUID: %s | 渠道名称: %s | 操作: %s | 目标: %s | 错误: %v",
				retryCtx.ChannelType,
				retryCtx.ChannelUUID,
				retryCtx.ChannelName,
				retryCtx.Operation,
				retryCtx.TargetInfo,
				err,
			)
			continue
		}

		// 成功
		if i > 0 {
			bc.l.Infof("[重试成功] 第 %d 次重试成功 | 渠道类型: %s | 渠道UUID: %s | 操作: %s",
				i, retryCtx.ChannelType, retryCtx.ChannelUUID, retryCtx.Operation)
		}
		return nil
	}

	finalErr := fmt.Errorf("[重试失败] 渠道类型: %s | 渠道UUID: %s | 渠道名称: %s | 操作: %s | 重试 %d 次后仍然失败: %w",
		retryCtx.ChannelType,
		retryCtx.ChannelUUID,
		retryCtx.ChannelName,
		retryCtx.Operation,
		bc.config.Options.RetryCount,
		lastErr,
	)
	bc.l.Error(finalErr.Error())
	return finalErr
}

// updateHealthStatus 更新健康状态
func (bc *baseClient) updateHealthStatus(healthy bool, err error) {
	bc.healthMutex.Lock()
	defer bc.healthMutex.Unlock()

	bc.lastHealthCheckTime = time.Now()
	bc.lastHealthCheckError = err

	if healthy {
		atomic.StoreInt32(&bc.healthy, 1)
	} else {
		atomic.StoreInt32(&bc.healthy, 0)
	}
}

// HealthCheck 默认健康检查实现
func (bc *baseClient) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	if atomic.LoadInt32(&bc.closed) == 1 {
		return &HealthStatus{
			Healthy:       false,
			Message:       "客户端已关闭",
			LastCheckTime: time.Now(),
		}, fmt.Errorf("客户端已关闭")
	}

	bc.updateHealthStatus(true, nil)
	return &HealthStatus{
		Healthy:       true,
		Message:       "客户端健康",
		LastCheckTime: time.Now(),
	}, nil
}

// TestConnection 默认测试连接实现
func (bc *baseClient) TestConnection(ctx context.Context, toEmail []string) (*TestResult, error) {
	return nil, fmt.Errorf("TestConnection 方法未实现")
}
