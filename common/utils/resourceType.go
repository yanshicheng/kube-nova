package utils

import "strings"

// 内置的 Kubernetes 资源类型列表
var ResourceTypes = []string{
	"Pod",
	"Deployment",
	"StatefulSet",
	"DaemonSet",
	"Job",
	"CronJob",
	"Service",
	"Ingress",
	"ConfigMap",
	"Secret",
	"PVC",
}

// IsResourceType 判断资源类型是否在内置列表中
// 参数：
//
//	resourceType - 要检查的资源类型字符串
//
// 返回：
//
//	bool - 如果是内置类型返回 true，否则返回 false
func IsResourceType(resourceType string) bool {
	for _, rt := range ResourceTypes {
		if strings.ToUpper(rt) == strings.ToUpper(resourceType) {
			return true
		}
	}
	return false
}

// GetResourceTypes 获取所有内置资源类型列表
func GetResourceTypes() []string {
	// 返回副本，避免外部修改
	result := make([]string, len(ResourceTypes))
	copy(result, ResourceTypes)
	return result
}
