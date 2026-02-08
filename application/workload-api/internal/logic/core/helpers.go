// logic/core/helpers.go
// 辅助函数，供所有 Logic 使用

package core

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8smanagertypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"
	networkingv1 "k8s.io/api/networking/v1"
)

// ==================== 审计详情配置 ====================

// AuditDetailConfig 审计详情配置
var AuditDetailConfig = struct {
	// RecordConfigMapValues 是否记录 ConfigMap 的具体值变更（true: 记录 key=value, false: 只记录 key）
	RecordConfigMapValues bool
	// RecordSecretValues 是否记录 Secret 的具体值变更（true: 记录 key=value, false: 只记录 key）
	RecordSecretValues bool
}{
	RecordConfigMapValues: false,
	RecordSecretValues:    false,
}

// ==================== 审计详情构建工具 ====================

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

// CompareByteMaps 对比两个 byte map 的差异（用于 Secret）
func CompareByteMaps(oldMap, newMap map[string][]byte) *MapDiffResult {
	oldStrMap := make(map[string]string)
	newStrMap := make(map[string]string)

	for k, v := range oldMap {
		oldStrMap[k] = string(v)
	}
	for k, v := range newMap {
		newStrMap[k] = string(v)
	}

	return CompareStringMaps(oldStrMap, newStrMap)
}

// BuildMapDiffDetail 构建 Map 变更详情字符串
// recordValues: true 记录具体值, false 只记录 key
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

// ==================== Service 端口对比 ====================

// PortDiffResult 端口对比结果
type PortDiffResult struct {
	Added    []string // 新增的端口描述
	Modified []string // 修改的端口描述
	Deleted  []string // 删除的端口描述
}

// CompareServicePorts 对比 Service 端口变更
func CompareServicePorts(oldPorts, newPorts []types.ServicePort) *PortDiffResult {
	result := &PortDiffResult{}

	oldPortMap := make(map[string]types.ServicePort)
	for _, p := range oldPorts {
		key := fmt.Sprintf("%s-%d", p.Name, p.Port)
		if p.Name == "" {
			key = fmt.Sprintf("%d", p.Port)
		}
		oldPortMap[key] = p
	}

	newPortMap := make(map[string]types.ServicePort)
	for _, p := range newPorts {
		key := fmt.Sprintf("%s-%d", p.Name, p.Port)
		if p.Name == "" {
			key = fmt.Sprintf("%d", p.Port)
		}
		newPortMap[key] = p
	}

	// 检查新增和修改
	for key, newPort := range newPortMap {
		if oldPort, exists := oldPortMap[key]; exists {
			// 检查是否有修改
			if oldPort.TargetPort != newPort.TargetPort ||
				oldPort.NodePort != newPort.NodePort ||
				oldPort.Protocol != newPort.Protocol {
				result.Modified = append(result.Modified,
					fmt.Sprintf("%s: %d→%s 改为 %d→%s",
						getPortName(newPort), oldPort.Port, oldPort.TargetPort,
						newPort.Port, newPort.TargetPort))
			}
		} else {
			result.Added = append(result.Added,
				fmt.Sprintf("%s:%d→%s", getPortName(newPort), newPort.Port, newPort.TargetPort))
		}
	}

	// 检查删除
	for key, oldPort := range oldPortMap {
		if _, exists := newPortMap[key]; !exists {
			result.Deleted = append(result.Deleted,
				fmt.Sprintf("%s:%d→%s", getPortName(oldPort), oldPort.Port, oldPort.TargetPort))
		}
	}

	return result
}

// BuildPortDiffDetail 构建端口变更详情
func BuildPortDiffDetail(diff *PortDiffResult) string {
	if diff == nil {
		return ""
	}

	var parts []string
	if len(diff.Added) > 0 {
		parts = append(parts, fmt.Sprintf("新增端口 [%s]", strings.Join(diff.Added, ", ")))
	}
	if len(diff.Modified) > 0 {
		parts = append(parts, fmt.Sprintf("修改端口 [%s]", strings.Join(diff.Modified, ", ")))
	}
	if len(diff.Deleted) > 0 {
		parts = append(parts, fmt.Sprintf("删除端口 [%s]", strings.Join(diff.Deleted, ", ")))
	}

	if len(parts) == 0 {
		return "端口无变更"
	}
	return strings.Join(parts, ", ")
}

func getPortName(p types.ServicePort) string {
	if p.Name != "" {
		return p.Name
	}
	return fmt.Sprintf("%s", p.Protocol)
}

// ==================== Ingress 规则对比 ====================

// IngressRuleDiffResult Ingress 规则对比结果
type IngressRuleDiffResult struct {
	AddedHosts    []string // 新增的主机
	DeletedHosts  []string // 删除的主机
	ModifiedHosts []string // 修改的主机（路径变更）
	TLSChanges    string   // TLS 变更描述
}

// CompareIngressRules 对比 Ingress 规则变更
func CompareIngressRules(oldIngress, newIngress *networkingv1.Ingress) *IngressRuleDiffResult {
	result := &IngressRuleDiffResult{}

	oldHosts := make(map[string]bool)
	newHosts := make(map[string]bool)

	for _, rule := range oldIngress.Spec.Rules {
		oldHosts[rule.Host] = true
	}
	for _, rule := range newIngress.Spec.Rules {
		newHosts[rule.Host] = true
	}

	// 检查新增
	for host := range newHosts {
		if !oldHosts[host] {
			result.AddedHosts = append(result.AddedHosts, host)
		}
	}

	// 检查删除
	for host := range oldHosts {
		if !newHosts[host] {
			result.DeletedHosts = append(result.DeletedHosts, host)
		}
	}

	// TLS 变更
	oldTLS := make(map[string]bool)
	newTLS := make(map[string]bool)
	for _, tls := range oldIngress.Spec.TLS {
		for _, host := range tls.Hosts {
			oldTLS[host] = true
		}
	}
	for _, tls := range newIngress.Spec.TLS {
		for _, host := range tls.Hosts {
			newTLS[host] = true
		}
	}

	var tlsAdded, tlsRemoved []string
	for host := range newTLS {
		if !oldTLS[host] {
			tlsAdded = append(tlsAdded, host)
		}
	}
	for host := range oldTLS {
		if !newTLS[host] {
			tlsRemoved = append(tlsRemoved, host)
		}
	}

	if len(tlsAdded) > 0 || len(tlsRemoved) > 0 {
		var tlsParts []string
		if len(tlsAdded) > 0 {
			tlsParts = append(tlsParts, fmt.Sprintf("启用TLS [%s]", strings.Join(tlsAdded, ", ")))
		}
		if len(tlsRemoved) > 0 {
			tlsParts = append(tlsParts, fmt.Sprintf("禁用TLS [%s]", strings.Join(tlsRemoved, ", ")))
		}
		result.TLSChanges = strings.Join(tlsParts, ", ")
	}

	return result
}

// BuildIngressDiffDetail 构建 Ingress 变更详情
func BuildIngressDiffDetail(diff *IngressRuleDiffResult) string {
	if diff == nil {
		return ""
	}

	var parts []string
	if len(diff.AddedHosts) > 0 {
		parts = append(parts, fmt.Sprintf("新增主机 [%s]", strings.Join(diff.AddedHosts, ", ")))
	}
	if len(diff.DeletedHosts) > 0 {
		parts = append(parts, fmt.Sprintf("删除主机 [%s]", strings.Join(diff.DeletedHosts, ", ")))
	}
	if diff.TLSChanges != "" {
		parts = append(parts, diff.TLSChanges)
	}

	if len(parts) == 0 {
		return "规则无变更"
	}
	return strings.Join(parts, ", ")
}

// ==================== Role 规则对比 ====================

// BuildRoleRulesSummary 构建 Role 规则摘要
func BuildRoleRulesSummary(rules []types.PolicyRuleInfo) string {
	if len(rules) == 0 {
		return "无规则"
	}

	var summaries []string
	for _, rule := range rules {
		resources := strings.Join(rule.Resources, ",")
		verbs := strings.Join(rule.Verbs, ",")
		summaries = append(summaries, fmt.Sprintf("%s(%s)", resources, verbs))
	}

	return strings.Join(summaries, "; ")
}

// ==================== RoleBinding 主体对比 ====================

// SubjectDiffResult 主体对比结果
type SubjectDiffResult struct {
	Added   []string
	Deleted []string
}

// CompareSubjects 对比主体变更
func CompareSubjects(oldSubjects, newSubjects []types.SubjectInfo) *SubjectDiffResult {
	result := &SubjectDiffResult{}

	oldMap := make(map[string]bool)
	newMap := make(map[string]bool)

	for _, s := range oldSubjects {
		key := fmt.Sprintf("%s/%s", s.Kind, s.Name)
		if s.Namespace != "" {
			key = fmt.Sprintf("%s/%s/%s", s.Kind, s.Namespace, s.Name)
		}
		oldMap[key] = true
	}

	for _, s := range newSubjects {
		key := fmt.Sprintf("%s/%s", s.Kind, s.Name)
		if s.Namespace != "" {
			key = fmt.Sprintf("%s/%s/%s", s.Kind, s.Namespace, s.Name)
		}
		newMap[key] = true
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
		return ""
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

// ==================== ServiceAccount 对比 ====================

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

// BuildCreateDetail 构建创建操作的详情
func BuildCreateDetail(resourceType, name string, details map[string]string) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("创建%s: %s", resourceType, name))

	for key, value := range details {
		if value != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", key, value))
		}
	}

	return strings.Join(parts, ", ")
}

// BuildDeleteDetail 构建删除操作的详情
func BuildDeleteDetail(resourceType, name string, extraInfo string) string {
	if extraInfo != "" {
		return fmt.Sprintf("删除%s: %s, %s", resourceType, name, extraInfo)
	}
	return fmt.Sprintf("删除%s: %s", resourceType, name)
}

// ==================== 时间处理 ====================

// calculateAge 计算资源的年龄（从创建时间到现在）
func calculateAge(creationTime time.Time) string {
	duration := time.Since(creationTime)

	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	if days > 0 {
		if hours > 0 {
			return formatDuration(days, hours, 0, "d", "h", "")
		}
		return formatDuration(days, 0, 0, "d", "", "")
	}

	if hours > 0 {
		if minutes > 0 {
			return formatDuration(0, hours, minutes, "", "h", "m")
		}
		return formatDuration(0, hours, 0, "", "h", "")
	}

	if minutes > 0 {
		return formatDuration(0, 0, minutes, "", "", "m")
	}

	return formatDuration(0, 0, int(duration.Seconds()), "", "", "s")
}

// formatDuration 格式化时间间隔
func formatDuration(d, h, m int, dUnit, hUnit, mUnit string) string {
	result := ""
	if d > 0 {
		result += formatUnit(d, dUnit)
	}
	if h > 0 {
		result += formatUnit(h, hUnit)
	}
	if m > 0 {
		result += formatUnit(m, mUnit)
	}
	return result
}

// formatUnit 格式化单个时间单位
func formatUnit(value int, unit string) string {
	if value > 0 {
		return formatInt(value) + unit
	}
	return ""
}

// formatInt 将整数转换为字符串
func formatInt(i int) string {
	return fmt.Sprintf("%d", i)
}

// ==================== IngressClass 相关 ====================

// isDefaultIngressClass 判断 IngressClass 是否为默认
func isDefaultIngressClass(ic *networkingv1.IngressClass) bool {
	if ic.Annotations == nil {
		return false
	}
	// 检查标准注解
	if val, ok := ic.Annotations["ingressclass.kubernetes.io/is-default-class"]; ok {
		return val == "true"
	}
	return false
}

// ==================== Ingress 类型转换 ====================

// convertRulesToTypes 转换 K8s Manager Ingress 规则到 API 类型
func convertRulesToTypes(rules []k8smanagertypes.IngressRuleInfo) []types.IngressRuleInfo {
	if rules == nil {
		return []types.IngressRuleInfo{}
	}

	result := make([]types.IngressRuleInfo, len(rules))
	for i, rule := range rules {
		result[i] = types.IngressRuleInfo{
			Host:  rule.Host,
			Paths: convertPathsToTypes(rule.Paths),
		}
	}
	return result
}

// convertPathsToTypes 转换路径信息
func convertPathsToTypes(paths []k8smanagertypes.IngressPathInfo) []types.IngressPathInfo {
	if paths == nil {
		return []types.IngressPathInfo{}
	}

	result := make([]types.IngressPathInfo, len(paths))
	for i, path := range paths {
		result[i] = types.IngressPathInfo{
			Path:     path.Path,
			PathType: path.PathType,
			Backend:  convertBackendToTypes(path.Backend),
		}
	}
	return result
}

// convertBackendToTypes 转换后端信息
func convertBackendToTypes(backend k8smanagertypes.IngressBackendInfo) types.IngressBackendInfo {
	result := types.IngressBackendInfo{
		ServiceName: backend.ServiceName,
		ServicePort: backend.ServicePort,
	}

	if backend.ResourceRef != nil {
		result.ResourceRef = types.IngressResourceRef{
			APIGroup: backend.ResourceRef.APIGroup,
			Kind:     backend.ResourceRef.Kind,
			Name:     backend.ResourceRef.Name,
		}
	}

	return result
}

// convertTLSToTypes 转换 TLS 配置到 API 类型
func convertTLSToTypes(tls []k8smanagertypes.IngressTLSInfo) []types.IngressTLSInfo {
	if tls == nil {
		return []types.IngressTLSInfo{}
	}

	result := make([]types.IngressTLSInfo, len(tls))
	for i, t := range tls {
		result[i] = types.IngressTLSInfo{
			Hosts:      t.Hosts,
			SecretName: t.SecretName,
		}
	}
	return result
}

// convertLoadBalancerToTypes 转换负载均衡器信息到 API 类型
func convertLoadBalancerToTypes(lb k8smanagertypes.IngressLoadBalancerInfo) types.IngressLoadBalancerInfo {
	if lb.Ingress == nil {
		return types.IngressLoadBalancerInfo{
			Ingress: []types.IngressLoadBalancerIngress{},
		}
	}

	ingresses := make([]types.IngressLoadBalancerIngress, len(lb.Ingress))
	for i, ing := range lb.Ingress {
		ingresses[i] = types.IngressLoadBalancerIngress{
			IP:       ing.IP,
			Hostname: ing.Hostname,
		}
	}

	return types.IngressLoadBalancerInfo{
		Ingress: ingresses,
	}
}

// ==================== 通用工具函数 ====================

// stringPtr 返回字符串指针
func stringPtr(s string) *string {
	return &s
}

// int32Ptr 返回 int32 指针
func int32Ptr(i int32) *int32 {
	return &i
}

// boolPtr 返回 bool 指针
func boolPtr(b bool) *bool {
	return &b
}

// getStringValue 安全地获取字符串指针的值
func getStringValue(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// getInt32Value 安全地获取 int32 指针的值
func getInt32Value(ptr *int32) int32 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

// getBoolValue 安全地获取 bool 指针的值
func getBoolValue(ptr *bool) bool {
	if ptr == nil {
		return false
	}
	return *ptr
}

// ==================== 错误处理 ====================

// wrapError 包装错误信息
func wrapError(operation, resourceType, resourceName string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s %s %s 失败: %v", operation, resourceType, resourceName, err)
}

// isNotFoundError 判断是否为资源不存在错误
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// 可以根据实际错误类型判断
	return strings.Contains(err.Error(), "不存在") ||
		strings.Contains(err.Error(), "not found")
}
