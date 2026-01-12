package crontab

import (
	"context"
	"time"
)

// 定义 Job 定时任务接口

type Job interface {
	// Name 返回任务名，用于日志记录和分布式锁
	Name() string

	// Spec 返回 cron 表达式
	Spec() string

	// Execute 执行任务逻辑
	Execute(ctx context.Context) error

	// Timeout 返回任务执行超时时间  如果任务执行时间超过此值，ctx 将被取消
	Timeout() time.Duration

	// RetryCount 返回任务失败重试次数 0 表示不重试
	RetryCount() int

	// RetryDelay 获取任务失败重试间隔时间
	RetryDelay() time.Duration

	// AllowConcurrent 是否允许并发执行, 如果设置为 true 则分布式场景下所有的pod都会执行任务
	AllowConcurrent() bool

	// Hooks 返回任务的回调函数 返回 nil 表示没有回调函数
	Hooks() JobHooks
}

// JobResult 任务执行结果
type JobResult struct {
	JobName   string        `json:"job_name"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
	Success   bool          `json:"success"`
	Error     string        `json:"error"`
	NodeID    string        `json:"node_id"`
	Retries   int           `json:"retries"`
}

// JobStatus 任务状态
type JobStatus int

const (
	JobStatusIdle    JobStatus = iota // 空闲
	JobStatusRunning                  // 运行中
	JobStatusSuccess                  // 成功
	JobStatusFailed                   // 失败
	JobStatusSkipped                  // 跳过
)

func (s JobStatus) String() string {
	switch s {
	case JobStatusIdle:
		return "idle"
	case JobStatusRunning:
		return "running"
	case JobStatusSuccess:
		return "success"
	case JobStatusFailed:
		return "failed"
	case JobStatusSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// SkipReason 任务跳过原因

type SkipReason int

const (
	SkipReasonLockHeld      SkipReason = iota // 其他节点持有分布式锁
	SkipReasonBeforeHookErr                   // 前置任务执行失败
	SkipReasonContextDone                     // 上下文取消 或者超时
)

func (s SkipReason) String() string {
	switch s {
	case SkipReasonLockHeld:
		return "lock_held_by_other_node"
	case SkipReasonBeforeHookErr:
		return "before_hook_err"
	case SkipReasonContextDone:
		return "context_done"
	default:
		return "unknown"
	}
}

// BaseJob 提供 job 接口的默认实现
type BaseJob struct {
	name            string
	spec            string
	timeout         time.Duration
	retryCount      int
	retryDelay      time.Duration
	allowConcurrent bool
	hooks           JobHooks
}

// NewBaseJob 创建一个 BaseJob
func NewBaseJob(name, spec string, opts ...BaseJobOption) *BaseJob {
	job := &BaseJob{
		name:            name,
		spec:            spec,
		timeout:         5 * time.Minute,    // 默认5分钟超时
		retryCount:      0,                  // 默认不重试
		retryDelay:      time.Second * 5,    // 默认5秒重试间隔
		allowConcurrent: false,              // 默认不允许并发
		hooks:           &DefaultJobHooks{}, // 默认空回调
	}

	for _, opt := range opts {
		opt(job)
	}

	return job
}

// BaseJobOption 基础任务配置选项
type BaseJobOption func(*BaseJob)

// WithTimeout 设置超时时间
func WithTimeout(timeout time.Duration) BaseJobOption {
	return func(j *BaseJob) {
		j.timeout = timeout
	}
}

// WithRetry 设置重试配置
func WithRetry(count int, delay time.Duration) BaseJobOption {
	return func(j *BaseJob) {
		j.retryCount = count
		j.retryDelay = delay
	}
}

// WithAllowConcurrent 设置是否允许并发
func WithAllowConcurrent(allow bool) BaseJobOption {
	return func(j *BaseJob) {
		j.allowConcurrent = allow
	}
}

// WithHooks 设置回调钩子
func WithHooks(hooks JobHooks) BaseJobOption {
	return func(j *BaseJob) {
		j.hooks = hooks
	}
}

func (j *BaseJob) Name() string              { return j.name }
func (j *BaseJob) Spec() string              { return j.spec }
func (j *BaseJob) Timeout() time.Duration    { return j.timeout }
func (j *BaseJob) RetryCount() int           { return j.retryCount }
func (j *BaseJob) RetryDelay() time.Duration { return j.retryDelay }
func (j *BaseJob) AllowConcurrent() bool     { return j.allowConcurrent }
func (j *BaseJob) Hooks() JobHooks           { return j.hooks }

func (j *BaseJob) Execute(ctx context.Context) error {
	return nil
}
