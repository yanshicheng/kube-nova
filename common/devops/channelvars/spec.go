package channelvars

import "encoding/json"

// VariableSpec 渠道变量规格
type VariableSpec struct {
	ID               string          `json:"id"`
	SpecProfile      string          `json:"specProfile"`      // 规格模板
	FieldKey         string          `json:"fieldKey"`         // 字段key
	FieldName        string          `json:"fieldName"`        // 字段中文名
	FieldKind        string          `json:"fieldKind"`        // 字段类型: endpoint/dynamic/address/credential/option/output
	ProviderKey      string          `json:"providerKey"`      // Provider key
	UIControl        string          `json:"uiControl"`        // UI控件类型
	Dependencies     *DependencyRule `json:"dependencies"`     // 依赖规则
	AllowManualInput bool            `json:"allowManualInput"` // 是否允许手填
	IsRequired       bool            `json:"isRequired"`       // 是否必填
	DefaultValue     string          `json:"defaultValue"`     // 默认值
	Placeholder      string          `json:"placeholder"`      // 占位符
	HelpText         string          `json:"helpText"`         // 帮助文本
	SortOrder        int             `json:"sortOrder"`        // 排序
}

// DependencyRule 依赖规则
type DependencyRule struct {
	DataSource  *DataSourceDep  `json:"dataSource"`  // 数据源依赖
	QueryParams *QueryParamsDep `json:"queryParams"` // 查询参数依赖
}

// DataSourceDep 数据源依赖
type DataSourceDep struct {
	Mode    string             `json:"mode"`    // allOf/anyOf
	Options []DataSourceOption `json:"options"` // 数据源选项
}

// DataSourceOption 数据源选项
type DataSourceOption struct {
	Type            string   `json:"type"`            // endpoint/address
	Fields          []string `json:"fields"`          // 依赖字段
	ResolveEndpoint bool     `json:"resolveEndpoint"` // 是否需要从address反查endpoint
	FallbackAllowed bool     `json:"fallbackAllowed"` // 反查失败是否允许降级
}

// QueryParamsDep 查询参数依赖
type QueryParamsDep struct {
	Mode   string   `json:"mode"`   // allOf/anyOf
	Fields []string `json:"fields"` // 依赖字段
}

// Option 选项
type Option struct {
	Label    string                 `json:"label"`
	Value    string                 `json:"value"`
	Disabled bool                   `json:"disabled,omitempty"`
	Children []Option               `json:"children,omitempty"`
	Extra    map[string]interface{} `json:"extra,omitempty"` // 额外信息
}

// FieldKind 字段类型常量
const (
	FieldKindEndpoint   = "endpoint"
	FieldKindDynamic    = "dynamic"
	FieldKindAddress    = "address"
	FieldKindCredential = "credential"
	FieldKindOption     = "option"
	FieldKindOutput     = "output"
)

// UIControl UI控件类型常量
const (
	UIControlSelect   = "select"
	UIControlInput    = "input"
	UIControlCascader = "cascader"
	UIControlRadio    = "radio"
	UIControlReadonly = "readonly"
	UIControlOutput   = "output"
	UIControlTextarea = "textarea"
)

// DependencyMode 依赖模式常量
const (
	DependencyModeAllOf = "allOf"
	DependencyModeAnyOf = "anyOf"
)

// ParseDependencies 解析依赖规则JSON
func ParseDependencies(jsonStr string) (*DependencyRule, error) {
	if jsonStr == "" {
		return nil, nil
	}

	var rule DependencyRule
	if err := json.Unmarshal([]byte(jsonStr), &rule); err != nil {
		return nil, err
	}

	return &rule, nil
}

// MarshalDependencies 序列化依赖规则为JSON
func MarshalDependencies(rule *DependencyRule) (string, error) {
	if rule == nil {
		return "", nil
	}

	data, err := json.Marshal(rule)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
