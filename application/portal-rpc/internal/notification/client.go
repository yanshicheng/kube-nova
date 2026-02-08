package notification

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

// RetryContext 重试上下文信息
// 包含重试时需要记录的详细信息，便于问题排查
type RetryContext struct {
	// ChannelType 渠道类型: dingtalk、email、wechat、feishu、webhook、site_message
	ChannelType string
	// ChannelUUID 渠道唯一标识
	ChannelUUID string
	// ChannelName 渠道名称
	ChannelName string
	// Operation 操作类型: send_alert、send_notification、test_connection
	Operation string
	// TargetInfo 目标信息，如收件人、webhook 地址等
	TargetInfo string
}

// baseClient 基础客户端实现
// 提供所有告警客户端的通用功能
type baseClient struct {
	// config 客户端配置
	config Config

	// l 日志记录器
	l logx.Logger
	// ctx 上下文
	ctx context.Context

	// healthy 健康状态标志，1 表示健康，0 表示不健康
	healthy int32
	// closed 关闭状态标志，1 表示已关闭，0 表示运行中
	closed int32
	// healthMutex 健康状态更新锁
	healthMutex sync.Mutex

	// lastHealthCheckTime 最后健康检查时间
	lastHealthCheckTime time.Time
	// lastHealthCheckError 最后健康检查错误
	lastHealthCheckError error

	// statsMu 统计信息读写锁
	statsMu sync.RWMutex
	// stats 统计信息
	stats Statistics

	// rateLimiter 速率限制器
	rateLimiter *rateLimiter
}

// rateLimiter 滑动窗口速率限制器
// 使用环形缓冲区实现高效的速率限制
type rateLimiter struct {
	// limit 窗口内允许的最大请求数
	limit int
	// window 时间窗口大小
	window time.Duration
	// mu 并发保护锁
	mu sync.Mutex

	// timestamps 环形缓冲区，存储请求时间戳（纳秒）
	timestamps []int64
	// head 缓冲区头指针，指向下一个写入位置
	head int
	// count 当前窗口内的请求计数
	count int
}

// newRateLimiter 创建速率限制器
// limitPerMinute 为 0 或负数时表示不限制
func newRateLimiter(limitPerMinute int) *rateLimiter {
	if limitPerMinute <= 0 {
		return &rateLimiter{
			limit: 0, // 不限制
		}
	}
	return &rateLimiter{
		limit:      limitPerMinute,
		window:     time.Minute,
		timestamps: make([]int64, limitPerMinute), // 预分配固定大小，避免动态扩容
	}
}

// Allow 检查是否允许请求通过
// 使用滑动窗口算法，时间复杂度 O(过期请求数)，空间复杂度 O(limit)
func (r *rateLimiter) Allow() bool {
	// 不限制的情况直接放行
	if r.limit <= 0 {
		return true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UnixNano()
	windowStart := now - int64(r.window)

	// 清理过期的时间戳
	// 由于时间戳是按顺序添加的，只需要从最旧的开始检查
	for r.count > 0 {
		// 计算最旧时间戳的索引（环形缓冲区）
		oldestIdx := (r.head - r.count + len(r.timestamps)) % len(r.timestamps)
		if r.timestamps[oldestIdx] > windowStart {
			// 最旧的时间戳还在窗口内，停止清理
			break
		}
		r.count--
	}

	// 检查是否超过限制
	if r.count >= r.limit {
		return false
	}

	// 记录新请求的时间戳
	r.timestamps[r.head] = now
	r.head = (r.head + 1) % len(r.timestamps)
	r.count++
	return true
}

// GetCurrentCount 获取当前窗口内的请求数（用于监控）
func (r *rateLimiter) GetCurrentCount() int {
	if r.limit <= 0 {
		return 0
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 先清理过期的
	now := time.Now().UnixNano()
	windowStart := now - int64(r.window)

	for r.count > 0 {
		oldestIdx := (r.head - r.count + len(r.timestamps)) % len(r.timestamps)
		if r.timestamps[oldestIdx] > windowStart {
			break
		}
		r.count--
	}

	return r.count
}

// newBaseClient 创建基础客户端
// 初始化通用配置和默认值
func newBaseClient(config Config) *baseClient {
	// 设置默认超时时间
	if config.Options.Timeout == 0 {
		config.Options.Timeout = 10 * time.Second
	}
	// 设置默认重试次数
	if config.Options.RetryCount == 0 {
		config.Options.RetryCount = 3
	}
	// 设置默认重试间隔
	if config.Options.RetryInterval == 0 {
		config.Options.RetryInterval = 2 * time.Second
	}
	// 设置默认速率限制
	if config.Options.RateLimitPerMinute == 0 {
		config.Options.RateLimitPerMinute = 60
	}

	bc := &baseClient{
		config:      config,
		ctx:         context.Background(),
		l:           logx.WithContext(context.Background()),
		rateLimiter: newRateLimiter(config.Options.RateLimitPerMinute),
	}

	// 初始化状态为健康且运行中
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

// GetUUID 获取渠道唯一标识
func (bc *baseClient) GetUUID() string {
	return bc.config.UUID
}

// GetName 获取渠道名称
// 如果未配置名称则返回 UUID
func (bc *baseClient) GetName() string {
	if bc.config.Name != "" {
		return bc.config.Name
	}
	return bc.config.UUID
}

// GetType 获取渠道类型
func (bc *baseClient) GetType() AlertType {
	return bc.config.Type
}

// GetConfig 获取渠道配置
func (bc *baseClient) GetConfig() Config {
	return bc.config
}

// WithContext 设置上下文
// 返回带有新上下文的客户端副本，避免并发问题
func (bc *baseClient) WithContext(ctx context.Context) Client {
	// 返回一个浅拷贝，只修改必要的字段
	newClient := &baseClient{
		config:      bc.config,
		ctx:         ctx,
		l:           logx.WithContext(ctx),
		healthy:     bc.healthy, // 原子值，直接复制
		closed:      bc.closed,  // 原子值，直接复制
		rateLimiter: bc.rateLimiter,
	}
	// 复制统计信息
	newClient.stats = bc.stats
	return newClient
}

// IsHealthy 快速检查是否健康
// 同时检查关闭状态和健康状态
func (bc *baseClient) IsHealthy() bool {
	if atomic.LoadInt32(&bc.closed) == 1 {
		return false
	}
	return atomic.LoadInt32(&bc.healthy) == 1
}

// GetStatistics 获取统计信息的副本
// 返回副本以避免外部修改
func (bc *baseClient) GetStatistics() *Statistics {
	bc.statsMu.RLock()
	defer bc.statsMu.RUnlock()

	// 创建副本
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
// 使用 CAS 操作确保只关闭一次
func (bc *baseClient) Close() error {
	if !atomic.CompareAndSwapInt32(&bc.closed, 0, 1) {
		return nil // 已经关闭过
	}

	bc.l.Infof("关闭告警客户端: %s (类型: %s)", bc.config.UUID, bc.config.Type)
	return nil
}

// recordSuccess 记录发送成功
// latency 为本次请求的耗时
func (bc *baseClient) recordSuccess(latency time.Duration) {
	// 未启用指标统计时直接返回
	if !bc.config.Options.EnableMetrics {
		return
	}

	bc.statsMu.Lock()
	defer bc.statsMu.Unlock()

	bc.stats.TotalSent++
	bc.stats.TotalSuccess++
	bc.stats.LastSentTime = time.Now()
	bc.stats.LastSuccessTime = time.Now()

	// 累加总延迟
	bc.stats.TotalLatency += latency

	// 计算真实平均延迟（总延迟 / 成功次数）
	if bc.stats.TotalSuccess > 0 {
		bc.stats.AverageLatency = bc.stats.TotalLatency / time.Duration(bc.stats.TotalSuccess)
	}
}

// recordFailure 记录发送失败
func (bc *baseClient) recordFailure() {
	// 未启用指标统计时直接返回
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
// 返回 nil 表示允许通过，返回 error 表示被限流
func (bc *baseClient) checkRateLimit() error {
	if !bc.rateLimiter.Allow() {
		return errorx.Msg(fmt.Sprintf("超过速率限制: 每分钟最多 %d 条", bc.config.Options.RateLimitPerMinute))
	}
	return nil
}

// retryWithBackoff 带重试的操作执行（不带详细上下文信息）
// 保持向后兼容
func (bc *baseClient) retryWithBackoff(ctx context.Context, operation func() error) error {
	retryCtx := &RetryContext{
		ChannelType: string(bc.config.Type),
		ChannelUUID: bc.config.UUID,
		ChannelName: bc.config.Name,
		Operation:   "unknown",
	}
	return bc.retryWithBackoffContext(ctx, retryCtx, operation)
}

// retryWithBackoffContext 带详细上下文的重试机制
// 支持指数退避和详细的重试日志
func (bc *baseClient) retryWithBackoffContext(ctx context.Context, retryCtx *RetryContext, operation func() error) error {
	var lastErr error

	for i := 0; i <= bc.config.Options.RetryCount; i++ {
		// 非首次尝试需要等待
		if i > 0 {
			// 记录重试日志
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

			// 等待重试间隔，同时监听上下文取消
			select {
			case <-ctx.Done():
				bc.l.Errorf("[重试中断] 上下文取消 | 渠道类型: %s | 渠道UUID: %s | 错误: %v",
					retryCtx.ChannelType, retryCtx.ChannelUUID, ctx.Err())
				return ctx.Err()
			case <-time.After(bc.config.Options.RetryInterval):
				// 继续重试
			}
		}

		// 执行操作
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

		// 操作成功
		if i > 0 {
			bc.l.Infof("[重试成功] 第 %d 次重试成功 | 渠道类型: %s | 渠道UUID: %s | 操作: %s",
				i, retryCtx.ChannelType, retryCtx.ChannelUUID, retryCtx.Operation)
		}
		return nil
	}

	// 所有重试都失败
	finalErr := errorx.Msg(fmt.Sprintf("[重试失败] 渠道类型: %s | 渠道UUID: %s | 渠道名称: %s | 操作: %s | 重试 %d 次后仍然失败: %v",
		retryCtx.ChannelType,
		retryCtx.ChannelUUID,
		retryCtx.ChannelName,
		retryCtx.Operation,
		bc.config.Options.RetryCount,
		lastErr,
	))
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
// 子类可以重写此方法实现更复杂的健康检查逻辑
func (bc *baseClient) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	// 检查是否已关闭
	if atomic.LoadInt32(&bc.closed) == 1 {
		return &HealthStatus{
			Healthy:       false,
			Message:       "客户端已关闭",
			LastCheckTime: time.Now(),
		}, errorx.Msg("客户端已关闭")
	}

	bc.updateHealthStatus(true, nil)
	return &HealthStatus{
		Healthy:       true,
		Message:       "客户端健康",
		LastCheckTime: time.Now(),
	}, nil
}

// TestConnection 默认测试连接实现
// 子类必须重写此方法实现实际的连接测试
func (bc *baseClient) TestConnection(ctx context.Context, toEmail []string) (*TestResult, error) {
	return nil, errorx.Msg("TestConnection 方法未实现")
}
