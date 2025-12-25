package types

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// ReplicaSetInfo ReplicaSet 信息
type ReplicaSetInfo struct {
	Name              string
	Namespace         string
	Replicas          int32
	ReadyReplicas     int32
	AvailableReplicas int32
	CreationTimestamp time.Time
	Images            []string
	OwnerReferences   []string // 所属的控制器（通常是 Deployment）
}

// ListReplicaSetResponse ReplicaSet 列表响应
type ListReplicaSetResponse struct {
	ListResponse
	Items []ReplicaSetInfo
}

// ReplicaSetOperator ReplicaSet 操作器接口
type ReplicaSetOperator interface {
	// 基础 CRUD 操作
	Create(*appsv1.ReplicaSet) (*appsv1.ReplicaSet, error)
	Get(namespace, name string) (*appsv1.ReplicaSet, error)
	Update(*appsv1.ReplicaSet) (*appsv1.ReplicaSet, error)
	Delete(namespace, name string) error
	List(namespace string, req ListRequest) (*ListReplicaSetResponse, error)

	// 高级操作
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	UpdateLabels(namespace, name string, labels map[string]string) error
	UpdateAnnotations(namespace, name string, annotations map[string]string) error

	// 扩缩容

	Scale(namespace, name string, replicas int32) error

	// 事件（使用 Deployment 中定义的 ListEventResponse）
	GetEvents(namespace, name string) ([]EventInfo, error) // 获取事件
}
