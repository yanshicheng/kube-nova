package operator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/yaml"
)

type pvcOperator struct {
	BaseOperator
	client          kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	pvcLister       corev1lister.PersistentVolumeClaimLister
	podLister       corev1lister.PodLister
	pvLister        corev1lister.PersistentVolumeLister
}

// NewPVCOperator 创建 PVC 操作器（不使用 informer）
func NewPVCOperator(ctx context.Context, client kubernetes.Interface) types.PVCOperator {
	return &pvcOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

// NewPVCOperatorWithInformer 创建 PVC 操作器（使用 informer）
func NewPVCOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.PVCOperator {
	var pvcLister corev1lister.PersistentVolumeClaimLister
	var podLister corev1lister.PodLister
	var pvLister corev1lister.PersistentVolumeLister

	if informerFactory != nil {
		pvcLister = informerFactory.Core().V1().PersistentVolumeClaims().Lister()
		podLister = informerFactory.Core().V1().Pods().Lister()
		pvLister = informerFactory.Core().V1().PersistentVolumes().Lister()
	}

	return &pvcOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		pvcLister:       pvcLister,
		podLister:       podLister,
		pvLister:        pvLister,
	}
}

// Create
func (p *pvcOperator) Create(pvc *corev1.PersistentVolumeClaim) error {
	if pvc == nil {
		return fmt.Errorf("PVC对象不能为空")
	}
	// 判断是否已经存在
	if _, err := p.Get(pvc.Namespace, pvc.Name); err == nil {
		return fmt.Errorf("PVC %s/%s 已经存在", pvc.Namespace, pvc.Name)
	}
	injectCommonAnnotations(pvc)
	_, err := p.client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(p.ctx, pvc, metav1.CreateOptions{})
	return err
}

// Get 获取指定的 PVC
func (p *pvcOperator) Get(namespace, name string) (*corev1.PersistentVolumeClaim, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	// 优先使用 informer
	if p.useInformer && p.pvcLister != nil {
		pvc, err := p.pvcLister.PersistentVolumeClaims(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("PVC %s/%s 不存在", namespace, name)
			}
			// informer 出错时回退到 API 调用
			pvc, apiErr := p.client.CoreV1().PersistentVolumeClaims(namespace).Get(p.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取PVC失败: %v", apiErr)
			}
			p.injectTypeMeta(pvc)
			return pvc, nil
		}
		// 返回副本以避免修改缓存
		result := pvc.DeepCopy()
		p.injectTypeMeta(result)
		return result, nil
	}

	// 使用 API 调用
	pvc, err := p.client.CoreV1().PersistentVolumeClaims(namespace).Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("PVC %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取PVC失败: %v", err)
	}
	p.injectTypeMeta(pvc)
	return pvc, nil
}

// injectTypeMeta 注入 TypeMeta
func (p *pvcOperator) injectTypeMeta(pvc *corev1.PersistentVolumeClaim) {
	pvc.TypeMeta = metav1.TypeMeta{
		Kind:       "PersistentVolumeClaim",
		APIVersion: "v1",
	}
}

// List 获取 PVC 列表，支持 search 和 labelSelector 过滤
func (p *pvcOperator) List(namespace string, search string, labelSelector string) (*types.ListPVCResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败: %v", err)
		}
		selector = parsedSelector
	}

	var pvcs []*corev1.PersistentVolumeClaim
	var err error

	// 优先使用 informer
	if p.useInformer && p.pvcLister != nil {
		pvcs, err = p.pvcLister.PersistentVolumeClaims(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取PVC列表失败: %v", err)
		}
	} else {
		// 使用 API 调用
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		pvcList, err := p.client.CoreV1().PersistentVolumeClaims(namespace).List(p.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取PVC列表失败: %v", err)
		}
		pvcs = make([]*corev1.PersistentVolumeClaim, len(pvcList.Items))
		for i := range pvcList.Items {
			pvcs[i] = &pvcList.Items[i]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*corev1.PersistentVolumeClaim, 0)
		searchLower := strings.ToLower(search)
		for _, pvc := range pvcs {
			if strings.Contains(strings.ToLower(pvc.Name), searchLower) {
				filtered = append(filtered, pvc)
			}
		}
		pvcs = filtered
	}

	// 转换为响应格式
	items := make([]types.PVCInfo, len(pvcs))
	for i, pvc := range pvcs {
		// 获取容量
		capacity := ""
		if pvc.Status.Capacity != nil {
			if storage, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
				capacity = storage.String()
			}
		}

		// 获取存储类
		storageClass := ""
		if pvc.Spec.StorageClassName != nil {
			storageClass = *pvc.Spec.StorageClassName
		}

		// 获取绑定的 PV
		volume := ""
		if pvc.Spec.VolumeName != "" {
			volume = pvc.Spec.VolumeName
		}

		items[i] = types.PVCInfo{
			Name:              pvc.Name,
			Namespace:         pvc.Namespace,
			Status:            pvc.Status.Phase,
			Volume:            volume,
			Capacity:          capacity,
			AccessModes:       pvc.Spec.AccessModes,
			StorageClass:      storageClass,
			VolumeMode:        pvc.Spec.VolumeMode,
			Labels:            pvc.Labels,
			Annotations:       pvc.Annotations,
			Age:               p.formatAge(pvc.CreationTimestamp.Time),
			CreationTimestamp: pvc.CreationTimestamp.UnixMilli(),
		}
	}

	return &types.ListPVCResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// GetYaml 获取 PVC 的 YAML 表示
func (p *pvcOperator) GetYaml(namespace, name string) (string, error) {
	pvc, err := p.Get(namespace, name)
	if err != nil {
		return "", err
	}

	// 清除 managedFields
	pvc.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(pvc)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

// Update 更新 PVC
func (p *pvcOperator) Update(namespace, name string, pvc *corev1.PersistentVolumeClaim) error {

	// 验证名称和命名空间
	if pvc.Name != name {
		return fmt.Errorf("YAML中的名称(%s)与参数名称(%s)不匹配", pvc.Name, name)
	}
	if pvc.Namespace != "" && pvc.Namespace != namespace {
		return fmt.Errorf("YAML中的命名空间(%s)与参数命名空间(%s)不匹配", pvc.Namespace, namespace)
	}
	pvc.Namespace = namespace

	// 获取现有的 PVC 以保留 ResourceVersion
	existing, err := p.Get(namespace, name)
	if err != nil {
		return fmt.Errorf("获取现有PVC失败: %v", err)
	}

	pvc.ResourceVersion = existing.ResourceVersion

	// 更新
	_, err = p.client.CoreV1().PersistentVolumeClaims(namespace).Update(p.ctx, pvc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("更新PVC失败: %v", err)
	}

	return nil
}

// Describe 获取 PVC 详情
func (p *pvcOperator) Describe(namespace, name string) (string, error) {
	pvc, err := p.Get(namespace, name)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Name:          %s\n", pvc.Name))
	sb.WriteString(fmt.Sprintf("Namespace:     %s\n", pvc.Namespace))

	// StorageClass
	if pvc.Spec.StorageClassName != nil {
		sb.WriteString(fmt.Sprintf("StorageClass:  %s\n", *pvc.Spec.StorageClassName))
	} else {
		sb.WriteString("StorageClass:  <none>\n")
	}

	// Status
	sb.WriteString(fmt.Sprintf("Status:        %s\n", pvc.Status.Phase))

	// Volume
	if pvc.Spec.VolumeName != "" {
		sb.WriteString(fmt.Sprintf("Volume:        %s\n", pvc.Spec.VolumeName))
	} else {
		sb.WriteString("Volume:        <none>\n")
	}

	// Labels
	if len(pvc.Labels) > 0 {
		sb.WriteString("Labels:        ")
		labels := make([]string, 0, len(pvc.Labels))
		for k, v := range pvc.Labels {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}
		sb.WriteString(strings.Join(labels, "\n               ") + "\n")
	} else {
		sb.WriteString("Labels:        <none>\n")
	}

	// Annotations
	if len(pvc.Annotations) > 0 {
		sb.WriteString("Annotations:   ")
		annotations := make([]string, 0, len(pvc.Annotations))
		for k, v := range pvc.Annotations {
			if len(v) > 50 {
				v = v[:50] + "..."
			}
			annotations = append(annotations, fmt.Sprintf("%s=%s", k, v))
		}
		sb.WriteString(strings.Join(annotations, "\n               ") + "\n")
	} else {
		sb.WriteString("Annotations:   <none>\n")
	}

	// Finalizers
	if len(pvc.Finalizers) > 0 {
		sb.WriteString(fmt.Sprintf("Finalizers:    [%s]\n", strings.Join(pvc.Finalizers, " ")))
	} else {
		sb.WriteString("Finalizers:    <none>\n")
	}

	// Capacity
	if pvc.Status.Capacity != nil {
		if storage, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			sb.WriteString(fmt.Sprintf("Capacity:      %s\n", storage.String()))
		}
	} else {
		sb.WriteString("Capacity:      <none>\n")
	}

	// Access Modes
	if len(pvc.Spec.AccessModes) > 0 {
		modes := p.formatAccessModes(pvc.Spec.AccessModes)
		sb.WriteString(fmt.Sprintf("Access Modes:  %s\n", modes))
	} else {
		sb.WriteString("Access Modes:  <none>\n")
	}

	// VolumeMode
	if pvc.Spec.VolumeMode != nil {
		sb.WriteString(fmt.Sprintf("VolumeMode:    %s\n", *pvc.Spec.VolumeMode))
	} else {
		sb.WriteString("VolumeMode:    Filesystem\n")
	}

	// Requested Storage
	requestStorage := p.getRequestStorage(pvc)
	if requestStorage != "" {
		sb.WriteString(fmt.Sprintf("Requested:     %s\n", requestStorage))
	}

	// Selector
	if pvc.Spec.Selector != nil {
		sb.WriteString("Selector:      ")
		if len(pvc.Spec.Selector.MatchLabels) > 0 {
			labels := make([]string, 0, len(pvc.Spec.Selector.MatchLabels))
			for k, v := range pvc.Spec.Selector.MatchLabels {
				labels = append(labels, fmt.Sprintf("%s=%s", k, v))
			}
			sb.WriteString(strings.Join(labels, ",") + "\n")
		} else {
			sb.WriteString("<none>\n")
		}
	} else {
		sb.WriteString("Selector:      <none>\n")
	}

	// Events
	events, err := p.getEvents(namespace, name)
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
		sb.WriteString("Events:        <none>\n")
	}

	return sb.String(), nil
}

// Delete 删除 PVC
func (p *pvcOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := p.client.CoreV1().PersistentVolumeClaims(namespace).Delete(p.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除PVC失败: %v", err)
	}

	return nil
}

// GetAssociation 获取关联信息
func (p *pvcOperator) GetAssociation(namespace, name string) (*types.PVCAssociation, error) {
	// 验证 PVC 存在
	pvc, err := p.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	association := &types.PVCAssociation{
		PVCName:   name,
		Namespace: namespace,
		PVName:    pvc.Spec.VolumeName,
		Pods:      make([]string, 0),
	}

	// 获取使用该 PVC 的 Pods
	var pods []*corev1.Pod
	if p.useInformer && p.podLister != nil {
		pods, err = p.podLister.Pods(namespace).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("获取Pod列表失败: %v", err)
		}
	} else {
		podList, err := p.client.CoreV1().Pods(namespace).List(p.ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取Pod列表失败: %v", err)
		}
		pods = make([]*corev1.Pod, len(podList.Items))
		for i := range podList.Items {
			pods[i] = &podList.Items[i]
		}
	}

	for _, pod := range pods {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == name {
				association.Pods = append(association.Pods, pod.Name)
				break
			}
		}
	}
	association.PodCount = len(association.Pods)

	return association, nil
}

// getRequestStorage 获取请求的存储大小
func (p *pvcOperator) getRequestStorage(pvc *corev1.PersistentVolumeClaim) string {
	if pvc.Spec.Resources.Requests != nil {
		if storage, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			return storage.String()
		}
	}
	return ""
}

// parseStorageSize 解析存储大小字符串为可读格式
func (p *pvcOperator) parseStorageSize(sizeStr string) string {
	quantity, err := resource.ParseQuantity(sizeStr)
	if err != nil {
		return sizeStr
	}
	return quantity.String()
}

// formatAge 格式化时间
func (p *pvcOperator) formatAge(t time.Time) string {
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

// formatAccessModes 格式化访问模式
func (p *pvcOperator) formatAccessModes(modes []corev1.PersistentVolumeAccessMode) string {
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

// getEvents 获取事件
func (p *pvcOperator) getEvents(namespace, name string) ([]types.EventInfo, error) {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=PersistentVolumeClaim", name)
	events, err := p.client.CoreV1().Events(namespace).List(p.ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, err
	}

	result := make([]types.EventInfo, 0, len(events.Items))
	for _, event := range events.Items {
		result = append(result, types.ConvertK8sEventToEventInfo(&event))
	}

	return result, nil
}
