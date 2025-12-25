package types

import (
	flaggerv1 "github.com/fluxcd/flagger/pkg/apis/flagger/v1beta1"
)

// CanaryInfo Canary 信息
type CanaryInfo struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	TargetRef         TargetRefInfo     `json:"targetRef"`        // 目标资源引用
	ProgressDeadline  int32             `json:"progressDeadline"` // 进度截止时间（秒）
	Status            string            `json:"status"`           // 状态: Initialized, Progressing, Promoting, Finalising, Succeeded, Failed
	CanaryWeight      int               `json:"canaryWeight"`     // 当前金丝雀权重
	FailedChecks      int               `json:"failedChecks"`     // 失败检查次数
	Phase             string            `json:"phase"`            // 阶段: Initializing, Waiting, Progressing, Promoting, Finalising, Succeeded, Failed
	LastTransition    string            `json:"lastTransition"`   // 最后状态变更时间
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Age               string            `json:"age"`               // 如: "2d5h"
	CreationTimestamp int64             `json:"creationTimestamp"` // 时间戳（毫秒）
}

// TargetRefInfo 目标资源引用信息
type TargetRefInfo struct {
	APIVersion string `json:"apiVersion"` // apps/v1
	Kind       string `json:"kind"`       // Deployment, DaemonSet
	Name       string `json:"name"`       // 资源名称
}

// CanaryAnalysis 金丝雀分析配置
type CanaryAnalysis struct {
	Interval   string        `json:"interval"`   // 分析间隔，如: "1m"
	Threshold  int32         `json:"threshold"`  // 失败阈值
	MaxWeight  int32         `json:"maxWeight"`  // 最大权重
	StepWeight int32         `json:"stepWeight"` // 步进权重
	Metrics    []MetricInfo  `json:"metrics"`    // 指标列表
	Webhooks   []WebhookInfo `json:"webhooks"`   // Webhook 列表
	Match      []MatchInfo   `json:"match"`      // 流量匹配规则
	Iterations int32         `json:"iterations"` // 迭代次数
}

// MetricInfo 指标信息
type MetricInfo struct {
	Name           string          `json:"name"`                     // 指标名称
	Interval       string          `json:"interval,omitempty"`       // 指标间隔
	ThresholdRange *ThresholdRange `json:"thresholdRange,omitempty"` // 阈值范围
	Query          string          `json:"query,omitempty"`          // 查询语句
	TemplateRef    *TemplateRef    `json:"templateRef,omitempty"`    // 模板引用
}

// ThresholdRange 阈值范围
type ThresholdRange struct {
	Min *float64 `json:"min,omitempty"` // 最小值
	Max *float64 `json:"max,omitempty"` // 最大值
}

// TemplateRef 模板引用
type TemplateRef struct {
	Name      string `json:"name"`      // 模板名称
	Namespace string `json:"namespace"` // 模板命名空间
}

// WebhookInfo Webhook 信息
type WebhookInfo struct {
	Name     string            `json:"name"`               // Webhook 名称
	Type     string            `json:"type"`               // 类型: pre-rollout, rollout, confirm-rollout, post-rollout, rollback
	URL      string            `json:"url"`                // URL 地址
	Timeout  string            `json:"timeout,omitempty"`  // 超时时间
	Metadata map[string]string `json:"metadata,omitempty"` // 元数据
}

// MatchInfo 流量匹配信息
type MatchInfo struct {
	Headers map[string]StringMatch `json:"headers,omitempty"` // HTTP 头匹配
}

// StringMatch 字符串匹配
type StringMatch struct {
	Exact  string `json:"exact,omitempty"`  // 精确匹配
	Prefix string `json:"prefix,omitempty"` // 前缀匹配
	Suffix string `json:"suffix,omitempty"` // 后缀匹配
	Regex  string `json:"regex,omitempty"`  // 正则匹配
}

// CanaryService 金丝雀服务配置
type CanaryService struct {
	Port          int32          `json:"port"`                    // 端口
	TargetPort    int32          `json:"targetPort,omitempty"`    // 目标端口
	Name          string         `json:"name,omitempty"`          // 服务名称
	PortName      string         `json:"portName,omitempty"`      // 端口名称
	Gateways      []string       `json:"gateways,omitempty"`      // Istio Gateway 列表
	Hosts         []string       `json:"hosts,omitempty"`         // 主机列表
	TrafficPolicy *TrafficPolicy `json:"trafficPolicy,omitempty"` // 流量策略
}

// TrafficPolicy 流量策略
type TrafficPolicy struct {
	TLS *TLSPolicy `json:"tls,omitempty"` // TLS 配置
}

// TLSPolicy TLS 策略
type TLSPolicy struct {
	Mode string `json:"mode"` // DISABLE, SIMPLE, MUTUAL, ISTIO_MUTUAL
}

// CanaryDetail Canary 详细信息
type CanaryDetail struct {
	CanaryInfo
	Analysis CanaryAnalysis `json:"analysis"` // 分析配置
	Service  CanaryService  `json:"service"`  // 服务配置
}

// ListCanaryResponse Canary 列表响应
type ListCanaryResponse struct {
	Total int          `json:"total"`
	Items []CanaryInfo `json:"items"`
}

// CanaryStatusCondition Canary 状态条件
type CanaryStatusCondition struct {
	Type               string `json:"type"`               // 类型: Promoted, Progressing
	Status             string `json:"status"`             // 状态: True, False, Unknown
	LastUpdateTime     string `json:"lastUpdateTime"`     // 最后更新时间
	LastTransitionTime string `json:"lastTransitionTime"` // 最后变更时间
	Reason             string `json:"reason"`             // 原因
	Message            string `json:"message"`            // 消息
}

// CanaryStatusResponse Canary 状态响应
type CanaryStatusResponse struct {
	Name            string                  `json:"name"`
	Namespace       string                  `json:"namespace"`
	Phase           string                  `json:"phase"`           // 当前阶段
	CanaryWeight    int                     `json:"canaryWeight"`    // 金丝雀权重
	FailedChecks    int                     `json:"failedChecks"`    // 失败检查次数
	Iterations      int                     `json:"iterations"`      // 迭代次数
	Conditions      []CanaryStatusCondition `json:"conditions"`      // 状态条件
	TrackedConfigs  map[string]string       `json:"trackedConfigs"`  // 跟踪的配置
	LastAppliedSpec string                  `json:"lastAppliedSpec"` // 最后应用的规格
}

// FlaggerOperator Flagger Canary 操作器接口
type FlaggerOperator interface {
	// ========== 基础 CRUD 操作 ==========
	Create(canary *flaggerv1.Canary) (*flaggerv1.Canary, error)
	Get(namespace, name string) (*flaggerv1.Canary, error)
	Update(canary *flaggerv1.Canary) (*flaggerv1.Canary, error)
	Delete(namespace, name string) error
	List(namespace string, search string, labelSelector string) (*ListCanaryResponse, error)

	// ========== YAML 操作 ==========
	GetYaml(namespace, name string) (string, error) // 获取 YAML

	// ========== 详情和状态 ==========
	GetDetail(namespace, name string) (*CanaryDetail, error)         // 获取详细信息
	GetStatus(namespace, name string) (*CanaryStatusResponse, error) // 获取状态信息
	GetByAutoscalerRef(namespace string, apiVersion string, kind string, name string) (*flaggerv1.Canary, error)
	GetByTargetRef(namespace string, apiVersion string, kind string, name string) (*flaggerv1.Canary, error)
	// ========== 金丝雀控制操作 ==========
	// 暂停金丝雀发布
	Pause(namespace, name string) error
	// 恢复金丝雀发布
	Resume(namespace, name string) error
	// 重置金丝雀状态
	Reset(namespace, name string) error
}
