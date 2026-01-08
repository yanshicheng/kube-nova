package alertservicelogic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/notification"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertNotifyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertNotifyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertNotifyLogic {
	return &AlertNotifyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertNotify 告警通知统一入口
func (l *AlertNotifyLogic) AlertNotify(in *pb.AlertNotifyReq) (*pb.AlertNotifyResp, error) {
	logx.Infof(" 收到告警通知请求: type=%s, userIds=%v, title=%s",
		in.AlertType, in.UserIds, in.Title)

	switch in.AlertType {
	case "prometheus":
		return l.handlePrometheusAlert(in)
	case "system":
		return l.handleSystemNotification(in)
	default:
		err := fmt.Errorf("不支持的告警类型: %s", in.AlertType)
		logx.Error(err)
		return nil, err
	}
}

// handlePrometheusAlert 处理 Prometheus 告警
func (l *AlertNotifyLogic) handlePrometheusAlert(in *pb.AlertNotifyReq) (*pb.AlertNotifyResp, error) {
	// 1. 验证参数
	if in.AlertData == "" {
		err := fmt.Errorf("alertData 不能为空")
		logx.Error(err)
		return nil, err
	}

	// 2. 解析告警数据
	var alerts []*notification.AlertInstance
	if err := json.Unmarshal([]byte(in.AlertData), &alerts); err != nil {
		return nil, fmt.Errorf("解析告警数据失败: %w", err)
	}

	if len(alerts) == 0 {
		return &pb.AlertNotifyResp{}, nil
	}

	// 3. 调用告警管理器发送通知
	// PrometheusAlertNotification 会自动：
	// - 按 ProjectID + Severity 分组
	// - 查询项目绑定的告警组
	// - 获取对应级别的通知渠道
	// - 查询告警组成员获取@人信息
	// - 并发发送到所有渠道
	// - 创建站内信
	// - 记录通知日志
	if err := l.svcCtx.AlertManager.PrometheusAlertNotification(l.ctx, alerts); err != nil {
		return nil, fmt.Errorf("告警通知发送失败: %w", err)
	}

	return &pb.AlertNotifyResp{}, nil
}

// handleSystemNotification 处理系统通知
func (l *AlertNotifyLogic) handleSystemNotification(in *pb.AlertNotifyReq) (*pb.AlertNotifyResp, error) {
	// 1. 验证参数
	if len(in.UserIds) == 0 {
		err := fmt.Errorf("userIds 不能为空")
		logx.Error(err)
		return nil, err
	}

	if in.Title == "" {
		err := fmt.Errorf("title 不能为空")
		logx.Error(err)
		return nil, err
	}

	if in.AlertData == "" {
		err := fmt.Errorf("content(alertData) 不能为空")
		logx.Error(err)
		return nil, err
	}

	logx.Infof("📬 系统通知: users=%v, title=%s", in.UserIds, in.Title)

	// 2. 调用默认通知方法
	// DefaultNotification 会：
	// - 查询默认告警组的 notification 级别渠道
	// - 发送通知给指定用户
	// - 创建站内信
	if err := l.svcCtx.AlertManager.DefaultNotification(
		l.ctx,
		in.UserIds,
		in.Title,
		in.AlertData,
	); err != nil {
		return nil, fmt.Errorf("系统通知发送失败: %w", err)
	}

	return &pb.AlertNotifyResp{}, nil
}
