package cluster

import (
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// Config 集群认证配置文件
type Config struct {

	// ID 集群唯一识别符
	ID string `json:"id" yaml:"id"`
	// Name 集群名称
	Name string `json:"name" yaml:"name"`
	// AuthType 集群认证方式
	AuthType AuthType `json:"authType" yaml:"authType"`
	// KubeConfigData kubeconfig 文件路径
	KubeConfigData string `json:"KubeConfigData,omitempty" yaml:"KubeConfigData,omitempty"`
	// APIServer 集群 API 服务器地址
	APIServer string `json:"apiServer,omitempty" yaml:"apiServer,omitempty"`
	// Token 集群认证 Token
	Token string `json:"token,omitempty" yaml:"token,omitempty"`
	// KeyFile 集群 客户端密钥 文件路径
	KeyFile string `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
	// CertFile 集群 客户端认证证书 文件路径
	CertFile string `json:"certFile,omitempty" yaml:"certFile,omitempty"`
	// CAFile 集群 CA 文件路径
	CAFile string `json:"caFile,omitempty" yaml:"caFile,omitempty"`
	// Insecure 是否跳过 TLS 验证
	Insecure bool `json:"insecure,omitempty" yaml:"insecure,omitempty"`
	// Options 集群客户端配置选项
	Options ClientOptions `json:"options,omitempty" yaml:"options,omitempty"`
}

// 客户端选项
type ClientOptions struct {
	// EnableInformer 是否启用 Informer
	EnableInformer bool `json:"enableInformer" yaml:"enableInformer"`
	// ResyncPeriod Informer 重新同步周期
	ResyncPeriod time.Duration `json:"resyncPeriod,omitempty" yaml:"resyncPeriod,omitempty"`
	// QPS 集群客户端每秒最大请求数
	QPS float32 `json:"qps,omitempty" yaml:"qps,omitempty"`
	// Burst 集群客户端请求突发大小
	Burst int `json:"burst,omitempty" yaml:"burst,omitempty"`
	// Timeout 集群客户端请求超时时间
	Timeout time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	// MaxConcurrentRequests 集群客户端最大并发请求数
	MaxConcurrentRequests int `json:"maxConcurrentRequests,omitempty" yaml:"maxConcurrentRequests,omitempty"`
}

// DefaultClientOptions 默认集群客户端配置
func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		EnableInformer:        true,             // 启用 Informer
		ResyncPeriod:          10 * time.Minute, // 集群重新同步周期 10 分钟
		QPS:                   100,              // QPS 限制 100
		Burst:                 300,              // 突发请求数 100
		Timeout:               10 * time.Second, // 请求超时 10 秒
		MaxConcurrentRequests: 500,              // 最大并发请求数 100
	}
}

// validateConfig 验证配置
func validateConfig(config *Config) error {
	logx.Infof("验证集群配置文件... %v", config)
	if config.ID == "" {
		return fmt.Errorf("集群 ID 不能为空")
	}

	switch config.AuthType {
	case AuthTypeKubeConfig:
		if config.KubeConfigData == "" {
			return fmt.Errorf("kubeconfig 不能为空。")
		}
	case AuthTypeToken:
		if config.APIServer == "" || config.Token == "" {
			return fmt.Errorf("API 服务器地址和 token 不能为空")
		}
	case AuthTypeCertificate:
		if config.APIServer == "" || config.CertFile == "" || config.KeyFile == "" {
			return fmt.Errorf("API 服务器地址和证书文件不能为空")
		}
	case AuthTypeInCluster:
		// 集群内认证不需要额外参数
	default:
		return fmt.Errorf("不支持的认证类型: %s", config.AuthType)
	}

	// 设置默认值（现在可以正确修改原始 config）
	defaults := DefaultClientOptions()

	if config.Options.QPS <= 0 {
		config.Options.QPS = defaults.QPS
	}
	if config.Options.Burst <= 0 {
		config.Options.Burst = defaults.Burst
	}
	if config.Options.ResyncPeriod == 0 {
		config.Options.ResyncPeriod = defaults.ResyncPeriod
	}
	if config.Options.Timeout == 0 {
		config.Options.Timeout = defaults.Timeout
	}
	if config.Options.MaxConcurrentRequests <= 0 {
		config.Options.MaxConcurrentRequests = defaults.MaxConcurrentRequests
	}

	return nil
}
