package types

import (
	corev1 "k8s.io/api/core/v1"
)

// EventInfo 事件信息
type EventInfo struct {
	Type               string            `json:"type"`               // Normal, Warning
	Reason             string            `json:"reason"`             // 事件原因
	Message            string            `json:"message"`            // 事件消息
	Count              int32             `json:"count"`              // 发生次数
	FirstTimestamp     int64             `json:"firstTimestamp"`     // 首次发生时间（毫秒）
	LastTimestamp      int64             `json:"lastTimestamp"`      // 最后发生时间（毫秒）
	Source             string            `json:"source"`             // 事件源组件
	InvolvedObjectKind string            `json:"involvedObjectKind"` // 关联对象类型
	InvolvedObjectName string            `json:"involvedObjectName"` // 关联对象名称
	InvolvedObjectUID  string            `json:"involvedObjectUid"`  // 关联对象 UID
	ReportingComponent string            `json:"reportingComponent"` // 报告组件
	ReportingInstance  string            `json:"reportingInstance"`  // 报告实例
	Action             string            `json:"action"`             // 动作
	EventTime          int64             `json:"eventTime"`          // 事件时间（毫秒）
	Series             *EventSeries      `json:"series,omitempty"`   // 事件序列
	Related            *ObjectReference  `json:"related,omitempty"`  // 相关对象
	Annotations        map[string]string `json:"annotations"`        // 注解
}

// EventSeries 事件序列信息
type EventSeries struct {
	Count            int32  `json:"count"`            // 序列中的事件数量
	LastObservedTime int64  `json:"lastObservedTime"` // 最后观察时间（毫秒）
	State            string `json:"state"`            // 状态
}

// ObjectReference 对象引用
type ObjectReference struct {
	Kind            string `json:"kind"`
	Namespace       string `json:"namespace,omitempty"`
	Name            string `json:"name"`
	UID             string `json:"uid"`
	APIVersion      string `json:"apiVersion"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
	FieldPath       string `json:"fieldPath,omitempty"`
}

// EventsListResponse 事件列表响应（带分页）
type EventsListResponse struct {
	Items      []EventInfo `json:"items"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	TotalPages int         `json:"totalPages"`
}

// EventsQueryRequest 事件查询请求（带分页）
type EventsQueryRequest struct {
	Namespace          string `form:"namespace,optional"`          // 命名空间（可选）
	InvolvedObjectKind string `form:"involvedObjectKind,optional"` // 关联对象类型
	InvolvedObjectName string `form:"involvedObjectName,optional"` // 关联对象名称
	InvolvedObjectUID  string `form:"involvedObjectUid,optional"`  // 关联对象 UID
	Type               string `form:"type,optional"`               // 事件类型 Normal/Warning
	Page               int    `form:"page,default=1"`              // 页码（从1开始）
	PageSize           int    `form:"pageSize,default=20"`         // 每页数量
}

// EventOperator 事件操作器接口
type EventOperator interface {
	// GetPodEvents 获取 Pod 的事件（带分页）
	GetPodEvents(namespace, podName string, page, pageSize int) (*EventsListResponse, error)

	// GetNamespaceEvents 获取命名空间的所有事件（带分页）
	GetNamespaceEvents(namespace string, page, pageSize int) (*EventsListResponse, error)

	// GetResourceEvents 获取特定资源的事件（带分页）
	GetResourceEvents(namespace, resourceKind, resourceName, resourceUID string, page, pageSize int) (*EventsListResponse, error)

	// QueryEvents 根据条件查询事件（带分页）
	QueryEvents(req EventsQueryRequest) (*EventsListResponse, error)

	// GetAllEvents 获取所有事件（跨命名空间，带分页）
	GetAllEvents(page, pageSize int) (*EventsListResponse, error)
}

// ConvertK8sEventToEventInfo 将 K8s Event 转换为 EventInfo
func ConvertK8sEventToEventInfo(event *corev1.Event) EventInfo {
	info := EventInfo{
		Type:               event.Type,
		Reason:             event.Reason,
		Message:            event.Message,
		Count:              event.Count,
		FirstTimestamp:     event.FirstTimestamp.UnixMilli(),
		LastTimestamp:      event.LastTimestamp.UnixMilli(),
		Source:             event.Source.Component,
		InvolvedObjectKind: event.InvolvedObject.Kind,
		InvolvedObjectName: event.InvolvedObject.Name,
		InvolvedObjectUID:  string(event.InvolvedObject.UID),
		ReportingComponent: event.ReportingController,
		ReportingInstance:  event.ReportingInstance,
		Action:             event.Action,
		Annotations:        event.Annotations,
	}

	// EventTime
	if !event.EventTime.IsZero() {
		info.EventTime = event.EventTime.UnixMilli()
	}

	// Series
	if event.Series != nil {
		info.Series = &EventSeries{
			Count:            event.Series.Count,
			LastObservedTime: event.Series.LastObservedTime.UnixMilli(),
			State:            string(event.Series.String()),
		}
	}

	// Related
	if event.Related != nil {
		info.Related = &ObjectReference{
			Kind:            event.Related.Kind,
			Namespace:       event.Related.Namespace,
			Name:            event.Related.Name,
			UID:             string(event.Related.UID),
			APIVersion:      event.Related.APIVersion,
			ResourceVersion: event.Related.ResourceVersion,
			FieldPath:       event.Related.FieldPath,
		}
	}

	return info
}
