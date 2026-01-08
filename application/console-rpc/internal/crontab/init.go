package crontab

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// InitConfig 初始化配置
type InitConfig struct {
	Redis  *redis.Redis // K8s 多 Pod 部署时必需
	NodeID string       // NodeID 节点标识（可选，默认使用 hostname-pid）

	EnableDistributedLock bool // EnableDistributedLock 是否启用分布式锁

	Location *time.Location // Location 时区（可选，默认 Local）

	GlobalHooks JobHooks // GlobalHooks 全局回调（可选）

	EnableAutoRenew bool // EnableAutoRenew 是否启用锁自动续期（可选）
}

// Init 初始化定时任务管理器
func Init(cfg InitConfig, jobs ...Job) (*Manager, error) {
	manager := NewManager(ManagerConfig{
		NodeID:                cfg.NodeID,
		Redis:                 cfg.Redis,
		EnableDistributedLock: cfg.EnableDistributedLock,
		Location:              cfg.Location,
		RecoverPanic:          true,
		GlobalHooks:           cfg.GlobalHooks,
		EnableAutoRenew:       cfg.EnableAutoRenew,
	})

	// 注册所有任务
	for _, job := range jobs {
		if err := manager.RegisterJob(job); err != nil {
			logx.Errorf("[Crontab] 注册任务失败, job=%s, error=%v", job.Name(), err)
		}
	}

	return manager, nil
}

// MustInit 初始化定时任务管理器
func MustInit(cfg InitConfig, jobs ...Job) *Manager {
	manager, err := Init(cfg, jobs...)
	if err != nil {
		panic(err)
	}
	return manager
}

// QuickJob 快速创建简单任务
// 适用于不需要复杂配置的场景
func QuickJob(name, spec string, fn func(ctx context.Context) error, opts ...QuickJobOption) Job {
	job := &quickJob{
		name:    name,
		spec:    spec,
		fn:      fn,
		timeout: 5 * time.Minute,
		hooks:   &DefaultJobHooks{},
	}

	for _, opt := range opts {
		opt(job)
	}

	return job
}

// QuickJobOption 快速任务配置选项
type QuickJobOption func(*quickJob)

// QuickWithTimeout 设置超时时间
func QuickWithTimeout(timeout time.Duration) QuickJobOption {
	return func(j *quickJob) {
		j.timeout = timeout
	}
}

// QuickWithRetry 设置重试配置
func QuickWithRetry(count int, delay time.Duration) QuickJobOption {
	return func(j *quickJob) {
		j.retryCount = count
		j.retryDelay = delay
	}
}

// QuickWithHooks 设置回调
func QuickWithHooks(hooks JobHooks) QuickJobOption {
	return func(j *quickJob) {
		j.hooks = hooks
	}
}

// QuickWithAllowConcurrent 设置允许并发
func QuickWithAllowConcurrent(allow bool) QuickJobOption {
	return func(j *quickJob) {
		j.allowConcurrent = allow
	}
}

type quickJob struct {
	name            string
	spec            string
	fn              func(ctx context.Context) error
	timeout         time.Duration
	retryCount      int
	retryDelay      time.Duration
	allowConcurrent bool
	hooks           JobHooks
}

func (j *quickJob) Name() string                      { return j.name }
func (j *quickJob) Spec() string                      { return j.spec }
func (j *quickJob) Execute(ctx context.Context) error { return j.fn(ctx) }
func (j *quickJob) Timeout() time.Duration            { return j.timeout }
func (j *quickJob) RetryCount() int                   { return j.retryCount }
func (j *quickJob) RetryDelay() time.Duration         { return j.retryDelay }
func (j *quickJob) AllowConcurrent() bool             { return j.allowConcurrent }
func (j *quickJob) Hooks() JobHooks                   { return j.hooks }

// QuickJobWithHooks 快速创建带回调的任务
func QuickJobWithHooks(
	name, spec string,
	fn func(ctx context.Context) error,
	onSuccess func(ctx context.Context, result *JobResult),
	onFailure func(ctx context.Context, result *JobResult, err error),
) Job {
	hooks := NewFunctionalJobHooks()
	if onSuccess != nil {
		hooks.WithOnSuccess(func(ctx context.Context, job Job, result *JobResult) {
			onSuccess(ctx, result)
		})
	}
	if onFailure != nil {
		hooks.WithOnFailure(func(ctx context.Context, job Job, result *JobResult, err error) {
			onFailure(ctx, result, err)
		})
	}

	return QuickJob(name, spec, fn, QuickWithHooks(hooks))
}
