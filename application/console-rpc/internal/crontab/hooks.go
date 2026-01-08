package crontab

import "context"

// JobHooks 定义任务接口回调函数
type JobHooks interface {
	// OnBeforeExecute 任务开始前调用，返回 error 将阻止任务执行，任务会被标记为 Skipped
	OnBeforeExecute(ctx context.Context, job Job) error

	// OnAfterExecute 任务结束后调用（无论成功失败都会调用）
	OnAfterExecute(ctx context.Context, job Job, result *JobResult)

	// OnSuccess 任务成功时调用
	// 在 OnAfterExecute 之前调用
	OnSuccess(ctx context.Context, job Job, result *JobResult)

	// OnFailure 任务失败时调用（包括重试耗尽后的最终失败）
	// 在 OnAfterExecute 之前调用
	OnFailure(ctx context.Context, job Job, result *JobResult, err error)

	// OnSkipped 任务被跳过时调用
	OnSkipped(ctx context.Context, job Job, reason SkipReason)

	// OnRetry 任务重试前调用
	// attempt 表示当前是第几次重试（从 1 开始）
	// err 是上次执行的错误
	// 返回 false 将终止重试，任务会立即失败
	OnRetry(ctx context.Context, job Job, attempt int, err error) bool

	// OnPanic 任务 panic 时调用
	OnPanic(ctx context.Context, job Job, recovered interface{}, stack []byte)
}

// DefaultJobHooks 提供 JobHooks 接口的默认空实现
type DefaultJobHooks struct{}

func (h *DefaultJobHooks) OnBeforeExecute(ctx context.Context, job Job) error {
	return nil
}

func (h *DefaultJobHooks) OnAfterExecute(ctx context.Context, job Job, result *JobResult) {
}

func (h *DefaultJobHooks) OnSuccess(ctx context.Context, job Job, result *JobResult) {
}

func (h *DefaultJobHooks) OnFailure(ctx context.Context, job Job, result *JobResult, err error) {
}

func (h *DefaultJobHooks) OnSkipped(ctx context.Context, job Job, reason SkipReason) {
}

func (h *DefaultJobHooks) OnRetry(ctx context.Context, job Job, attempt int, err error) bool {
	return true // 默认继续重试
}

func (h *DefaultJobHooks) OnPanic(ctx context.Context, job Job, recovered interface{}, stack []byte) {
}

// FunctionalJobHooks 函数式回调实现
type FunctionalJobHooks struct {
	beforeExecute func(ctx context.Context, job Job) error
	afterExecute  func(ctx context.Context, job Job, result *JobResult)
	onSuccess     func(ctx context.Context, job Job, result *JobResult)
	onFailure     func(ctx context.Context, job Job, result *JobResult, err error)
	onSkipped     func(ctx context.Context, job Job, reason SkipReason)
	onRetry       func(ctx context.Context, job Job, attempt int, err error) bool
	onPanic       func(ctx context.Context, job Job, recovered interface{}, stack []byte)
}

// NewFunctionalJobHooks 创建函数式回调
func NewFunctionalJobHooks() *FunctionalJobHooks {
	return &FunctionalJobHooks{}
}

// WithBeforeExecute 设置 OnBeforeExecute 回调
func (h *FunctionalJobHooks) WithBeforeExecute(fn func(ctx context.Context, job Job) error) *FunctionalJobHooks {
	h.beforeExecute = fn
	return h
}

// WithAfterExecute 设置 OnAfterExecute 回调
func (h *FunctionalJobHooks) WithAfterExecute(fn func(ctx context.Context, job Job, result *JobResult)) *FunctionalJobHooks {
	h.afterExecute = fn
	return h
}

// WithOnSuccess 设置 OnSuccess 回调
func (h *FunctionalJobHooks) WithOnSuccess(fn func(ctx context.Context, job Job, result *JobResult)) *FunctionalJobHooks {
	h.onSuccess = fn
	return h
}

// WithOnFailure 设置 OnFailure 回调
func (h *FunctionalJobHooks) WithOnFailure(fn func(ctx context.Context, job Job, result *JobResult, err error)) *FunctionalJobHooks {
	h.onFailure = fn
	return h
}

// WithOnSkipped 设置 OnSkipped 回调
func (h *FunctionalJobHooks) WithOnSkipped(fn func(ctx context.Context, job Job, reason SkipReason)) *FunctionalJobHooks {
	h.onSkipped = fn
	return h
}

// WithOnRetry 设置 OnRetry 回调
func (h *FunctionalJobHooks) WithOnRetry(fn func(ctx context.Context, job Job, attempt int, err error) bool) *FunctionalJobHooks {
	h.onRetry = fn
	return h
}

// WithOnPanic 设置 OnPanic 回调
func (h *FunctionalJobHooks) WithOnPanic(fn func(ctx context.Context, job Job, recovered interface{}, stack []byte)) *FunctionalJobHooks {
	h.onPanic = fn
	return h
}

// 实现 JobHooks 接口

func (h *FunctionalJobHooks) OnBeforeExecute(ctx context.Context, job Job) error {
	if h.beforeExecute != nil {
		return h.beforeExecute(ctx, job)
	}
	return nil
}

func (h *FunctionalJobHooks) OnAfterExecute(ctx context.Context, job Job, result *JobResult) {
	if h.afterExecute != nil {
		h.afterExecute(ctx, job, result)
	}
}

func (h *FunctionalJobHooks) OnSuccess(ctx context.Context, job Job, result *JobResult) {
	if h.onSuccess != nil {
		h.onSuccess(ctx, job, result)
	}
}

func (h *FunctionalJobHooks) OnFailure(ctx context.Context, job Job, result *JobResult, err error) {
	if h.onFailure != nil {
		h.onFailure(ctx, job, result, err)
	}
}

func (h *FunctionalJobHooks) OnSkipped(ctx context.Context, job Job, reason SkipReason) {
	if h.onSkipped != nil {
		h.onSkipped(ctx, job, reason)
	}
}

func (h *FunctionalJobHooks) OnRetry(ctx context.Context, job Job, attempt int, err error) bool {
	if h.onRetry != nil {
		return h.onRetry(ctx, job, attempt, err)
	}
	return true
}

func (h *FunctionalJobHooks) OnPanic(ctx context.Context, job Job, recovered interface{}, stack []byte) {
	if h.onPanic != nil {
		h.onPanic(ctx, job, recovered, stack)
	}
}

// ChainJobHooks 链式回调，允许组合多个 JobHooks
// 所有 hooks 会按顺序执行
type ChainJobHooks struct {
	hooks []JobHooks
}

// NewChainJobHooks 创建链式回调
func NewChainJobHooks(hooks ...JobHooks) *ChainJobHooks {
	return &ChainJobHooks{hooks: hooks}
}

// Add 添加回调
func (c *ChainJobHooks) Add(hook JobHooks) *ChainJobHooks {
	c.hooks = append(c.hooks, hook)
	return c
}

func (c *ChainJobHooks) OnBeforeExecute(ctx context.Context, job Job) error {
	for _, h := range c.hooks {
		if err := h.OnBeforeExecute(ctx, job); err != nil {
			return err
		}
	}
	return nil
}

func (c *ChainJobHooks) OnAfterExecute(ctx context.Context, job Job, result *JobResult) {
	for _, h := range c.hooks {
		h.OnAfterExecute(ctx, job, result)
	}
}

func (c *ChainJobHooks) OnSuccess(ctx context.Context, job Job, result *JobResult) {
	for _, h := range c.hooks {
		h.OnSuccess(ctx, job, result)
	}
}

func (c *ChainJobHooks) OnFailure(ctx context.Context, job Job, result *JobResult, err error) {
	for _, h := range c.hooks {
		h.OnFailure(ctx, job, result, err)
	}
}

func (c *ChainJobHooks) OnSkipped(ctx context.Context, job Job, reason SkipReason) {
	for _, h := range c.hooks {
		h.OnSkipped(ctx, job, reason)
	}
}

func (c *ChainJobHooks) OnRetry(ctx context.Context, job Job, attempt int, err error) bool {
	for _, h := range c.hooks {
		if !h.OnRetry(ctx, job, attempt, err) {
			return false
		}
	}
	return true
}

func (c *ChainJobHooks) OnPanic(ctx context.Context, job Job, recovered interface{}, stack []byte) {
	for _, h := range c.hooks {
		h.OnPanic(ctx, job, recovered, stack)
	}
}
