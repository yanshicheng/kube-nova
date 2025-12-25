package cluster

import (
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// ResourceType 资源类型
type ResourceType string

const (
	ResourceNamespace   ResourceType = "Namespace"
	ResourcePod         ResourceType = "Pod"
	ResourceService     ResourceType = "Service"
	ResourceEndpoints   ResourceType = "Endpoints"
	ResourceDeployment  ResourceType = "Deployment"
	ResourceDaemonSet   ResourceType = "DaemonSet"
	ResourceStatefulSet ResourceType = "StatefulSet"
	ResourceReplicaSet  ResourceType = "ReplicaSet"
	ResourceJob         ResourceType = "Job"
	ResourceCronJob     ResourceType = "CronJob"
	ResourceIngress     ResourceType = "Ingress"
	ResourceConfigMap   ResourceType = "ConfigMap"
	ResourceSecret      ResourceType = "Secret"
	ResourceNode        ResourceType = "Node"
	ResourceVPA         ResourceType = "VerticalPodAutoscaler"
	ResourceHPA         ResourceType = "HorizontalPodAutoscaler"
	ResourceCanary      ResourceType = "Canary"
	// Role
	ResourceRole               ResourceType = "Role"
	ResourceClusterRole        ResourceType = "ClusterRole"
	ResourceRoleBinding        ResourceType = "RoleBinding"
	ResourceClusterRoleBinding ResourceType = "ClusterRoleBinding"
	ResourceServiceAccount     ResourceType = "ServiceAccount"
)

// 必须启动的核心资源列表
var CoreResources = []ResourceType{
	ResourceRole,
	ResourceClusterRole,
	ResourceRoleBinding,
	ResourceClusterRoleBinding,
	ResourceServiceAccount,
	ResourceNamespace,
	ResourcePod,
	ResourceService,
	ResourceEndpoints,
	ResourceDeployment,
	ResourceDaemonSet,
	ResourceStatefulSet,
	ResourceReplicaSet,
	ResourceJob,
	ResourceCronJob,
	ResourceIngress,
	ResourceHPA,
}

// 可选资源列表
var OptionalResources = []ResourceType{
	ResourceStatefulSet,
	ResourceConfigMap,
	ResourceSecret,
	ResourceNode,
}

// InformerManager Informer 管理器
type InformerManager struct {
	factory    informers.SharedInformerFactory // informer 工厂
	kubeClient kubernetes.Interface
}
