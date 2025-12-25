package utils

import (
	"k8s.io/client-go/rest"
)

// NewRestConfig 封装函数，用于返回 Kubernetes REST 配置
func NewRestConfig(ipaddr, token string, insecure bool) *rest.Config {

	config := &rest.Config{
		Host:        ipaddr,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: insecure,
		},
	}

	return config
}
