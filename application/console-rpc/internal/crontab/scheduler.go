package crontab

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// Scheduler 调度器接口
type Scheduler interface {
	// Start 启动调度器
	Start() error

	// Stop 停止调度器
	Stop() error

	// AddJob 添加任务
	AddJob(job Job) error

	// RemoveJob 移除任务
	RemoveJob(name string) error

	// IsRunning 是否正在运行
	IsRunning() bool

	// GetRegistry 获取任务注册中心
	GetRegistry() Registry

	// TriggerJob 手动触发任务
	TriggerJob(ctx context.Context, name string) error
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	// NodeID 节点标识，用于分布式锁和日志
	// 如果为空，将使用 hostname + pid
	NodeID string

	// Redis 客户端，用于分布式锁
	Redis *redis.Redis

	// EnableDistributedLock 是否启用分布式锁
	// 在 K8s 多 Pod 环境下应该启用
	EnableDistributedLock bool

	// LockTTLMultiplier 锁过期时间倍数
	// 锁的 TTL = 任务超时时间 * LockTTLMultiplier
	// 默认 1.5
	LockTTLMultiplier float64

	// RecoverPanic 是否恢复 panic
	RecoverPanic bool

	// Location 时区
	Location *time.Location

	// GlobalHooks 全局回调，所有任务都会触发
	// 任务自身的回调会在全局回调之后执行
	GlobalHooks JobHooks

	// EnableAutoRenew 是否启用锁自动续期
	// 适用于执行时间不确定的长任务
	EnableAutoRenew bool
}

// DefaultScheduler 默认调度器实现
type DefaultScheduler struct {
	config      SchedulerConfig
	cron        *cron.Cron
	registry    Registry
	locker      DistributedLocker
	nodeID      string
	running     int32
	mu          sync.RWMutex
	entryIDs    map[string]cron.EntryID
	jobStatus   map[string]*jobStatusInfo
	globalHooks JobHooks
}

type jobStatusInfo struct {
	lastRun    time.Time
	lastStatus JobStatus
	runCount   int64
	failCount  int64
	mu         sync.RWMutex
}

// NewScheduler 创建调度器
func NewScheduler(cfg SchedulerConfig) *DefaultScheduler {
	// 设置默认值
	if cfg.NodeID == "" {
		hostname, _ := os.Hostname()
		cfg.NodeID = fmt.Sprintf("%s-%d", hostname, os.Getpid())
	}

	if cfg.LockTTLMultiplier <= 0 {
		cfg.LockTTLMultiplier = 1.5
	}

	if cfg.Location == nil {
		cfg.Location = time.Local
	}

	// 创建 cron 实例
	cronOpts := []cron.Option{
		cron.WithSeconds(), // 支持秒级调度
		cron.WithLocation(cfg.Location),
	}

	if cfg.RecoverPanic {
		cronOpts = append(cronOpts, cron.WithChain(cron.Recover(cron.DefaultLogger)))
	}

	scheduler := &DefaultScheduler{
		config:      cfg,
		cron:        cron.New(cronOpts...),
		registry:    NewRegistry(),
		nodeID:      cfg.NodeID,
		entryIDs:    make(map[string]cron.EntryID),
		jobStatus:   make(map[string]*jobStatusInfo),
		globalHooks: cfg.GlobalHooks,
	}

	// 初始化分布式锁
	if cfg.EnableDistributedLock && cfg.Redis != nil {
		if cfg.EnableAutoRenew {
			scheduler.locker = NewLockWithAutoRenew(cfg.Redis, cfg.NodeID)
		} else {
			scheduler.locker = NewRedisDistributedLock(cfg.Redis, cfg.NodeID)
		}
	} else {
		scheduler.locker = NewNoopLocker(cfg.NodeID)
	}

	return scheduler
}

// Start 启动调度器
func (s *DefaultScheduler) Start() error {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return fmt.Errorf("scheduler already running")
	}

	logx.Infof("Crontab 启动调度器, nodeID=%s, 分布式锁=%v, 任务数=%d",
		s.nodeID, s.config.EnableDistributedLock, s.registry.Count())

	s.cron.Start()
	return nil
}

// Stop 停止调度器
func (s *DefaultScheduler) Stop() error {
	if !atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		return fmt.Errorf("scheduler not running")
	}

	logx.Infof("Crontab 停止调度器, nodeID=%s", s.nodeID)

	ctx := s.cron.Stop()
	<-ctx.Done()

	return nil
}

// AddJob 添加任务
func (s *DefaultScheduler) AddJob(job Job) error {
	if job == nil {
		return fmt.Errorf("job cannot be nil")
	}

	// 注册到注册中心
	if err := s.registry.Register(job); err != nil {
		return fmt.Errorf("register job failed: %w", err)
	}

	// 包装任务执行函数
	wrappedFunc := s.wrapJobFunc(job)

	// 添加到 cron
	entryID, err := s.cron.AddFunc(job.Spec(), wrappedFunc)
	if err != nil {
		// 注册失败，回滚
		s.registry.Unregister(job.Name())
		return fmt.Errorf("add cron func failed: %w", err)
	}

	s.mu.Lock()
	s.entryIDs[job.Name()] = entryID
	s.jobStatus[job.Name()] = &jobStatusInfo{}
	s.mu.Unlock()

	logx.Infof("Crontab 添加任务成功, name=%s, spec=%s, timeout=%v",
		job.Name(), job.Spec(), job.Timeout())

	return nil
}

// RemoveJob 移除任务
func (s *DefaultScheduler) RemoveJob(name string) error {
	s.mu.Lock()
	entryID, exists := s.entryIDs[name]
	if exists {
		delete(s.entryIDs, name)
		delete(s.jobStatus, name)
	}
	s.mu.Unlock()

	if !exists {
		return fmt.Errorf("job '%s' not found", name)
	}

	s.cron.Remove(entryID)

	if err := s.registry.Unregister(name); err != nil {
		return fmt.Errorf("unregister job failed: %w", err)
	}

	logx.Infof("Crontab 移除任务成功, name=%s", name)
	return nil
}

// IsRunning 是否正在运行
func (s *DefaultScheduler) IsRunning() bool {
	return atomic.LoadInt32(&s.running) == 1
}

// GetRegistry 获取任务注册中心
func (s *DefaultScheduler) GetRegistry() Registry {
	return s.registry
}

// TriggerJob 手动触发任务
func (s *DefaultScheduler) TriggerJob(ctx context.Context, name string) error {
	job, exists := s.registry.Get(name)
	if !exists {
		return fmt.Errorf("job '%s' not found", name)
	}

	logx.WithContext(ctx).Infof("Crontab 手动触发任务, name=%s, nodeID=%s",
		name, s.nodeID)

	// 异步执行
	go s.executeJob(ctx, job)

	return nil
}

// wrapJobFunc 包装任务执行函数
func (s *DefaultScheduler) wrapJobFunc(job Job) func() {
	return func() {
		ctx := context.Background()
		s.executeJob(ctx, job)
	}
}

// executeJob 执行任务（核心执行流程）
func (s *DefaultScheduler) executeJob(ctx context.Context, job Job) {
	jobName := job.Name()
	startTime := time.Now()

	// 获取回调钩子
	hooks := s.getEffectiveHooks(job)

	// 初始化结果
	result := &JobResult{
		JobName:   jobName,
		StartTime: startTime,
		NodeID:    s.nodeID,
	}

	// 更新状态为运行中
	s.updateJobStatus(jobName, JobStatusRunning)

	// 计算锁的 TTL
	lockTTL := time.Duration(float64(job.Timeout()) * s.config.LockTTLMultiplier)

	// 尝试获取分布式锁
	var release func()
	if !job.AllowConcurrent() {
		acquired, releaseFunc, err := s.locker.TryLock(ctx, jobName, lockTTL)
		if err != nil {
			logx.WithContext(ctx).Errorf("Crontab 获取锁失败, job=%s, error=%v", jobName, err)
			s.updateJobStatusWithFail(jobName)

			// 构建失败结果
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			result.Success = false
			result.Error = fmt.Sprintf("获取分布式锁失败: %v", err)

			// 回调：失败
			s.safeCallHook(func() {
				hooks.OnFailure(ctx, job, result, err)
			})
			// 回调：执行后
			s.safeCallHook(func() {
				hooks.OnAfterExecute(ctx, job, result)
			})
			return
		}

		if !acquired {
			logx.WithContext(ctx).Infof("Crontab 任务跳过(其他节点执行中), job=%s, nodeID=%s",
				jobName, s.nodeID)
			s.updateJobStatus(jobName, JobStatusSkipped)
			// 回调：跳过
			s.safeCallHook(func() {
				hooks.OnSkipped(ctx, job, SkipReasonLockHeld)
			})
			return
		}

		release = releaseFunc
		defer release()
	}

	// 回调：执行前
	if err := s.safeCallBeforeHook(ctx, job, hooks); err != nil {
		logx.WithContext(ctx).Infof("Crontab OnBeforeExecute 返回错误，跳过任务, job=%s, error=%v",
			jobName, err)
		s.updateJobStatus(jobName, JobStatusSkipped)
		// 回调：跳过
		s.safeCallHook(func() {
			hooks.OnSkipped(ctx, job, SkipReasonBeforeHookErr)
		})
		return
	}

	// 创建带超时的 context
	execCtx, cancel := context.WithTimeout(ctx, job.Timeout())
	defer cancel()

	// 执行任务（带重试）
	var lastErr error
	maxRetries := job.RetryCount()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// 回调：重试
			shouldContinue := s.safeCallRetryHook(execCtx, job, hooks, attempt, lastErr)
			if !shouldContinue {
				logx.WithContext(execCtx).Infof("Crontab OnRetry 返回 false，终止重试, job=%s, attempt=%d",
					jobName, attempt)
				break
			}
			time.Sleep(job.RetryDelay())
		}

		result.Retries = attempt
		lastErr = s.safeExecute(execCtx, job, hooks)
		if lastErr == nil {
			break
		}

		logx.WithContext(execCtx).Errorf("Crontab 任务执行失败, job=%s, attempt=%d/%d, error=%v",
			jobName, attempt, maxRetries, lastErr)
	}

	// 计算结果
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if lastErr != nil {
		result.Success = false
		result.Error = lastErr.Error()
		s.updateJobStatusWithFail(jobName)

		logx.WithContext(ctx).Errorf("Crontab 任务最终失败, job=%s, duration=%v, retries=%d, error=%v",
			jobName, result.Duration, result.Retries, lastErr)

		// 回调：失败
		s.safeCallHook(func() {
			hooks.OnFailure(ctx, job, result, lastErr)
		})
	} else {
		result.Success = true
		s.updateJobStatus(jobName, JobStatusSuccess)

		logx.WithContext(ctx).Infof("Crontab 任务执行成功, job=%s, duration=%v, nodeID=%s",
			jobName, result.Duration, s.nodeID)

		// 回调：成功
		s.safeCallHook(func() {
			hooks.OnSuccess(ctx, job, result)
		})
	}

	// 回调：执行后（无论成功失败）
	s.safeCallHook(func() {
		hooks.OnAfterExecute(ctx, job, result)
	})
}

// safeExecute 安全执行任务
// safeExecute 安全执行任务（捕获 panic）
func (s *DefaultScheduler) safeExecute(ctx context.Context, job Job, hooks JobHooks) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			err = fmt.Errorf("job panic: %v", r)
			logx.WithContext(ctx).Errorf("Crontab 任务 panic, job=%s, panic=%v\nstack:\n%s",
				job.Name(), r, string(stack))
			// 回调：panic
			s.safeCallHook(func() {
				hooks.OnPanic(ctx, job, r, stack)
			})
		}
	}()

	// 自动设置 Context，初始化 Logger
	if setter, ok := job.(interface{ SetContext(context.Context) }); ok {
		setter.SetContext(ctx)
	}

	return job.Execute(ctx)
}

// getEffectiveHooks 获取有效的回调钩子
// 如果任务有自己的回调，使用任务回调；否则使用全局回调
// 如果两者都有，则组合使用（全局回调先执行）
func (s *DefaultScheduler) getEffectiveHooks(job Job) JobHooks {
	jobHooks := job.Hooks()

	if s.globalHooks == nil && jobHooks == nil {
		return &DefaultJobHooks{}
	}

	if s.globalHooks == nil {
		return jobHooks
	}

	if jobHooks == nil {
		return s.globalHooks
	}

	// 组合全局回调和任务回调
	return NewChainJobHooks(s.globalHooks, jobHooks)
}

// safeCallHook 安全调用回调（捕获回调中的 panic）
func (s *DefaultScheduler) safeCallHook(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			logx.Errorf("Crontab 回调函数 panic: %v\n%s", r, debug.Stack())
		}
	}()
	fn()
}

// safeCallBeforeHook 安全调用 OnBeforeExecute
func (s *DefaultScheduler) safeCallBeforeHook(ctx context.Context, job Job, hooks JobHooks) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("OnBeforeExecute panic: %v", r)
			logx.Errorf("Crontab OnBeforeExecute panic: %v\n%s", r, debug.Stack())
		}
	}()
	return hooks.OnBeforeExecute(ctx, job)
}

// safeCallRetryHook 安全调用 OnRetry
func (s *DefaultScheduler) safeCallRetryHook(ctx context.Context, job Job, hooks JobHooks, attempt int, err error) (shouldContinue bool) {
	defer func() {
		if r := recover(); r != nil {
			shouldContinue = false // panic 时终止重试
			logx.Errorf("Crontab OnRetry panic: %v\n%s", r, debug.Stack())
		}
	}()
	return hooks.OnRetry(ctx, job, attempt, err)
}

// updateJobStatus 更新任务状态
func (s *DefaultScheduler) updateJobStatus(name string, status JobStatus) {
	s.mu.RLock()
	info, exists := s.jobStatus[name]
	s.mu.RUnlock()

	if exists {
		info.mu.Lock()
		info.lastRun = time.Now()
		info.lastStatus = status
		if status == JobStatusSuccess {
			info.runCount++
		}
		info.mu.Unlock()
	}
}

// updateJobStatusWithFail 更新任务失败状态
func (s *DefaultScheduler) updateJobStatusWithFail(name string) {
	s.mu.RLock()
	info, exists := s.jobStatus[name]
	s.mu.RUnlock()

	if exists {
		info.mu.Lock()
		info.lastRun = time.Now()
		info.lastStatus = JobStatusFailed
		info.failCount++
		info.mu.Unlock()
	}
}

// GetJobStatus 获取任务状态
func (s *DefaultScheduler) GetJobStatus(name string) (JobStatus, time.Time, int64, int64, bool) {
	s.mu.RLock()
	info, exists := s.jobStatus[name]
	s.mu.RUnlock()

	if !exists {
		return JobStatusIdle, time.Time{}, 0, 0, false
	}

	info.mu.RLock()
	defer info.mu.RUnlock()
	return info.lastStatus, info.lastRun, info.runCount, info.failCount, true
}

// GetNodeID 获取节点ID
func (s *DefaultScheduler) GetNodeID() string {
	return s.nodeID
}

// GetAllJobStatus 获取所有任务状态
func (s *DefaultScheduler) GetAllJobStatus() map[string]*JobStatusSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*JobStatusSummary)
	for name, info := range s.jobStatus {
		info.mu.RLock()
		result[name] = &JobStatusSummary{
			Name:       name,
			LastRun:    info.lastRun,
			LastStatus: info.lastStatus.String(),
			RunCount:   info.runCount,
			FailCount:  info.failCount,
		}
		info.mu.RUnlock()
	}
	return result
}

// JobStatusSummary 任务状态摘要
type JobStatusSummary struct {
	Name       string    `json:"name"`
	LastRun    time.Time `json:"last_run"`
	LastStatus string    `json:"last_status"`
	RunCount   int64     `json:"run_count"`
	FailCount  int64     `json:"fail_count"`
}
