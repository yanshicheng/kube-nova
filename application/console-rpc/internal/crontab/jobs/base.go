package jobs

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/crontab"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

// BaseJob 任务基类
type BaseJob struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext

	name            string
	spec            string
	timeout         time.Duration
	retryCount      int
	retryDelay      time.Duration
	allowConcurrent bool
	hooks           crontab.JobHooks
}

// BaseJobOption 任务配置选项
type BaseJobOption func(*BaseJob)

// NewBaseJob 创建任务基类
func NewBaseJob(svcCtx *svc.ServiceContext, name, spec string, opts ...BaseJobOption) *BaseJob {
	job := &BaseJob{
		svcCtx:          svcCtx,
		name:            name,
		spec:            spec,
		timeout:         5 * time.Minute, // 默认5分钟超时
		retryCount:      0,               // 默认不重试
		retryDelay:      5 * time.Second, // 默认5秒重试间隔
		allowConcurrent: false,           // 默认不允许并发
		hooks:           &crontab.DefaultJobHooks{},
	}

	for _, opt := range opts {
		opt(job)
	}

	return job
}

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
func WithHooks(hooks crontab.JobHooks) BaseJobOption {
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
func (j *BaseJob) Hooks() crontab.JobHooks   { return j.hooks }

// Execute 默认执行方法（子类需重写）
func (j *BaseJob) Execute(ctx context.Context) error {
	return nil
}

// SetContext 设置上下文（由调度器在执行前调用）
// 同时初始化 Logger
func (j *BaseJob) SetContext(ctx context.Context) {
	j.ctx = ctx
	j.Logger = logx.WithContext(ctx)
}

// Ctx 获取上下文
func (j *BaseJob) Ctx() context.Context {
	if j.ctx == nil {
		logx.Errorf("[Crontab] 警告: 任务 %s 的 Context 未初始化，使用 Background", j.name)
		return context.Background()
	}
	return j.ctx
}

// SvcCtx 获取服务上下文
func (j *BaseJob) SvcCtx() *svc.ServiceContext {
	return j.svcCtx
}

// ContextAwareJob 上下文感知任务接口
type ContextAwareJob interface {
	crontab.Job
	SetContext(ctx context.Context)
}
