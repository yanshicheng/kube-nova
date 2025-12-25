package consumer

import "context"

// Consumer 消费者接口
type Consumer interface {
	// Start 启动消费者
	Start(ctx context.Context) error

	// Stop 停止消费者
	Stop() error

	// Name 消费者名称
	Name() string
}
