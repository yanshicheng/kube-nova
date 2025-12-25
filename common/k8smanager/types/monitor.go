package types

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

// ==================== Probe Types ====================

// ProbeInfo Probe 信息
type ProbeInfo struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Age               string            `json:"age"` // 如: "2d5h"
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	CreationTimestamp int64             `json:"creationTimestamp"` // 时间戳（毫秒）
}

// ListProbeResponse Probe 列表响应
type ListProbeResponse struct {
	Total int         `json:"total"`
	Items []ProbeInfo `json:"items"`
}

// ProbeOperator Probe 操作器接口
type ProbeOperator interface {
	// Create 创建 Probe
	Create(probe *monitoringv1.Probe) error

	// Get 获取 Probe 的 YAML
	Get(namespace, name string) (string, error)

	// List 获取 Probe 列表，支持搜索
	List(namespace string, search string) (*ListProbeResponse, error)

	// Update 更新 Probe
	Update(namespace, name string, probe *monitoringv1.Probe) error

	// Delete 删除 Probe
	Delete(namespace, name string) error
}

// ==================== PrometheusRule Types ====================

// PrometheusRuleInfo PrometheusRule 信息
type PrometheusRuleInfo struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Age               string            `json:"age"` // 如: "2d5h"
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	CreationTimestamp int64             `json:"creationTimestamp"` // 时间戳（毫秒）
}

// ListPrometheusRuleResponse PrometheusRule 列表响应
type ListPrometheusRuleResponse struct {
	Total int                  `json:"total"`
	Items []PrometheusRuleInfo `json:"items"`
}

// PrometheusRuleOperator PrometheusRule 操作器接口
type PrometheusRuleOperator interface {
	// Create 创建 PrometheusRule
	Create(rule *monitoringv1.PrometheusRule) error

	// Get 获取 PrometheusRule 的 YAML
	Get(namespace, name string) (string, error)

	// List 获取 PrometheusRule 列表，支持搜索
	List(namespace string, search string) (*ListPrometheusRuleResponse, error)

	// Update 更新 PrometheusRule
	Update(namespace, name string, rule *monitoringv1.PrometheusRule) error

	// Delete 删除 PrometheusRule
	Delete(namespace, name string) error
}
