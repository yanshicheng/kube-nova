// internal/consumer/alert_service.go
package consumer

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
)

// AlertConsumerService 告警消费者服务
// 实现 go-zero 的 Service 接口
type AlertConsumerService struct {
	consumer Consumer        // 实际的消费者实现
	ctx      context.Context // 服务上下文
	cancel   context.CancelFunc
}

// NewAlertConsumerService 创建告警消费者服务
func NewAlertConsumerService(consumer Consumer) *AlertConsumerService {
	ctx, cancel := context.WithCancel(context.Background())
	return &AlertConsumerService{
		consumer: consumer,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 启动告警消费者服务
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
func (s *AlertConsumerService) Stop() {
	logx.Info("[AlertConsumerService] 正在停止告警消费者...")

	s.cancel()

	if err := s.consumer.Stop(); err != nil {
		logx.Errorf("[AlertConsumerService] 停止失败: %v", err)
	} else {
		logx.Info("[AlertConsumerService] 告警消费者已停止")
	}
}
