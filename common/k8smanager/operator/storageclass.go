package operator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	storagev1lister "k8s.io/client-go/listers/storage/v1"
	"sigs.k8s.io/yaml"
)

const (
	// DefaultStorageClassAnnotation 默认 StorageClass 注解
	DefaultStorageClassAnnotation = "storageclass.kubernetes.io/is-default-class"
	// BetaDefaultStorageClassAnnotation 旧版默认 StorageClass 注解
	BetaDefaultStorageClassAnnotation = "storageclass.beta.kubernetes.io/is-default-class"
)

type storageClassOperator struct {
	BaseOperator
	client             kubernetes.Interface
	informerFactory    informers.SharedInformerFactory
	storageClassLister storagev1lister.StorageClassLister
	pvLister           corev1lister.PersistentVolumeLister
}

// NewStorageClassOperator 创建 StorageClass 操作器（不使用 informer）
func NewStorageClassOperator(ctx context.Context, client kubernetes.Interface) types.StorageClassOperator {
	return &storageClassOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

// NewStorageClassOperatorWithInformer 创建 StorageClass 操作器（使用 informer）
func NewStorageClassOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.StorageClassOperator {
	var storageClassLister storagev1lister.StorageClassLister
	var pvLister corev1lister.PersistentVolumeLister

	if informerFactory != nil {
		storageClassLister = informerFactory.Storage().V1().StorageClasses().Lister()
		pvLister = informerFactory.Core().V1().PersistentVolumes().Lister()
	}

	return &storageClassOperator{
		BaseOperator:       NewBaseOperator(ctx, informerFactory != nil),
		client:             client,
		informerFactory:    informerFactory,
		storageClassLister: storageClassLister,
		pvLister:           pvLister,
	}
}

// Create 创建 StorageClass
func (s *storageClassOperator) Create(sc *storagev1.StorageClass) (*storagev1.StorageClass, error) {
	if sc == nil || sc.Name == "" {
		return nil, fmt.Errorf("StorageClass对象和名称不能为空")
	}

	created, err := s.client.StorageV1().StorageClasses().Create(s.ctx, sc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建StorageClass失败: %v", err)
	}

	return created, nil
}

// Get 获取 StorageClass
func (s *storageClassOperator) Get(name string) (*storagev1.StorageClass, error) {
	if name == "" {
		return nil, fmt.Errorf("名称不能为空")
	}

	// 优先使用 informer
	if s.useInformer && s.storageClassLister != nil {
		sc, err := s.storageClassLister.Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("StorageClass %s 不存在", name)
			}
			// informer 出错时回退到 API 调用
			sc, apiErr := s.client.StorageV1().StorageClasses().Get(s.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取StorageClass失败: %v", apiErr)
			}
			s.injectTypeMeta(sc)
			return sc, nil
		}
		// 返回副本以避免修改缓存
		result := sc.DeepCopy()
		s.injectTypeMeta(result)
		return result, nil
	}

	// 使用 API 调用
	sc, err := s.client.StorageV1().StorageClasses().Get(s.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("StorageClass %s 不存在", name)
		}
		return nil, fmt.Errorf("获取StorageClass失败: %v", err)
	}

	s.injectTypeMeta(sc)
	return sc, nil
}

// injectTypeMeta 注入 TypeMeta
func (s *storageClassOperator) injectTypeMeta(sc *storagev1.StorageClass) {
	sc.TypeMeta = metav1.TypeMeta{
		Kind:       "StorageClass",
		APIVersion: "storage.k8s.io/v1",
	}
}

// Update 更新 StorageClass
func (s *storageClassOperator) Update(sc *storagev1.StorageClass) (*storagev1.StorageClass, error) {
	if sc == nil || sc.Name == "" {
		return nil, fmt.Errorf("StorageClass对象和名称不能为空")
	}

	updated, err := s.client.StorageV1().StorageClasses().Update(s.ctx, sc, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新StorageClass失败: %v", err)
	}

	return updated, nil
}

// Delete 删除 StorageClass
func (s *storageClassOperator) Delete(name string) error {
	if name == "" {
		return fmt.Errorf("名称不能为空")
	}

	err := s.client.StorageV1().StorageClasses().Delete(s.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除StorageClass失败: %v", err)
	}

	return nil
}

// List 获取 StorageClass 列表
func (s *storageClassOperator) List(search string, labelSelector string) (*types.ListStorageClassResponse, error) {
	var selector labels.Selector = labels.Everything()
	if labelSelector != "" {
		parsedSelector, err := labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败: %v", err)
		}
		selector = parsedSelector
	}

	var storageClasses []*storagev1.StorageClass
	var err error

	// 优先使用 informer
	if s.useInformer && s.storageClassLister != nil {
		storageClasses, err = s.storageClassLister.List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取StorageClass列表失败: %v", err)
		}
	} else {
		// 使用 API 调用
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		scList, err := s.client.StorageV1().StorageClasses().List(s.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取StorageClass列表失败: %v", err)
		}
		storageClasses = make([]*storagev1.StorageClass, len(scList.Items))
		for i := range scList.Items {
			storageClasses[i] = &scList.Items[i]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*storagev1.StorageClass, 0)
		searchLower := strings.ToLower(search)
		for _, sc := range storageClasses {
			if strings.Contains(strings.ToLower(sc.Name), searchLower) ||
				strings.Contains(strings.ToLower(sc.Provisioner), searchLower) {
				filtered = append(filtered, sc)
			}
		}
		storageClasses = filtered
	}

	// 转换为响应格式
	items := make([]types.StorageClassInfo, len(storageClasses))
	for i, sc := range storageClasses {
		items[i] = s.toStorageClassInfo(sc)
	}

	// 按名称排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return &types.ListStorageClassResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// toStorageClassInfo 转换为 StorageClassInfo
func (s *storageClassOperator) toStorageClassInfo(sc *storagev1.StorageClass) types.StorageClassInfo {
	reclaimPolicy := "Delete"
	if sc.ReclaimPolicy != nil {
		reclaimPolicy = string(*sc.ReclaimPolicy)
	}

	volumeBindingMode := "Immediate"
	if sc.VolumeBindingMode != nil {
		volumeBindingMode = string(*sc.VolumeBindingMode)
	}

	allowExpansion := false
	if sc.AllowVolumeExpansion != nil {
		allowExpansion = *sc.AllowVolumeExpansion
	}

	return types.StorageClassInfo{
		Name:                 sc.Name,
		Provisioner:          sc.Provisioner,
		ReclaimPolicy:        reclaimPolicy,
		VolumeBindingMode:    volumeBindingMode,
		AllowVolumeExpansion: allowExpansion,
		IsDefault:            s.isDefaultStorageClass(sc),
		Parameters:           sc.Parameters,
		MountOptions:         sc.MountOptions,
		Labels:               sc.Labels,
		Annotations:          sc.Annotations,
		Age:                  s.formatAge(sc.CreationTimestamp.Time),
		CreationTimestamp:    sc.CreationTimestamp.UnixMilli(),
	}
}

// isDefaultStorageClass 判断是否为默认 StorageClass
func (s *storageClassOperator) isDefaultStorageClass(sc *storagev1.StorageClass) bool {
	if sc.Annotations == nil {
		return false
	}
	if val, ok := sc.Annotations[DefaultStorageClassAnnotation]; ok && val == "true" {
		return true
	}
	if val, ok := sc.Annotations[BetaDefaultStorageClassAnnotation]; ok && val == "true" {
		return true
	}
	return false
}

// formatAge 格式化时间
func (s *storageClassOperator) formatAge(t time.Time) string {
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

// GetYaml 获取 StorageClass 的 YAML
func (s *storageClassOperator) GetYaml(name string) (string, error) {
	sc, err := s.Get(name)
	if err != nil {
		return "", err
	}

	// 清理 ManagedFields
	sc.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(sc)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

// Describe 获取 StorageClass 详情
func (s *storageClassOperator) Describe(name string) (string, error) {
	sc, err := s.Get(name)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Name:                  %s\n", sc.Name))
	sb.WriteString(fmt.Sprintf("IsDefaultClass:        %v\n", s.isDefaultStorageClass(sc)))
	sb.WriteString(fmt.Sprintf("Annotations:           %s\n", s.formatAnnotations(sc.Annotations)))
	sb.WriteString(fmt.Sprintf("Provisioner:           %s\n", sc.Provisioner))

	// Parameters
	if len(sc.Parameters) > 0 {
		sb.WriteString("Parameters:            ")
		params := make([]string, 0, len(sc.Parameters))
		for k, v := range sc.Parameters {
			params = append(params, fmt.Sprintf("%s=%s", k, v))
		}
		sb.WriteString(strings.Join(params, ",") + "\n")
	} else {
		sb.WriteString("Parameters:            <none>\n")
	}

	// AllowVolumeExpansion
	if sc.AllowVolumeExpansion != nil {
		sb.WriteString(fmt.Sprintf("AllowVolumeExpansion:  %v\n", *sc.AllowVolumeExpansion))
	} else {
		sb.WriteString("AllowVolumeExpansion:  <unset>\n")
	}

	// MountOptions
	if len(sc.MountOptions) > 0 {
		sb.WriteString(fmt.Sprintf("MountOptions:          %s\n", strings.Join(sc.MountOptions, ",")))
	} else {
		sb.WriteString("MountOptions:          <none>\n")
	}

	// ReclaimPolicy
	if sc.ReclaimPolicy != nil {
		sb.WriteString(fmt.Sprintf("ReclaimPolicy:         %s\n", *sc.ReclaimPolicy))
	} else {
		sb.WriteString("ReclaimPolicy:         Delete\n")
	}

	// VolumeBindingMode
	if sc.VolumeBindingMode != nil {
		sb.WriteString(fmt.Sprintf("VolumeBindingMode:     %s\n", *sc.VolumeBindingMode))
	} else {
		sb.WriteString("VolumeBindingMode:     Immediate\n")
	}

	// AllowedTopologies
	if len(sc.AllowedTopologies) > 0 {
		sb.WriteString("AllowedTopologies:     \n")
		for _, topology := range sc.AllowedTopologies {
			for _, expr := range topology.MatchLabelExpressions {
				sb.WriteString(fmt.Sprintf("                       %s in %v\n", expr.Key, expr.Values))
			}
		}
	} else {
		sb.WriteString("AllowedTopologies:     <none>\n")
	}

	// Events
	events, err := s.getEvents(sc.Name)
	if err == nil && len(events) > 0 {
		sb.WriteString("Events:\n")
		sb.WriteString("  Type    Reason  Age  From  Message\n")
		sb.WriteString("  ----    ------  ---  ----  -------\n")
		for _, event := range events {
			age := s.formatAge(time.UnixMilli(event.LastTimestamp))
			sb.WriteString(fmt.Sprintf("  %s  %s  %s  %s  %s\n",
				event.Type, event.Reason, age, event.Source, event.Message))
		}
	} else {
		sb.WriteString("Events:                <none>\n")
	}

	return sb.String(), nil
}

// formatAnnotations 格式化注解
func (s *storageClassOperator) formatAnnotations(annotations map[string]string) string {
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
func (s *storageClassOperator) getEvents(name string) ([]types.EventInfo, error) {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=StorageClass", name)
	events, err := s.client.CoreV1().Events("").List(s.ctx, metav1.ListOptions{
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

// Watch 监听 StorageClass 变化
func (s *storageClassOperator) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.client.StorageV1().StorageClasses().Watch(s.ctx, opts)
}

// UpdateLabels 更新标签
func (s *storageClassOperator) UpdateLabels(name string, labelMap map[string]string) error {
	sc, err := s.Get(name)
	if err != nil {
		return err
	}

	if sc.Labels == nil {
		sc.Labels = make(map[string]string)
	}
	for k, v := range labelMap {
		sc.Labels[k] = v
	}

	_, err = s.Update(sc)
	return err
}

// UpdateAnnotations 更新注解
func (s *storageClassOperator) UpdateAnnotations(name string, annotations map[string]string) error {
	sc, err := s.Get(name)
	if err != nil {
		return err
	}

	if sc.Annotations == nil {
		sc.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		sc.Annotations[k] = v
	}

	_, err = s.Update(sc)
	return err
}

// SetDefault 设置为默认 StorageClass
func (s *storageClassOperator) SetDefault(name string) error {
	// 首先取消其他默认 StorageClass
	scList, err := s.client.StorageV1().StorageClasses().List(s.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("获取StorageClass列表失败: %v", err)
	}

	for _, sc := range scList.Items {
		if s.isDefaultStorageClass(&sc) && sc.Name != name {
			if err := s.UnsetDefault(sc.Name); err != nil {
				return fmt.Errorf("取消默认StorageClass %s 失败: %v", sc.Name, err)
			}
		}
	}

	// 设置新的默认 StorageClass
	sc, err := s.Get(name)
	if err != nil {
		return err
	}

	if sc.Annotations == nil {
		sc.Annotations = make(map[string]string)
	}
	sc.Annotations[DefaultStorageClassAnnotation] = "true"

	_, err = s.Update(sc)
	return err
}

// UnsetDefault 取消默认 StorageClass
func (s *storageClassOperator) UnsetDefault(name string) error {
	sc, err := s.Get(name)
	if err != nil {
		return err
	}

	if sc.Annotations == nil {
		return nil
	}

	delete(sc.Annotations, DefaultStorageClassAnnotation)
	delete(sc.Annotations, BetaDefaultStorageClassAnnotation)

	_, err = s.Update(sc)
	return err
}

// GetAssociatedPVs 获取关联的 PV 列表
func (s *storageClassOperator) GetAssociatedPVs(name string) (*types.StorageClassPVResponse, error) {
	// 验证 StorageClass 存在
	_, err := s.Get(name)
	if err != nil {
		return nil, err
	}

	var pvList []*corev1.PersistentVolume

	// 获取所有 PV
	if s.useInformer && s.pvLister != nil {
		pvList, err = s.pvLister.List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("获取PV列表失败: %v", err)
		}
	} else {
		list, err := s.client.CoreV1().PersistentVolumes().List(s.ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取PV列表失败: %v", err)
		}
		pvList = make([]*corev1.PersistentVolume, len(list.Items))
		for i := range list.Items {
			pvList[i] = &list.Items[i]
		}
	}

	// 过滤属于该 StorageClass 的 PV
	response := &types.StorageClassPVResponse{
		StorageClassName: name,
		PVs:              make([]types.StorageClassPVInfo, 0),
	}

	var totalCapacity resource.Quantity
	boundCount := 0
	availableCount := 0

	for _, pv := range pvList {
		if pv.Spec.StorageClassName == name {
			claim := ""
			if pv.Spec.ClaimRef != nil {
				claim = fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
			}

			capacity := ""
			if qty, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
				capacity = qty.String()
				totalCapacity.Add(qty)
			}

			pvInfo := types.StorageClassPVInfo{
				Name:        pv.Name,
				Capacity:    capacity,
				AccessModes: s.formatAccessModes(pv.Spec.AccessModes),
				Status:      string(pv.Status.Phase),
				Claim:       claim,
				Age:         s.formatAge(pv.CreationTimestamp.Time),
			}
			response.PVs = append(response.PVs, pvInfo)

			switch pv.Status.Phase {
			case corev1.VolumeBound:
				boundCount++
			case corev1.VolumeAvailable:
				availableCount++
			}
		}
	}

	response.PVCount = len(response.PVs)
	response.BoundPVCount = boundCount
	response.AvailablePVCount = availableCount
	response.TotalCapacity = totalCapacity.String()

	// 按名称排序
	sort.Slice(response.PVs, func(i, j int) bool {
		return response.PVs[i].Name < response.PVs[j].Name
	})

	return response, nil
}

// formatAccessModes 格式化访问模式
func (s *storageClassOperator) formatAccessModes(modes []corev1.PersistentVolumeAccessMode) string {
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

// CanDelete 检查是否可以安全删除
func (s *storageClassOperator) CanDelete(name string) (bool, string, error) {
	pvResponse, err := s.GetAssociatedPVs(name)
	if err != nil {
		return false, "", err
	}

	if pvResponse.PVCount > 0 {
		return false, fmt.Sprintf("StorageClass被%d个PV使用，删除可能导致这些PV无法正常工作", pvResponse.PVCount), nil
	}

	return true, "", nil
}
