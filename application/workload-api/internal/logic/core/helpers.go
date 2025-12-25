// logic/core/helpers.go
// 辅助函数，供所有 Logic 使用

package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8smanagertypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"
	networkingv1 "k8s.io/api/networking/v1"
)

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
