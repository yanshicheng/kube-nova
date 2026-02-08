package cluster

import (
	"fmt"
	"sort"
	"strings"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
)

// ==================== 时间处理 ====================

// formatAge 格式化时间为年龄字符串
func formatAge(t time.Time) string {
	duration := time.Since(t)
	if duration.Hours() >= 24 {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	}
	if duration.Hours() >= 1 {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	if duration.Minutes() >= 1 {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	return fmt.Sprintf("%ds", int(duration.Seconds()))
}

// ==================== Map 对比工具 ====================

// MapDiffResult Map 对比结果
type MapDiffResult struct {
	Added    map[string]string // 新增的键值对
	Modified map[string]struct {
		Old string
		New string
	} // 修改的键值对
	Deleted map[string]string // 删除的键值对
}

// CompareStringMaps 对比两个 string map 的差异
func CompareStringMaps(oldMap, newMap map[string]string) *MapDiffResult {
	result := &MapDiffResult{
		Added: make(map[string]string),
		Modified: make(map[string]struct {
			Old string
			New string
		}),
		Deleted: make(map[string]string),
	}

	if oldMap == nil {
		oldMap = make(map[string]string)
	}
	if newMap == nil {
		newMap = make(map[string]string)
	}

	// 检查新增和修改
	for key, newVal := range newMap {
		if oldVal, exists := oldMap[key]; exists {
			if oldVal != newVal {
				result.Modified[key] = struct {
					Old string
					New string
				}{Old: oldVal, New: newVal}
			}
		} else {
			result.Added[key] = newVal
		}
	}

	// 检查删除
	for key, oldVal := range oldMap {
		if _, exists := newMap[key]; !exists {
			result.Deleted[key] = oldVal
		}
	}

	return result
}

// BuildMapDiffDetail 构建 Map 变更详情字符串
func BuildMapDiffDetail(diff *MapDiffResult, recordValues bool) string {
	if diff == nil {
		return "无变更"
	}

	var parts []string

	// 新增
	if len(diff.Added) > 0 {
		keys := getSortedKeys(diff.Added)
		if recordValues {
			var items []string
			for _, k := range keys {
				items = append(items, fmt.Sprintf("%s=%s", k, truncateValue(diff.Added[k], 50)))
			}
			parts = append(parts, fmt.Sprintf("新增 [%s]", strings.Join(items, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("新增 [%s]", strings.Join(keys, ", ")))
		}
	}

	// 修改
	if len(diff.Modified) > 0 {
		var modKeys []string
		for k := range diff.Modified {
			modKeys = append(modKeys, k)
		}
		sort.Strings(modKeys)

		if recordValues {
			var items []string
			for _, k := range modKeys {
				v := diff.Modified[k]
				items = append(items, fmt.Sprintf("%s: %s → %s", k, truncateValue(v.Old, 30), truncateValue(v.New, 30)))
			}
			parts = append(parts, fmt.Sprintf("修改 [%s]", strings.Join(items, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("修改 [%s]", strings.Join(modKeys, ", ")))
		}
	}

	// 删除
	if len(diff.Deleted) > 0 {
		keys := getSortedKeys(diff.Deleted)
		if recordValues {
			var items []string
			for _, k := range keys {
				items = append(items, fmt.Sprintf("%s=%s", k, truncateValue(diff.Deleted[k], 50)))
			}
			parts = append(parts, fmt.Sprintf("删除 [%s]", strings.Join(items, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("删除 [%s]", strings.Join(keys, ", ")))
		}
	}

	if len(parts) == 0 {
		return "无变更"
	}

	return strings.Join(parts, ", ")
}

// HasMapChanges 检查是否有变更
func HasMapChanges(diff *MapDiffResult) bool {
	if diff == nil {
		return false
	}
	return len(diff.Added) > 0 || len(diff.Modified) > 0 || len(diff.Deleted) > 0
}

// ==================== 字符串切片对比 ====================

// CompareStringSlices 对比字符串切片
func CompareStringSlices(oldSlice, newSlice []string) (added, deleted []string) {
	oldMap := make(map[string]bool)
	newMap := make(map[string]bool)

	for _, s := range oldSlice {
		oldMap[s] = true
	}
	for _, s := range newSlice {
		newMap[s] = true
	}

	for s := range newMap {
		if !oldMap[s] {
			added = append(added, s)
		}
	}
	for s := range oldMap {
		if !newMap[s] {
			deleted = append(deleted, s)
		}
	}

	return
}

// BuildSliceDiffDetail 构建切片变更详情
func BuildSliceDiffDetail(itemName string, added, deleted []string) string {
	var parts []string
	if len(added) > 0 {
		parts = append(parts, fmt.Sprintf("新增%s [%s]", itemName, strings.Join(added, ", ")))
	}
	if len(deleted) > 0 {
		parts = append(parts, fmt.Sprintf("删除%s [%s]", itemName, strings.Join(deleted, ", ")))
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

// ==================== ClusterRole 规则对比 ====================

// PolicyRuleDiffResult PolicyRule 对比结果
type PolicyRuleDiffResult struct {
	Added    []string // 新增的规则描述
	Modified []string // 修改的规则描述
	Deleted  []string // 删除的规则描述
}

// FormatPolicyRule 格式化单条规则为可读字符串
func FormatPolicyRule(rule rbacv1.PolicyRule) string {
	var parts []string

	if len(rule.APIGroups) > 0 {
		apiGroups := rule.APIGroups
		for i, g := range apiGroups {
			if g == "" {
				apiGroups[i] = "core"
			}
		}
		parts = append(parts, fmt.Sprintf("apiGroups=[%s]", strings.Join(apiGroups, ",")))
	}

	if len(rule.Resources) > 0 {
		parts = append(parts, fmt.Sprintf("resources=[%s]", strings.Join(rule.Resources, ",")))
	}

	if len(rule.Verbs) > 0 {
		parts = append(parts, fmt.Sprintf("verbs=[%s]", strings.Join(rule.Verbs, ",")))
	}

	if len(rule.ResourceNames) > 0 {
		parts = append(parts, fmt.Sprintf("resourceNames=[%s]", strings.Join(rule.ResourceNames, ",")))
	}

	if len(rule.NonResourceURLs) > 0 {
		parts = append(parts, fmt.Sprintf("nonResourceURLs=[%s]", strings.Join(rule.NonResourceURLs, ",")))
	}

	return strings.Join(parts, ", ")
}

// FormatPolicyRulesShort 格式化规则列表为简短描述
func FormatPolicyRulesShort(rules []rbacv1.PolicyRule) string {
	if len(rules) == 0 {
		return "无规则"
	}

	var summaries []string
	for _, rule := range rules {
		resources := strings.Join(rule.Resources, ",")
		verbs := strings.Join(rule.Verbs, ",")
		if resources == "" && len(rule.NonResourceURLs) > 0 {
			resources = strings.Join(rule.NonResourceURLs, ",")
		}
		summaries = append(summaries, fmt.Sprintf("%s(%s)", resources, verbs))
	}

	result := strings.Join(summaries, "; ")
	if len(result) > 200 {
		return result[:200] + "..."
	}
	return result
}

// ComparePolicyRules 对比 PolicyRule 变更
func ComparePolicyRules(oldRules, newRules []rbacv1.PolicyRule) *PolicyRuleDiffResult {
	result := &PolicyRuleDiffResult{}

	// 将规则转换为字符串用于对比
	oldRuleStrs := make(map[string]bool)
	newRuleStrs := make(map[string]bool)

	for _, rule := range oldRules {
		oldRuleStrs[FormatPolicyRule(rule)] = true
	}
	for _, rule := range newRules {
		newRuleStrs[FormatPolicyRule(rule)] = true
	}

	// 检查新增
	for ruleStr := range newRuleStrs {
		if !oldRuleStrs[ruleStr] {
			result.Added = append(result.Added, ruleStr)
		}
	}

	// 检查删除
	for ruleStr := range oldRuleStrs {
		if !newRuleStrs[ruleStr] {
			result.Deleted = append(result.Deleted, ruleStr)
		}
	}

	return result
}

// BuildPolicyRuleDiffDetail 构建规则变更详情
func BuildPolicyRuleDiffDetail(diff *PolicyRuleDiffResult) string {
	if diff == nil {
		return "规则无变更"
	}

	var parts []string
	if len(diff.Added) > 0 {
		parts = append(parts, fmt.Sprintf("新增规则: %s", strings.Join(diff.Added, "; ")))
	}
	if len(diff.Deleted) > 0 {
		parts = append(parts, fmt.Sprintf("删除规则: %s", strings.Join(diff.Deleted, "; ")))
	}

	if len(parts) == 0 {
		return "规则无变更"
	}

	result := strings.Join(parts, ", ")
	if len(result) > 500 {
		return result[:500] + "..."
	}
	return result
}

// HasPolicyRuleChanges 检查规则是否有变更
func HasPolicyRuleChanges(diff *PolicyRuleDiffResult) bool {
	if diff == nil {
		return false
	}
	return len(diff.Added) > 0 || len(diff.Deleted) > 0
}

// ==================== Subject 主体对比 ====================

// SubjectDiffResult 主体对比结果
type SubjectDiffResult struct {
	Added   []string
	Deleted []string
}

// FormatSubject 格式化主体为可读字符串
func FormatSubject(subject rbacv1.Subject) string {
	if subject.Namespace != "" {
		return fmt.Sprintf("%s/%s/%s", subject.Kind, subject.Namespace, subject.Name)
	}
	return fmt.Sprintf("%s/%s", subject.Kind, subject.Name)
}

// CompareSubjects 对比主体变更
func CompareSubjects(oldSubjects, newSubjects []rbacv1.Subject) *SubjectDiffResult {
	result := &SubjectDiffResult{}

	oldMap := make(map[string]bool)
	newMap := make(map[string]bool)

	for _, s := range oldSubjects {
		oldMap[FormatSubject(s)] = true
	}
	for _, s := range newSubjects {
		newMap[FormatSubject(s)] = true
	}

	for key := range newMap {
		if !oldMap[key] {
			result.Added = append(result.Added, key)
		}
	}

	for key := range oldMap {
		if !newMap[key] {
			result.Deleted = append(result.Deleted, key)
		}
	}

	return result
}

// BuildSubjectDiffDetail 构建主体变更详情
func BuildSubjectDiffDetail(diff *SubjectDiffResult) string {
	if diff == nil {
		return "主体无变更"
	}

	var parts []string
	if len(diff.Added) > 0 {
		parts = append(parts, fmt.Sprintf("新增主体 [%s]", strings.Join(diff.Added, ", ")))
	}
	if len(diff.Deleted) > 0 {
		parts = append(parts, fmt.Sprintf("删除主体 [%s]", strings.Join(diff.Deleted, ", ")))
	}

	if len(parts) == 0 {
		return "主体无变更"
	}
	return strings.Join(parts, ", ")
}

// HasSubjectChanges 检查主体是否有变更
func HasSubjectChanges(diff *SubjectDiffResult) bool {
	if diff == nil {
		return false
	}
	return len(diff.Added) > 0 || len(diff.Deleted) > 0
}

// FormatSubjectsList 格式化主体列表为简短描述
func FormatSubjectsList(subjects []rbacv1.Subject) string {
	if len(subjects) == 0 {
		return "无主体"
	}

	var users, groups, serviceAccounts []string
	for _, s := range subjects {
		switch s.Kind {
		case "User":
			users = append(users, s.Name)
		case "Group":
			groups = append(groups, s.Name)
		case "ServiceAccount":
			if s.Namespace != "" {
				serviceAccounts = append(serviceAccounts, fmt.Sprintf("%s/%s", s.Namespace, s.Name))
			} else {
				serviceAccounts = append(serviceAccounts, s.Name)
			}
		}
	}

	var parts []string
	if len(users) > 0 {
		parts = append(parts, fmt.Sprintf("用户: %s", strings.Join(users, ",")))
	}
	if len(groups) > 0 {
		parts = append(parts, fmt.Sprintf("组: %s", strings.Join(groups, ",")))
	}
	if len(serviceAccounts) > 0 {
		parts = append(parts, fmt.Sprintf("ServiceAccount: %s", strings.Join(serviceAccounts, ",")))
	}

	if len(parts) == 0 {
		return "无主体"
	}
	return strings.Join(parts, ", ")
}

// ==================== StorageClass 相关 ====================

// FormatStorageClassConfig 格式化 StorageClass 配置
func FormatStorageClassConfig(provisioner, reclaimPolicy, volumeBindingMode string, allowExpansion bool, parameters map[string]string) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Provisioner: %s", provisioner))
	parts = append(parts, fmt.Sprintf("回收策略: %s", reclaimPolicy))
	parts = append(parts, fmt.Sprintf("绑定模式: %s", volumeBindingMode))

	if allowExpansion {
		parts = append(parts, "允许扩展: 是")
	} else {
		parts = append(parts, "允许扩展: 否")
	}

	if len(parameters) > 0 {
		var paramParts []string
		for k, v := range parameters {
			paramParts = append(paramParts, fmt.Sprintf("%s=%s", k, truncateValue(v, 30)))
		}
		sort.Strings(paramParts)
		parts = append(parts, fmt.Sprintf("参数: [%s]", strings.Join(paramParts, ", ")))
	}

	return strings.Join(parts, ", ")
}

// ==================== IngressClass 相关 ====================

// FormatIngressClassConfig 格式化 IngressClass 配置
func FormatIngressClassConfig(controller string, isDefault bool, hasParams bool) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("控制器: %s", controller))

	if isDefault {
		parts = append(parts, "默认: 是")
	}

	if hasParams {
		parts = append(parts, "有参数配置")
	}

	return strings.Join(parts, ", ")
}

// ==================== PV 相关 ====================

// FormatPVConfig 格式化 PV 配置
func FormatPVConfig(capacity, storageClass, reclaimPolicy string, accessModes []string) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("容量: %s", capacity))

	if storageClass != "" {
		parts = append(parts, fmt.Sprintf("StorageClass: %s", storageClass))
	}

	parts = append(parts, fmt.Sprintf("回收策略: %s", reclaimPolicy))

	if len(accessModes) > 0 {
		parts = append(parts, fmt.Sprintf("访问模式: [%s]", strings.Join(accessModes, ", ")))
	}

	return strings.Join(parts, ", ")
}

// ==================== 通用工具函数 ====================

// truncateValue 截断过长的值
func truncateValue(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// getSortedKeys 获取排序后的 map keys
func getSortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// joinNonEmpty 连接非空字符串
func joinNonEmpty(sep string, parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, sep)
}
