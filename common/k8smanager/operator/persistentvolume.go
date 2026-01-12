package operator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/yaml"
)

type persistentVolumeOperator struct {
	BaseOperator
	client          kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	pvLister        corev1lister.PersistentVolumeLister
	pvcLister       corev1lister.PersistentVolumeClaimLister
	podLister       corev1lister.PodLister
}

// NewPersistentVolumeOperator 创建 PV 操作器（不使用 informer）
func NewPersistentVolumeOperator(ctx context.Context, client kubernetes.Interface) types.PersistentVolumeOperator {
	return &persistentVolumeOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

// NewPersistentVolumeOperatorWithInformer 创建 PV 操作器（使用 informer）
func NewPersistentVolumeOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.PersistentVolumeOperator {
	var pvLister corev1lister.PersistentVolumeLister
	var pvcLister corev1lister.PersistentVolumeClaimLister
	var podLister corev1lister.PodLister

	if informerFactory != nil {
		pvLister = informerFactory.Core().V1().PersistentVolumes().Lister()
		pvcLister = informerFactory.Core().V1().PersistentVolumeClaims().Lister()
		podLister = informerFactory.Core().V1().Pods().Lister()
	}

	return &persistentVolumeOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		pvLister:        pvLister,
		pvcLister:       pvcLister,
		podLister:       podLister,
	}
}

// Create 创建 PV
func (p *persistentVolumeOperator) Create(pv *corev1.PersistentVolume) (*corev1.PersistentVolume, error) {
	if pv == nil || pv.Name == "" {
		return nil, fmt.Errorf("PV对象和名称不能为空")
	}
	injectCommonAnnotations(pv)
	created, err := p.client.CoreV1().PersistentVolumes().Create(p.ctx, pv, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建PV失败: %v", err)
	}

	return created, nil
}

// Get 获取 PV
func (p *persistentVolumeOperator) Get(name string) (*corev1.PersistentVolume, error) {
	if name == "" {
		return nil, fmt.Errorf("名称不能为空")
	}

	// 优先使用 informer
	if p.useInformer && p.pvLister != nil {
		pv, err := p.pvLister.Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("PV %s 不存在", name)
			}
			// informer 出错时回退到 API 调用
			pv, apiErr := p.client.CoreV1().PersistentVolumes().Get(p.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取PV失败: %v", apiErr)
			}
			p.injectTypeMeta(pv)
			return pv, nil
		}
		result := pv.DeepCopy()
		p.injectTypeMeta(result)
		return result, nil
	}

	// 使用 API 调用
	pv, err := p.client.CoreV1().PersistentVolumes().Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("PV %s 不存在", name)
		}
		return nil, fmt.Errorf("获取PV失败: %v", err)
	}

	p.injectTypeMeta(pv)
	return pv, nil
}

// injectTypeMeta 注入 TypeMeta
func (p *persistentVolumeOperator) injectTypeMeta(pv *corev1.PersistentVolume) {
	pv.TypeMeta = metav1.TypeMeta{
		Kind:       "PersistentVolume",
		APIVersion: "v1",
	}
}

// Update 更新 PV
func (p *persistentVolumeOperator) Update(pv *corev1.PersistentVolume) (*corev1.PersistentVolume, error) {
	if pv == nil || pv.Name == "" {
		return nil, fmt.Errorf("PV对象和名称不能为空")
	}

	updated, err := p.client.CoreV1().PersistentVolumes().Update(p.ctx, pv, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新PV失败: %v", err)
	}

	return updated, nil
}

// Delete 删除 PV
func (p *persistentVolumeOperator) Delete(name string) error {
	if name == "" {
		return fmt.Errorf("名称不能为空")
	}

	err := p.client.CoreV1().PersistentVolumes().Delete(p.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除PV失败: %v", err)
	}

	return nil
}

// List 获取 PV 列表
func (p *persistentVolumeOperator) List(search string, labelSelector string, status string, storageClass string) (*types.ListPersistentVolumeResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败: %v", err)
		}
		selector = parsedSelector
	}

	var pvList []*corev1.PersistentVolume
	var err error

	// 优先使用 informer
	if p.useInformer && p.pvLister != nil {
		pvList, err = p.pvLister.List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取PV列表失败: %v", err)
		}
	} else {
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		list, err := p.client.CoreV1().PersistentVolumes().List(p.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取PV列表失败: %v", err)
		}
		pvList = make([]*corev1.PersistentVolume, len(list.Items))
		for i := range list.Items {
			pvList[i] = &list.Items[i]
		}
	}

	// 状态过滤
	if status != "" {
		filtered := make([]*corev1.PersistentVolume, 0)
		for _, pv := range pvList {
			if string(pv.Status.Phase) == status {
				filtered = append(filtered, pv)
			}
		}
		pvList = filtered
	}

	// StorageClass 过滤
	if storageClass != "" {
		filtered := make([]*corev1.PersistentVolume, 0)
		for _, pv := range pvList {
			if pv.Spec.StorageClassName == storageClass {
				filtered = append(filtered, pv)
			}
		}
		pvList = filtered
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*corev1.PersistentVolume, 0)
		searchLower := strings.ToLower(search)
		for _, pv := range pvList {
			if strings.Contains(strings.ToLower(pv.Name), searchLower) {
				filtered = append(filtered, pv)
			}
		}
		pvList = filtered
	}

	// 转换为响应格式
	items := make([]types.PersistentVolumeInfo, len(pvList))
	for i, pv := range pvList {
		items[i] = p.toPersistentVolumeInfo(pv)
	}

	// 按名称排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return &types.ListPersistentVolumeResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// toPersistentVolumeInfo 转换为 PersistentVolumeInfo
func (p *persistentVolumeOperator) toPersistentVolumeInfo(pv *corev1.PersistentVolume) types.PersistentVolumeInfo {
	capacity := ""
	if qty, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		capacity = qty.String()
	}

	claim := ""
	if pv.Spec.ClaimRef != nil {
		claim = fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
	}

	reclaimPolicy := "Delete"
	if pv.Spec.PersistentVolumeReclaimPolicy != "" {
		reclaimPolicy = string(pv.Spec.PersistentVolumeReclaimPolicy)
	}

	volumeMode := "Filesystem"
	if pv.Spec.VolumeMode != nil {
		volumeMode = string(*pv.Spec.VolumeMode)
	}

	volumeAttributesClass := "<unset>"
	if pv.Spec.VolumeAttributesClassName != nil && *pv.Spec.VolumeAttributesClassName != "" {
		volumeAttributesClass = *pv.Spec.VolumeAttributesClassName
	}

	return types.PersistentVolumeInfo{
		Name:                  pv.Name,
		Capacity:              capacity,
		AccessModes:           p.formatAccessModes(pv.Spec.AccessModes),
		ReclaimPolicy:         reclaimPolicy,
		Status:                string(pv.Status.Phase),
		Claim:                 claim,
		StorageClass:          pv.Spec.StorageClassName,
		VolumeAttributesClass: volumeAttributesClass,
		Reason:                pv.Status.Reason,
		VolumeMode:            volumeMode,
		Labels:                pv.Labels,
		Annotations:           pv.Annotations,
		Age:                   p.formatAge(pv.CreationTimestamp.Time),
		CreationTimestamp:     pv.CreationTimestamp.UnixMilli(),
	}
}

// formatAccessModes 格式化访问模式
func (p *persistentVolumeOperator) formatAccessModes(modes []corev1.PersistentVolumeAccessMode) string {
	abbrevs := make([]string, 0, len(modes))
	for _, mode := range modes {
		switch mode {
		case corev1.ReadWriteOnce:
			abbrevs = append(abbrevs, "RWO")
		case corev1.ReadOnlyMany:
			abbrevs = append(abbrevs, "ROX")
		case corev1.ReadWriteMany:
			abbrevs = append(abbrevs, "RWX")
		case corev1.ReadWriteOncePod:
			abbrevs = append(abbrevs, "RWOP")
		default:
			abbrevs = append(abbrevs, string(mode))
		}
	}
	return strings.Join(abbrevs, ",")
}

// formatAge 格式化时间
func (p *persistentVolumeOperator) formatAge(t time.Time) string {
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

// GetYaml 获取 PV 的 YAML
func (p *persistentVolumeOperator) GetYaml(name string) (string, error) {
	pv, err := p.Get(name)
	if err != nil {
		return "", err
	}

	pv.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(pv)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

// Describe 获取 PV 详情
func (p *persistentVolumeOperator) Describe(name string) (string, error) {
	pv, err := p.Get(name)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Name:            %s\n", pv.Name))
	sb.WriteString(fmt.Sprintf("Labels:          %s\n", p.formatLabels(pv.Labels)))
	sb.WriteString(fmt.Sprintf("Annotations:     %s\n", p.formatAnnotations(pv.Annotations)))
	sb.WriteString(fmt.Sprintf("Finalizers:      %v\n", pv.Finalizers))
	sb.WriteString(fmt.Sprintf("StorageClass:    %s\n", pv.Spec.StorageClassName))
	sb.WriteString(fmt.Sprintf("Status:          %s\n", pv.Status.Phase))

	// Claim
	if pv.Spec.ClaimRef != nil {
		sb.WriteString(fmt.Sprintf("Claim:           %s/%s\n", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name))
	} else {
		sb.WriteString("Claim:           \n")
	}

	sb.WriteString(fmt.Sprintf("Reclaim Policy:  %s\n", pv.Spec.PersistentVolumeReclaimPolicy))
	sb.WriteString(fmt.Sprintf("Access Modes:    %s\n", p.formatAccessModes(pv.Spec.AccessModes)))

	// VolumeMode
	if pv.Spec.VolumeMode != nil {
		sb.WriteString(fmt.Sprintf("VolumeMode:      %s\n", *pv.Spec.VolumeMode))
	} else {
		sb.WriteString("VolumeMode:      Filesystem\n")
	}

	// Capacity
	if qty, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		sb.WriteString(fmt.Sprintf("Capacity:        %s\n", qty.String()))
	}

	// Node Affinity
	if pv.Spec.NodeAffinity != nil && pv.Spec.NodeAffinity.Required != nil {
		sb.WriteString("Node Affinity:   \n")
		for _, term := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms {
			for _, expr := range term.MatchExpressions {
				sb.WriteString(fmt.Sprintf("                 Required Terms: %s %s %v\n", expr.Key, expr.Operator, expr.Values))
			}
		}
	} else {
		sb.WriteString("Node Affinity:   <none>\n")
	}

	// Message
	if pv.Status.Message != "" {
		sb.WriteString(fmt.Sprintf("Message:         %s\n", pv.Status.Message))
	}

	// Source Type
	sb.WriteString(fmt.Sprintf("Source:\n"))
	p.describePVSource(&sb, pv)

	// Events
	events, err := p.getEvents(pv.Name)
	if err == nil && len(events) > 0 {
		sb.WriteString("Events:\n")
		sb.WriteString("  Type    Reason  Age  From  Message\n")
		sb.WriteString("  ----    ------  ---  ----  -------\n")
		for _, event := range events {
			age := p.formatAge(time.UnixMilli(event.LastTimestamp))
			sb.WriteString(fmt.Sprintf("  %s  %s  %s  %s  %s\n",
				event.Type, event.Reason, age, event.Source, event.Message))
		}
	} else {
		sb.WriteString("Events:          <none>\n")
	}

	return sb.String(), nil
}

// describePVSource 描述 PV 的存储源
func (p *persistentVolumeOperator) describePVSource(sb *strings.Builder, pv *corev1.PersistentVolume) {
	source := pv.Spec.PersistentVolumeSource

	switch {
	case source.NFS != nil:
		sb.WriteString("    Type:      NFS (an NFS mount that lasts the lifetime of a pod)\n")
		sb.WriteString(fmt.Sprintf("    Server:    %s\n", source.NFS.Server))
		sb.WriteString(fmt.Sprintf("    Path:      %s\n", source.NFS.Path))
		sb.WriteString(fmt.Sprintf("    ReadOnly:  %v\n", source.NFS.ReadOnly))

	case source.HostPath != nil:
		sb.WriteString("    Type:      HostPath (bare host directory volume)\n")
		sb.WriteString(fmt.Sprintf("    Path:      %s\n", source.HostPath.Path))
		if source.HostPath.Type != nil {
			sb.WriteString(fmt.Sprintf("    HostPathType: %s\n", *source.HostPath.Type))
		}

	case source.CSI != nil:
		sb.WriteString("    Type:      CSI (a Container Storage Interface volume)\n")
		sb.WriteString(fmt.Sprintf("    Driver:    %s\n", source.CSI.Driver))
		sb.WriteString(fmt.Sprintf("    VolumeHandle: %s\n", source.CSI.VolumeHandle))
		sb.WriteString(fmt.Sprintf("    ReadOnly:  %v\n", source.CSI.ReadOnly))
		if len(source.CSI.VolumeAttributes) > 0 {
			sb.WriteString("    VolumeAttributes:\n")
			for k, v := range source.CSI.VolumeAttributes {
				sb.WriteString(fmt.Sprintf("      %s=%s\n", k, v))
			}
		}

	case source.Local != nil:
		sb.WriteString("    Type:      Local (a network-attached local volume)\n")
		sb.WriteString(fmt.Sprintf("    Path:      %s\n", source.Local.Path))

	case source.ISCSI != nil:
		sb.WriteString("    Type:      ISCSI (an ISCSI disk volume)\n")
		sb.WriteString(fmt.Sprintf("    TargetPortal: %s\n", source.ISCSI.TargetPortal))
		sb.WriteString(fmt.Sprintf("    IQN:       %s\n", source.ISCSI.IQN))
		sb.WriteString(fmt.Sprintf("    Lun:       %d\n", source.ISCSI.Lun))

	case source.FC != nil:
		sb.WriteString("    Type:      FC (Fibre Channel volume)\n")
		sb.WriteString(fmt.Sprintf("    TargetWWNs: %v\n", source.FC.TargetWWNs))
		if source.FC.Lun != nil {
			sb.WriteString(fmt.Sprintf("    Lun:       %d\n", *source.FC.Lun))
		}

	case source.RBD != nil:
		sb.WriteString("    Type:      RBD (Ceph RBD volume)\n")
		sb.WriteString(fmt.Sprintf("    CephMonitors: %v\n", source.RBD.CephMonitors))
		sb.WriteString(fmt.Sprintf("    RBDImage:  %s\n", source.RBD.RBDImage))
		sb.WriteString(fmt.Sprintf("    RBDPool:   %s\n", source.RBD.RBDPool))

	default:
		sb.WriteString("    Type:      Unknown\n")
	}
}

// formatLabels 格式化标签
func (p *persistentVolumeOperator) formatLabels(labelMap map[string]string) string {
	if len(labelMap) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(labelMap))
	for k, v := range labelMap {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
}

// formatAnnotations 格式化注解
func (p *persistentVolumeOperator) formatAnnotations(annotations map[string]string) string {
	if len(annotations) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(annotations))
	for k, v := range annotations {
		if len(v) > 50 {
			v = v[:50] + "..."
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
}

// getEvents 获取事件
func (p *persistentVolumeOperator) getEvents(name string) ([]types.EventInfo, error) {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=PersistentVolume", name)
	events, err := p.client.CoreV1().Events("").List(p.ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, err
	}

	result := make([]types.EventInfo, 0, len(events.Items))
	for _, event := range events.Items {
		eventInfo := types.EventInfo{
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

		// 处理 EventTime
		if !event.EventTime.IsZero() {
			eventInfo.EventTime = event.EventTime.UnixMilli()
		}

		// 处理 Series
		if event.Series != nil {
			eventInfo.Series = &types.EventSeries{
				Count:            event.Series.Count,
				LastObservedTime: event.Series.LastObservedTime.UnixMilli(),
			}
		}

		// 处理 Related
		if event.Related != nil {
			eventInfo.Related = &types.ObjectReference{
				Kind:            event.Related.Kind,
				Namespace:       event.Related.Namespace,
				Name:            event.Related.Name,
				UID:             string(event.Related.UID),
				APIVersion:      event.Related.APIVersion,
				ResourceVersion: event.Related.ResourceVersion,
				FieldPath:       event.Related.FieldPath,
			}
		}

		result = append(result, eventInfo)
	}

	return result, nil
}

// Watch 监听 PV 变化
func (p *persistentVolumeOperator) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return p.client.CoreV1().PersistentVolumes().Watch(p.ctx, opts)
}

// UpdateLabels 更新标签
func (p *persistentVolumeOperator) UpdateLabels(name string, labelMap map[string]string) error {
	pv, err := p.Get(name)
	if err != nil {
		return err
	}

	if pv.Labels == nil {
		pv.Labels = make(map[string]string)
	}
	for k, v := range labelMap {
		pv.Labels[k] = v
	}

	_, err = p.Update(pv)
	return err
}

// UpdateAnnotations 更新注解
func (p *persistentVolumeOperator) UpdateAnnotations(name string, annotations map[string]string) error {
	pv, err := p.Get(name)
	if err != nil {
		return err
	}

	if pv.Annotations == nil {
		pv.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		pv.Annotations[k] = v
	}

	_, err = p.Update(pv)
	return err
}

// GetUsage 获取 PV 使用情况
func (p *persistentVolumeOperator) GetUsage(name string) (*types.PVUsageInfo, error) {
	pv, err := p.Get(name)
	if err != nil {
		return nil, err
	}

	response := &types.PVUsageInfo{
		PVName:     name,
		Status:     string(pv.Status.Phase),
		UsedByPods: make([]types.PodRefInfo, 0),
		CanDelete:  true,
	}

	// 获取绑定的 PVC 信息
	if pv.Spec.ClaimRef != nil {
		pvcNamespace := pv.Spec.ClaimRef.Namespace
		pvcName := pv.Spec.ClaimRef.Name

		var pvc *corev1.PersistentVolumeClaim
		if p.useInformer && p.pvcLister != nil {
			pvc, _ = p.pvcLister.PersistentVolumeClaims(pvcNamespace).Get(pvcName)
		} else {
			pvc, _ = p.client.CoreV1().PersistentVolumeClaims(pvcNamespace).Get(p.ctx, pvcName, metav1.GetOptions{})
		}

		if pvc != nil {
			response.ClaimInfo = &types.PVClaimInfo{
				Name:         pvc.Name,
				Namespace:    pvc.Namespace,
				Status:       string(pvc.Status.Phase),
				Capacity:     p.getCapacityString(pvc.Status.Capacity),
				AccessModes:  p.formatAccessModes(pvc.Status.AccessModes),
				StorageClass: p.getStorageClassName(pvc),
				Labels:       pvc.Labels,
				Age:          p.formatAge(pvc.CreationTimestamp.Time),
			}

			// 查找使用该 PVC 的 Pod
			pods := p.findPodsUsingPVC(pvcNamespace, pvcName)
			response.UsedByPods = pods

			if len(pods) > 0 {
				response.CanDelete = false
				response.DeleteReason = fmt.Sprintf("PV被%d个Pod使用中", len(pods))
			}
		}
	}

	if pv.Status.Phase == corev1.VolumeBound {
		response.CanDelete = false
		if response.DeleteReason == "" {
			response.DeleteReason = "PV处于Bound状态"
		}
	}

	return response, nil
}

// getCapacityString 获取容量字符串
func (p *persistentVolumeOperator) getCapacityString(capacity corev1.ResourceList) string {
	if qty, ok := capacity[corev1.ResourceStorage]; ok {
		return qty.String()
	}
	return ""
}

// getStorageClassName 获取 StorageClass 名称
func (p *persistentVolumeOperator) getStorageClassName(pvc *corev1.PersistentVolumeClaim) string {
	if pvc.Spec.StorageClassName != nil {
		return *pvc.Spec.StorageClassName
	}
	return ""
}

// findPodsUsingPVC 查找使用指定 PVC 的 Pod
func (p *persistentVolumeOperator) findPodsUsingPVC(namespace, pvcName string) []types.PodRefInfo {
	var pods []*corev1.Pod

	if p.useInformer && p.podLister != nil {
		pods, _ = p.podLister.Pods(namespace).List(labels.Everything())
	} else {
		podList, err := p.client.CoreV1().Pods(namespace).List(p.ctx, metav1.ListOptions{})
		if err != nil {
			return nil
		}
		pods = make([]*corev1.Pod, len(podList.Items))
		for i := range podList.Items {
			pods[i] = &podList.Items[i]
		}
	}

	result := make([]types.PodRefInfo, 0)
	for _, pod := range pods {
		for _, vol := range pod.Spec.Volumes {
			if vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName == pvcName {
				result = append(result, types.PodRefInfo{
					Name:       pod.Name,
					Namespace:  pod.Namespace,
					NodeName:   pod.Spec.NodeName,
					Status:     string(pod.Status.Phase),
					VolumeName: vol.Name,
				})
				break
			}
		}
	}

	return result
}

// CanDelete 检查是否可以安全删除
func (p *persistentVolumeOperator) CanDelete(name string) (bool, string, error) {
	usage, err := p.GetUsage(name)
	if err != nil {
		return false, "", err
	}

	return usage.CanDelete, usage.DeleteReason, nil
}

// GetByStorageClass 根据 StorageClass 获取 PV 列表
func (p *persistentVolumeOperator) GetByStorageClass(storageClassName string) (*types.ListPersistentVolumeResponse, error) {
	return p.List("", "", "", storageClassName)
}

// GetByStatus 根据状态获取 PV 列表
func (p *persistentVolumeOperator) GetByStatus(status string) (*types.ListPersistentVolumeResponse, error) {
	return p.List("", "", status, "")
}
