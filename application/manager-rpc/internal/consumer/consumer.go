package consumer

import "context"

// Consumer 消费者接口
// 定义消费者的基本行为，所有消费者实现都应实现此接口
type Consumer interface {
	// Start 启动消费者
	// 参数：
	//   - ctx: 上下文，用于控制消费者生命周期
	// 返回：
	//   - error: 启动失败时返回错误
	Start(ctx context.Context) error

	// Stop 停止消费者
	// 应确保所有工作协程安全退出
	// 返回：
	//   - error: 停止失败时返回错误
	Stop() error

	// Name 返回消费者名称
	// 用于日志标识和监控
	// 返回：
	//   - string: 消费者名称
	Name() string
}
