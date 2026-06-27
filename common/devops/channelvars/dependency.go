package channelvars

import (
	"fmt"
)

// DependencyChecker 依赖检查器
type DependencyChecker struct {
	specs  []VariableSpec
	values map[string]interface{}
}

// NewDependencyChecker 创建依赖检查器
func NewDependencyChecker(specs []VariableSpec, values map[string]interface{}) *DependencyChecker {
	return &DependencyChecker{
		specs:  specs,
		values: values,
	}
}

// CheckResult 检查结果
type CheckResult struct {
	Satisfied bool   // 依赖是否满足
	Visible   bool   // 字段是否可见
	Reason    string // 不满足的原因
}

// Check 检查字段依赖
func (c *DependencyChecker) Check(fieldKey string) CheckResult {
	spec := c.findSpec(fieldKey)
	if spec == nil {
		return CheckResult{
			Satisfied: false,
			Visible:   false,
			Reason:    fmt.Sprintf("spec not found for field: %s", fieldKey),
		}
	}

	if spec.Dependencies == nil {
		return CheckResult{
			Satisfied: true,
			Visible:   true,
		}
	}

	// 检查数据源依赖
	if spec.Dependencies.DataSource != nil {
		result := c.checkDataSource(spec.Dependencies.DataSource)
		if !result.Satisfied {
			return result
		}
	}

	// 检查查询参数依赖
	if spec.Dependencies.QueryParams != nil {
		result := c.checkQueryParams(spec.Dependencies.QueryParams)
		if !result.Satisfied {
			return result
		}
	}

	return CheckResult{
		Satisfied: true,
		Visible:   true,
	}
}

// checkDataSource 检查数据源依赖
func (c *DependencyChecker) checkDataSource(dep *DataSourceDep) CheckResult {
	if dep.Mode == DependencyModeAllOf {
		// 所有选项都必须满足
		for _, opt := range dep.Options {
			if !c.checkDataSourceOption(opt) {
				return CheckResult{
					Satisfied: false,
					Visible:   true,
					Reason:    fmt.Sprintf("data source not satisfied: %v", opt.Fields),
				}
			}
		}
		return CheckResult{Satisfied: true, Visible: true}
	}

	// anyOf: 满足任意一个即可
	for _, opt := range dep.Options {
		if c.checkDataSourceOption(opt) {
			return CheckResult{Satisfied: true, Visible: true}
		}
	}

	return CheckResult{
		Satisfied: false,
		Visible:   true,
		Reason:    "no data source option satisfied",
	}
}

// checkDataSourceOption 检查数据源选项
func (c *DependencyChecker) checkDataSourceOption(opt DataSourceOption) bool {
	for _, field := range opt.Fields {
		val, ok := c.values[field]
		if !ok || c.isEmpty(val) {
			return false
		}
	}
	return true
}

// checkQueryParams 检查查询参数依赖
func (c *DependencyChecker) checkQueryParams(dep *QueryParamsDep) CheckResult {
	if dep.Mode == DependencyModeAllOf {
		// 所有字段都必须有值
		for _, field := range dep.Fields {
			val, ok := c.values[field]
			if !ok || c.isEmpty(val) {
				return CheckResult{
					Satisfied: false,
					Visible:   true,
					Reason:    fmt.Sprintf("query param not satisfied: %s", field),
				}
			}
		}
		return CheckResult{Satisfied: true, Visible: true}
	}

	// anyOf: 满足任意一个即可
	for _, field := range dep.Fields {
		val, ok := c.values[field]
		if ok && !c.isEmpty(val) {
			return CheckResult{Satisfied: true, Visible: true}
		}
	}

	return CheckResult{
		Satisfied: false,
		Visible:   true,
		Reason:    "no query param satisfied",
	}
}

// isEmpty 判断值是否为空
func (c *DependencyChecker) isEmpty(val interface{}) bool {
	if val == nil {
		return true
	}

	switch v := val.(type) {
	case string:
		return v == ""
	case int, int64:
		return v == 0
	case float64:
		return v == 0
	default:
		return false
	}
}

// findSpec 查找规格
func (c *DependencyChecker) findSpec(fieldKey string) *VariableSpec {
	for _, spec := range c.specs {
		if spec.FieldKey == fieldKey {
			return &spec
		}
	}
	return nil
}

// GetDownstreamFields 获取依赖某个字段的下游字段
func (c *DependencyChecker) GetDownstreamFields(fieldKey string) []string {
	downstream := make([]string, 0)

	for _, spec := range c.specs {
		if c.isDependsOn(spec, fieldKey) {
			downstream = append(downstream, spec.FieldKey)
		}
	}

	return downstream
}

// isDependsOn 判断spec是否依赖某个字段
func (c *DependencyChecker) isDependsOn(spec VariableSpec, fieldKey string) bool {
	if spec.Dependencies == nil {
		return false
	}

	// 检查数据源依赖
	if spec.Dependencies.DataSource != nil {
		for _, opt := range spec.Dependencies.DataSource.Options {
			for _, field := range opt.Fields {
				if field == fieldKey {
					return true
				}
			}
		}
	}

	// 检查查询参数依赖
	if spec.Dependencies.QueryParams != nil {
		for _, field := range spec.Dependencies.QueryParams.Fields {
			if field == fieldKey {
				return true
			}
		}
	}

	return false
}

// ExtractParentValues 提取父级依赖值
func (c *DependencyChecker) ExtractParentValues(spec VariableSpec) map[string]string {
	result := make(map[string]string)

	if spec.Dependencies == nil {
		return result
	}

	// 提取数据源字段
	if spec.Dependencies.DataSource != nil {
		for _, opt := range spec.Dependencies.DataSource.Options {
			for _, field := range opt.Fields {
				if val, ok := c.values[field]; ok {
					result[field] = fmt.Sprintf("%v", val)
				}
			}
		}
	}

	// 提取查询参数字段
	if spec.Dependencies.QueryParams != nil {
		for _, field := range spec.Dependencies.QueryParams.Fields {
			if val, ok := c.values[field]; ok {
				result[field] = fmt.Sprintf("%v", val)
			}
		}
	}

	return result
}
