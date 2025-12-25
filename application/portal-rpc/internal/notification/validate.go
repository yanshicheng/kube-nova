package notification

import (
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// ValidateConfig 验证配置（导出函数供外部调用）
func ValidateConfig(config Config) error {
	return validateConfig(config)
}

// validateConfig 验证配置
func validateConfig(config Config) error {
	logx.Infof("验证告警配置: Type=%s, UUID=%s, Name=%s", config.Type, config.UUID, config.Name)

	if config.UUID == "" {
		return fmt.Errorf("告警渠道 UUID 不能为空")
	}

	if config.Type == "" {
		return fmt.Errorf("告警类型不能为空")
	}

	// 根据类型验证相应的配置
	switch config.Type {
	case AlertTypeDingTalk:
		if config.DingTalk == nil {
			return fmt.Errorf("钉钉配置不能为空")
		}
		if config.DingTalk.Webhook == "" {
			return fmt.Errorf("钉钉 Webhook 地址不能为空")
		}

	case AlertTypeWeChat:
		if config.WeChat == nil {
			return fmt.Errorf("企业微信配置不能为空")
		}
		if config.WeChat.Webhook == "" {
			return fmt.Errorf("企业微信 Webhook 地址不能为空")
		}

	case AlertTypeFeiShu:
		if config.FeiShu == nil {
			return fmt.Errorf("飞书配置不能为空")
		}
		if config.FeiShu.Webhook == "" {
			return fmt.Errorf("飞书 Webhook 地址不能为空")
		}

	case AlertTypeEmail:
		if config.Email == nil {
			return fmt.Errorf("邮件配置不能为空")
		}
		if config.Email.SMTPHost == "" || config.Email.SMTPPort == 0 {
			return fmt.Errorf("SMTP 服务器地址和端口不能为空")
		}
		if config.Email.Username == "" {
			return fmt.Errorf("邮箱用户名不能为空")
		}
		if config.Email.Password == "" {
			return fmt.Errorf("邮箱密码不能为空")
		}

	case AlertTypeSMS:
		if config.SMS == nil {
			return fmt.Errorf("短信配置不能为空")
		}
		return fmt.Errorf("短信告警暂未实现")

	case AlertTypeVoiceCall:
		if config.VoiceCall == nil {
			return fmt.Errorf("语音告警配置不能为空")
		}
		return fmt.Errorf("语音告警暂未实现")

	case AlertTypeWebhook:
		if config.Webhook == nil {
			return fmt.Errorf("Webhook 配置不能为空")
		}
		if config.Webhook.URL == "" {
			return fmt.Errorf("Webhook URL 不能为空")
		}

	case AlertTypeSiteMessage:
		// 站内信不需要特殊配置验证

	default:
		return fmt.Errorf("不支持的告警类型: %s", config.Type)
	}

	// 设置默认选项
	setDefaultOptions(&config)

	return nil
}

// setDefaultOptions 设置默认选项
func setDefaultOptions(config *Config) {
	if config.Options.Timeout == 0 {
		config.Options.Timeout = 10 * time.Second
	}
	if config.Options.RetryCount == 0 {
		config.Options.RetryCount = 3
	}
	if config.Options.RetryInterval == 0 {
		config.Options.RetryInterval = 2 * time.Second
	}
	if config.Options.RateLimitPerMinute == 0 {
		config.Options.RateLimitPerMinute = 60
	}
}
