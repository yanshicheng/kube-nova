package alertservicelogic

import (
	"context"
	"encoding/json"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/notification"
	notification2 "github.com/yanshicheng/kube-nova/application/portal-rpc/internal/notification"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type AlertChannelsTestLinkLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertChannelsTestLinkLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertChannelsTestLinkLogic {
	return &AlertChannelsTestLinkLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertChannelsTestLink 测试告警渠道连接
func (l *AlertChannelsTestLinkLogic) AlertChannelsTestLink(in *pb.TestLinkReq) (*pb.TestLinkResp, error) {
	// 生成临时 UUID
	testUUID := "test-link-" + time.Now().Format("20060102150405")

	// 构建配置对象
	config := notification.Config{
		UUID: testUUID,
		Name: "测试通道",
		Type: notification2.AlertType(in.ChannelType),
	}

	// 根据渠道类型解析具体配置
	switch notification2.AlertType(in.ChannelType) {
	case notification2.AlertTypeWeChat:
		var wechatConfig notification.WeChatConfig
		if err := json.Unmarshal([]byte(in.Config), &wechatConfig); err != nil {
			l.Errorf("解析企业微信配置失败: %v, config: %s", err, in.Config)
			return nil, errorx.Msg("企业微信配置格式错误: " + err.Error())
		}
		config.WeChat = &wechatConfig

	case notification2.AlertTypeDingTalk:
		var dingtalkConfig notification.DingTalkConfig
		if err := json.Unmarshal([]byte(in.Config), &dingtalkConfig); err != nil {
			l.Errorf("解析钉钉配置失败: %v, config: %s", err, in.Config)
			return nil, errorx.Msg("钉钉配置格式错误: " + err.Error())
		}
		config.DingTalk = &dingtalkConfig

	case notification2.AlertTypeFeiShu:
		var feishuConfig notification.FeiShuConfig
		if err := json.Unmarshal([]byte(in.Config), &feishuConfig); err != nil {
			l.Errorf("解析飞书配置失败: %v, config: %s", err, in.Config)
			return nil, errorx.Msg("飞书配置格式错误: " + err.Error())
		}
		config.FeiShu = &feishuConfig

	case notification2.AlertTypeEmail:
		var emailConfig notification.EmailConfig
		if err := json.Unmarshal([]byte(in.Config), &emailConfig); err != nil {
			l.Errorf("解析邮件配置失败: %v, config: %s", err, in.Config)
			return nil, errorx.Msg("邮件配置格式错误: " + err.Error())
		}
		config.Email = &emailConfig

	case notification2.AlertTypeWebhook:
		var webhookConfig notification.WebhookConfig
		if err := json.Unmarshal([]byte(in.Config), &webhookConfig); err != nil {
			l.Errorf("解析Webhook配置失败: %v, config: %s", err, in.Config)
			return nil, errorx.Msg("Webhook配置格式错误: " + err.Error())
		}
		config.Webhook = &webhookConfig

	case notification2.AlertTypeSMS:
		return nil, errorx.Msg("短信告警暂未实现")

	case notification2.AlertTypeVoiceCall:
		return nil, errorx.Msg("语音告警暂未实现")

	case notification2.AlertTypeSiteMessage:
		// 站内信不需要测试连接
		return &pb.TestLinkResp{}, nil

	default:
		return nil, errorx.Msg("不支持的告警渠道类型: " + in.ChannelType)
	}

	// 设置默认选项
	config.Options = notification.DefaultClientOptions()

	// 验证配置
	if err := notification2.ValidateConfig(config); err != nil {
		l.Errorf("告警渠道配置验证失败: %v", err)
		return nil, errorx.Msg("配置验证失败: " + err.Error())
	}

	// 添加临时测试渠道到全局 AlertManager
	if err := l.svcCtx.AlertManager.AddChannel(config); err != nil {
		l.Errorf("添加测试渠道失败: %v", err)
		return nil, errorx.Msg("添加测试渠道失败: " + err.Error())
	}

	// 确保测试完成后删除临时渠道
	defer func() {
		if err := l.svcCtx.AlertManager.RemoveChannel(testUUID); err != nil {
			l.Errorf("删除测试渠道失败: UUID=%s, Error=%v", testUUID, err)
		}
	}()

	// 获取刚添加的渠道客户端
	client, err := l.svcCtx.AlertManager.GetChannel(l.ctx, testUUID)
	if err != nil {
		l.Errorf("获取测试渠道客户端失败: UUID=%s, Error=%v", testUUID, err)
		return nil, errorx.Msg("获取测试渠道失败: " + err.Error())
	}

	// 调用客户端的 TestConnection 方法
	testResult, err := client.TestConnection(l.ctx, in.ToNotifys)
	if err != nil {
		l.Errorf("测试连接失败: ChannelType=%s, Error=%v", in.ChannelType, err)
		return nil, errorx.Msg("测试连接失败: " + err.Error())
	}

	// 检查测试结果
	if !testResult.Success {
		l.Errorf("测试连接不成功: ChannelType=%s, Message=%s", in.ChannelType, testResult.Message)
		return nil, errorx.Msg("测试失败: " + testResult.Message)
	}

	l.Infof("测试连接成功: ChannelType=%s, Latency=%v, Message=%s",
		in.ChannelType, testResult.Latency, testResult.Message)

	return &pb.TestLinkResp{}, nil
}
