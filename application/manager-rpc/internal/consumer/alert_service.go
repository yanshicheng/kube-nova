package consumer

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
)

// AlertConsumerService 告警消费者服务
// 实现 go-zero 的 Service 接口，用于在 ServiceGroup 中管理生命周期
// 封装 Consumer 接口，提供统一的启动和停止方法
type AlertConsumerService struct {
	consumer Consumer           // 实际的消费者实现
	ctx      context.Context    // 服务上下文，用于控制生命周期
	cancel   context.CancelFunc // 取消函数，用于停止服务
}

// NewAlertConsumerService 创建告警消费者服务实例
// 参数：
//   - consumer: 消费者实现
//
// 返回：
//   - *AlertConsumerService: 服务实例
func NewAlertConsumerService(consumer Consumer) *AlertConsumerService {
	ctx, cancel := context.WithCancel(context.Background())
	return &AlertConsumerService{
		consumer: consumer,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 启动告警消费者服务
// 实现 go-zero Service 接口
// 该方法会阻塞直到收到停止信号
func (s *AlertConsumerService) Start() {
	logx.Info("[AlertConsumerService] 启动告警消费者...")

	// 启动消费者
	if err := s.consumer.Start(s.ctx); err != nil {
		logx.Errorf("[AlertConsumerService] 启动失败: %v", err)
		return
	}

	logx.Info("[AlertConsumerService] 告警消费者启动成功")

	// 阻塞等待停止信号
	<-s.ctx.Done()

	logx.Info("[AlertConsumerService] 收到停止信号")
}

// Stop 停止告警消费者服务
// 实现 go-zero Service 接口
// 取消上下文并停止消费者
func (s *AlertConsumerService) Stop() {
	logx.Info("[AlertConsumerService] 正在停止告警消费者...")

	// 取消上下文
	s.cancel()

	// 停止消费者
	if err := s.consumer.Stop(); err != nil {
		logx.Errorf("[AlertConsumerService] 停止失败: %v", err)
	} else {
		logx.Info("[AlertConsumerService] 告警消费者已停止")
	}
}
