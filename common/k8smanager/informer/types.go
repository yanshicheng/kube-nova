package informer

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceType 资源类型
type ResourceType string

const (
	// 核心资源
	ResourceNamespace ResourceType = "Namespace"
	ResourcePod       ResourceType = "Pod"
	ResourceService   ResourceType = "Service"
	ResourceEndpoints ResourceType = "Endpoints"
	ResourceConfigMap ResourceType = "ConfigMap"
	ResourceSecret    ResourceType = "Secret"
	ResourceNode      ResourceType = "Node"
	ResourcePVC       ResourceType = "PersistentVolumeClaim"
	ResourcePV        ResourceType = "PersistentVolume"

	// 工作负载资源
	ResourceDeployment  ResourceType = "Deployment"
	ResourceStatefulSet ResourceType = "StatefulSet"
	ResourceDaemonSet   ResourceType = "DaemonSet"
	ResourceReplicaSet  ResourceType = "ReplicaSet"
	ResourceJob         ResourceType = "Job"
	ResourceCronJob     ResourceType = "CronJob"

	// 网络资源
	ResourceIngress       ResourceType = "Ingress"
	ResourceNetworkPolicy ResourceType = "NetworkPolicy"

	// 存储资源
	ResourceStorageClass ResourceType = "StorageClass"

	// RBAC资源
	ResourceServiceAccount     ResourceType = "ServiceAccount"
	ResourceRole               ResourceType = "Role"
	ResourceRoleBinding        ResourceType = "RoleBinding"
	ResourceClusterRole        ResourceType = "ClusterRole"
	ResourceClusterRoleBinding ResourceType = "ClusterRoleBinding"
)

// ResourceInfo 资源信息
type ResourceInfo struct {
	// 资源类型
	Type ResourceType

	// 资源组版本
	GroupVersion schema.GroupVersion

	// 资源名称（复数）
	Resource string

	// 是否是命名空间资源
	Namespaced bool

	// 是否启用
	Enabled bool
}

// GetDefaultResources 获取默认需要监控的资源列表
func GetDefaultResources() []ResourceInfo {
	return []ResourceInfo{
		// 核心资源 - v1
		{
			Type:         ResourceNamespace,
			GroupVersion: schema.GroupVersion{Group: "", Version: "v1"},
			Resource:     "namespaces",
			Namespaced:   false,
			Enabled:      true,
		},
		{
			Type:         ResourcePod,
			GroupVersion: schema.GroupVersion{Group: "", Version: "v1"},
			Resource:     "pods",
			Namespaced:   true,
			Enabled:      true,
		},
		{
			Type:         ResourceService,
			GroupVersion: schema.GroupVersion{Group: "", Version: "v1"},
			Resource:     "services",
			Namespaced:   true,
			Enabled:      true,
		},
		{
			Type:         ResourceEndpoints,
			GroupVersion: schema.GroupVersion{Group: "", Version: "v1"},
			Resource:     "endpoints",
			Namespaced:   true,
			Enabled:      true,
		},
		{
			Type:         ResourceConfigMap,
			GroupVersion: schema.GroupVersion{Group: "", Version: "v1"},
			Resource:     "configmaps",
			Namespaced:   true,
			Enabled:      true,
		},
		{
			Type:         ResourceSecret,
			GroupVersion: schema.GroupVersion{Group: "", Version: "v1"},
			Resource:     "secrets",
			Namespaced:   true,
			Enabled:      true,
		},
		{
			Type:         ResourceNode,
			GroupVersion: schema.GroupVersion{Group: "", Version: "v1"},
			Resource:     "nodes",
			Namespaced:   false,
			Enabled:      true,
		},
		{
			Type:         ResourcePVC,
			GroupVersion: schema.GroupVersion{Group: "", Version: "v1"},
			Resource:     "persistentvolumeclaims",
			Namespaced:   true,
			Enabled:      true,
		},
		{
			Type:         ResourcePV,
			GroupVersion: schema.GroupVersion{Group: "", Version: "v1"},
			Resource:     "persistentvolumes",
			Namespaced:   false,
			Enabled:      true,
		},

		// 工作负载资源 - apps/v1
		{
			Type:         ResourceDeployment,
			GroupVersion: schema.GroupVersion{Group: "apps", Version: "v1"},
			Resource:     "deployments",
			Namespaced:   true,
			Enabled:      true,
		},
		{
			Type:         ResourceStatefulSet,
			GroupVersion: schema.GroupVersion{Group: "apps", Version: "v1"},
			Resource:     "statefulsets",
			Namespaced:   true,
			Enabled:      true,
		},
		{
			Type:         ResourceDaemonSet,
			GroupVersion: schema.GroupVersion{Group: "apps", Version: "v1"},
			Resource:     "daemonsets",
			Namespaced:   true,
			Enabled:      true,
		},
		{
			Type:         ResourceReplicaSet,
			GroupVersion: schema.GroupVersion{Group: "apps", Version: "v1"},
			Resource:     "replicasets",
			Namespaced:   true,
			Enabled:      true,
		},

		// 批处理资源 - batch/v1
		{
			Type:         ResourceJob,
			GroupVersion: schema.GroupVersion{Group: "batch", Version: "v1"},
			Resource:     "jobs",
			Namespaced:   true,
			Enabled:      true,
		},
		{
			Type:         ResourceCronJob,
			GroupVersion: schema.GroupVersion{Group: "batch", Version: "v1"},
			Resource:     "cronjobs",
			Namespaced:   true,
			Enabled:      true,
		},

		// 网络资源 - networking.k8s.io/v1
		{
			Type:         ResourceIngress,
			GroupVersion: schema.GroupVersion{Group: "networking.k8s.io", Version: "v1"},
			Resource:     "ingresses",
			Namespaced:   true,
			Enabled:      true,
		},
		{
			Type:         ResourceNetworkPolicy,
			GroupVersion: schema.GroupVersion{Group: "networking.k8s.io", Version: "v1"},
			Resource:     "networkpolicies",
			Namespaced:   true,
			Enabled:      true,
		},

		// 存储资源 - storage.k8s.io/v1
		{
			Type:         ResourceStorageClass,
			GroupVersion: schema.GroupVersion{Group: "storage.k8s.io", Version: "v1"},
			Resource:     "storageclasses",
			Namespaced:   false,
			Enabled:      true,
		},

		// RBAC资源 - rbac.authorization.k8s.io/v1
		{
			Type:         ResourceServiceAccount,
			GroupVersion: schema.GroupVersion{Group: "", Version: "v1"},
			Resource:     "serviceaccounts",
			Namespaced:   true,
			Enabled:      false, // 默认不启用
		},
		{
			Type:         ResourceRole,
			GroupVersion: schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1"},
			Resource:     "roles",
			Namespaced:   true,
			Enabled:      false, // 默认不启用
		},
		{
			Type:         ResourceRoleBinding,
			GroupVersion: schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1"},
			Resource:     "rolebindings",
			Namespaced:   true,
			Enabled:      false, // 默认不启用
		},
		{
			Type:         ResourceClusterRole,
			GroupVersion: schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1"},
			Resource:     "clusterroles",
			Namespaced:   false,
			Enabled:      false, // 默认不启用
		},
		{
			Type:         ResourceClusterRoleBinding,
			GroupVersion: schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1"},
			Resource:     "clusterrolebindings",
			Namespaced:   false,
			Enabled:      false, // 默认不启用
		},
	}
}

// GetEnabledResources 获取启用的资源列表
func GetEnabledResources() []ResourceInfo {
	var enabled []ResourceInfo
	for _, resource := range GetDefaultResources() {
		if resource.Enabled {
			enabled = append(enabled, resource)
		}
	}
	return enabled
}
