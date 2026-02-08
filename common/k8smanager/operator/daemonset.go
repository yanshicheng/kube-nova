package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	"github.com/zeromicro/go-zero/core/logx"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/yaml"
)

const (
	AnnotationNodeSelector = "ikubeops.com/node-selector"
)

type daemonSetOperator struct {
	BaseOperator
	client            kubernetes.Interface
	informerFactory   informers.SharedInformerFactory
	daemonSetLister   v1.DaemonSetLister
	daemonSetInformer cache.SharedIndexInformer
}

func NewDaemonSetOperator(ctx context.Context, client kubernetes.Interface) types.DaemonSetOperator {
	return &daemonSetOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

func NewDaemonSetOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.DaemonSetOperator {
	var daemonSetLister v1.DaemonSetLister
	var daemonSetInformer cache.SharedIndexInformer

	if informerFactory != nil {
		daemonSetInformer = informerFactory.Apps().V1().DaemonSets().Informer()
		daemonSetLister = informerFactory.Apps().V1().DaemonSets().Lister()
	}

	return &daemonSetOperator{
		BaseOperator:      NewBaseOperator(ctx, informerFactory != nil),
		client:            client,
		informerFactory:   informerFactory,
		daemonSetLister:   daemonSetLister,
		daemonSetInformer: daemonSetInformer,
	}
}

func (d *daemonSetOperator) Create(daemonSet *appsv1.DaemonSet) (*appsv1.DaemonSet, error) {
	if daemonSet == nil || daemonSet.Name == "" || daemonSet.Namespace == "" {
		return nil, fmt.Errorf("DaemonSet对象、名称和命名空间不能为空")
	}
	injectCommonAnnotations(daemonSet)
	if daemonSet.Labels == nil {
		daemonSet.Labels = make(map[string]string)
	}
	if daemonSet.Annotations == nil {
		daemonSet.Annotations = make(map[string]string)
	}

	created, err := d.client.AppsV1().DaemonSets(daemonSet.Namespace).Create(d.ctx, daemonSet, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建DaemonSet失败: %v", err)
	}

	return created, nil
}

func (d *daemonSetOperator) Get(namespace, name string) (*appsv1.DaemonSet, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	if d.daemonSetLister != nil {
		daemonSet, err := d.daemonSetLister.DaemonSets(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("DaemonSet %s/%s 不存在", namespace, name)
			}
			daemonSet, apiErr := d.client.AppsV1().DaemonSets(namespace).Get(d.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取DaemonSet失败")
			}
			return daemonSet, nil
		}
		return daemonSet, nil
	}

	daemonSet, err := d.client.AppsV1().DaemonSets(namespace).Get(d.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("DaemonSet %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取DaemonSet失败")
	}

	return daemonSet, nil
}

func (d *daemonSetOperator) Update(daemonSet *appsv1.DaemonSet) (*appsv1.DaemonSet, error) {
	if daemonSet == nil || daemonSet.Name == "" || daemonSet.Namespace == "" {
		return nil, fmt.Errorf("DaemonSet对象、名称和命名空间不能为空")
	}

	updated, err := d.client.AppsV1().DaemonSets(daemonSet.Namespace).Update(d.ctx, daemonSet, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新DaemonSet失败: %v", err)
	}

	return updated, nil
}

func (d *daemonSetOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := d.client.AppsV1().DaemonSets(namespace).Delete(d.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除DaemonSet失败: %v", err)
	}

	return nil
}

func (d *daemonSetOperator) List(namespace string, req types.ListRequest) (*types.ListDaemonSetResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	if req.SortBy == "" {
		req.SortBy = "name"
	}

	var selector labels.Selector = labels.Everything()
	if req.Labels != "" {
		parsedSelector, err := labels.Parse(req.Labels)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败")
		}
		selector = parsedSelector
	}

	var daemonSets []*appsv1.DaemonSet
	var err error

	if d.useInformer && d.daemonSetLister != nil {
		daemonSets, err = d.daemonSetLister.DaemonSets(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取DaemonSet列表失败")
		}
	} else {
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		daemonSetList, err := d.client.AppsV1().DaemonSets(namespace).List(d.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取DaemonSet列表失败")
		}
		daemonSets = make([]*appsv1.DaemonSet, len(daemonSetList.Items))
		for i := range daemonSetList.Items {
			daemonSets[i] = &daemonSetList.Items[i]
		}
	}

	if req.Search != "" {
		filtered := make([]*appsv1.DaemonSet, 0)
		searchLower := strings.ToLower(req.Search)
		for _, ds := range daemonSets {
			if strings.Contains(strings.ToLower(ds.Name), searchLower) {
				filtered = append(filtered, ds)
			}
		}
		daemonSets = filtered
	}

	sort.Slice(daemonSets, func(i, j int) bool {
		var less bool
		switch req.SortBy {
		case "creationTime", "creationTimestamp":
			less = daemonSets[i].CreationTimestamp.Before(&daemonSets[j].CreationTimestamp)
		default:
			less = daemonSets[i].Name < daemonSets[j].Name
		}
		if req.SortDesc {
			return !less
		}
		return less
	})

	total := len(daemonSets)
	totalPages := (total + req.PageSize - 1) / req.PageSize
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	if start >= total {
		return &types.ListDaemonSetResponse{
			ListResponse: types.ListResponse{
				Total:      total,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: totalPages,
			},
			Items: []types.DaemonSetInfo{},
		}, nil
	}

	if end > total {
		end = total
	}

	pageDaemonSets := daemonSets[start:end]
	items := make([]types.DaemonSetInfo, len(pageDaemonSets))
	for i, ds := range pageDaemonSets {
		items[i] = d.convertToDaemonSetInfo(ds)
	}

	return &types.ListDaemonSetResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: items,
	}, nil
}

func (d *daemonSetOperator) convertToDaemonSetInfo(ds *appsv1.DaemonSet) types.DaemonSetInfo {
	images := make([]string, 0)
	for _, container := range ds.Spec.Template.Spec.Containers {
		images = append(images, container.Image)
	}

	return types.DaemonSetInfo{
		Name:                   ds.Name,
		Namespace:              ds.Namespace,
		DesiredNumberScheduled: ds.Status.DesiredNumberScheduled,
		CurrentNumberScheduled: ds.Status.CurrentNumberScheduled,
		NumberReady:            ds.Status.NumberReady,
		NumberAvailable:        ds.Status.NumberAvailable,
		CreationTimestamp:      ds.CreationTimestamp.Time,
		Images:                 images,
	}
}

func (d *daemonSetOperator) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return d.client.AppsV1().DaemonSets(namespace).Watch(d.ctx, opts)
}

func (d *daemonSetOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	if daemonSet.Labels == nil {
		daemonSet.Labels = make(map[string]string)
	}
	for k, v := range labels {
		daemonSet.Labels[k] = v
	}

	_, err = d.Update(daemonSet)
	return err
}

func (d *daemonSetOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	if daemonSet.Annotations == nil {
		daemonSet.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		daemonSet.Annotations[k] = v
	}

	_, err = d.Update(daemonSet)
	return err
}

func (d *daemonSetOperator) GetYaml(namespace, name string) (string, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return "", err
	}

	daemonSet.TypeMeta = metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "DaemonSet",
	}
	daemonSet.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(daemonSet)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

func (d *daemonSetOperator) GetPods(namespace, name string) ([]types.PodDetailInfo, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	labelSelector := metav1.FormatLabelSelector(daemonSet.Spec.Selector)
	podList, err := d.client.CoreV1().Pods(namespace).List(d.ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("获取Pod列表失败: %v", err)
	}

	pods := make([]types.PodDetailInfo, 0, len(podList.Items))
	for i := range podList.Items {
		pod := &podList.Items[i]
		pods = append(pods, d.convertToPodDetailInfo(pod))
	}

	return pods, nil
}

func (d *daemonSetOperator) convertToPodDetailInfo(pod *corev1.Pod) types.PodDetailInfo {
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}

	readyCount := 0
	totalCount := len(pod.Status.ContainerStatuses)
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			readyCount++
		}
	}

	age := time.Since(pod.CreationTimestamp.Time).Round(time.Second).String()

	return types.PodDetailInfo{
		Name:         pod.Name,
		Namespace:    pod.Namespace,
		Status:       string(pod.Status.Phase),
		Ready:        fmt.Sprintf("%d/%d", readyCount, totalCount),
		Restarts:     restarts,
		Age:          age,
		Node:         pod.Spec.NodeName,
		PodIP:        pod.Status.PodIP,
		Labels:       pod.Labels,
		CreationTime: pod.CreationTimestamp.UnixMilli(),
	}
}

// ==================== 镜像管理 ====================

// GetContainerImages 获取所有容器的镜像信息
func (d *daemonSetOperator) GetContainerImages(namespace, name string) (*types.ContainerInfoList, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return ConvertPodSpecToContainerImages(&daemonSet.Spec.Template.Spec), nil
}

func (d *daemonSetOperator) UpdateImage(req *types.UpdateImageRequest) error {
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}
	if req.Namespace == "" {
		return fmt.Errorf("命名空间不能为空")
	}
	if req.Name == "" {
		return fmt.Errorf("DaemonSet名称不能为空")
	}
	if req.ContainerName == "" {
		return fmt.Errorf("容器名称不能为空")
	}
	if req.Image == "" {
		return fmt.Errorf("镜像不能为空")
	}

	if err := validateImageFormat(req.Image); err != nil {
		return fmt.Errorf("镜像格式无效: %v", err)
	}

	daemonSet, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	var oldImage string
	found := false

	for i := range daemonSet.Spec.Template.Spec.InitContainers {
		container := &daemonSet.Spec.Template.Spec.InitContainers[i]
		if container.Name == req.ContainerName {
			oldImage = container.Image
			container.Image = req.Image
			found = true
			break
		}
	}

	if !found {
		for i := range daemonSet.Spec.Template.Spec.Containers {
			container := &daemonSet.Spec.Template.Spec.Containers[i]
			if container.Name == req.ContainerName {
				oldImage = container.Image
				container.Image = req.Image
				found = true
				break
			}
		}
	}

	if !found {
		for i := range daemonSet.Spec.Template.Spec.EphemeralContainers {
			container := &daemonSet.Spec.Template.Spec.EphemeralContainers[i]
			if container.Name == req.ContainerName {
				oldImage = container.Image
				container.Image = req.Image
				found = true
				break
			}
		}
	}

	if !found {
		availableContainers := d.getAvailableContainerNames(daemonSet)
		return fmt.Errorf("未找到容器 '%s'，可用容器: %v", req.ContainerName, availableContainers)
	}

	if oldImage == req.Image {
		return nil
	}

	if daemonSet.Annotations == nil {
		daemonSet.Annotations = make(map[string]string)
	}

	changeCause := req.Reason
	if changeCause == "" {
		changeCause = fmt.Sprintf("image updated: %s %s -> %s",
			req.ContainerName, extractImageTag(oldImage), extractImageTag(req.Image))
	}
	daemonSet.Annotations["kubernetes.io/change-cause"] = changeCause

	_, err = d.Update(daemonSet)
	if err != nil {
		return fmt.Errorf("更新DaemonSet失败: %v", err)
	}

	return nil
}

// UpdateImages 批量更新镜像
func (d *daemonSetOperator) UpdateImages(req *types.UpdateImagesRequest) error {
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}
	if req.Namespace == "" {
		return fmt.Errorf("命名空间不能为空")
	}
	if req.Name == "" {
		return fmt.Errorf("DaemonSet名称不能为空")
	}

	if len(req.Containers.Containers) == 0 {
		return fmt.Errorf("未指定要更新的容器")
	}

	for _, c := range req.Containers.Containers {
		if err := validateImageFormat(c.Image); err != nil {
			return fmt.Errorf("容器 '%s' 镜像格式无效: %v", c.ContainerName, err)
		}
	}

	daemonSet, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	changes, err := ApplyImagesToPodSpec(&daemonSet.Spec.Template.Spec, req.Containers.Containers)
	if err != nil {
		return fmt.Errorf("应用镜像更新失败: %v", err)
	}

	if len(changes) == 0 {
		return nil
	}

	if daemonSet.Annotations == nil {
		daemonSet.Annotations = make(map[string]string)
	}

	changeCause := req.Reason
	if changeCause == "" {
		changeCause = fmt.Sprintf("images updated: %s", strings.Join(changes, ", "))
	}
	daemonSet.Annotations["kubernetes.io/change-cause"] = changeCause

	_, err = d.Update(daemonSet)
	if err != nil {
		return fmt.Errorf("更新DaemonSet失败: %v", err)
	}

	return nil
}

func (d *daemonSetOperator) getAvailableContainerNames(daemonSet *appsv1.DaemonSet) []string {
	names := make([]string, 0)
	for _, c := range daemonSet.Spec.Template.Spec.InitContainers {
		names = append(names, c.Name+" (init)")
	}
	for _, c := range daemonSet.Spec.Template.Spec.Containers {
		names = append(names, c.Name)
	}
	for _, c := range daemonSet.Spec.Template.Spec.EphemeralContainers {
		names = append(names, c.Name+" (ephemeral)")
	}
	return names
}

func (d *daemonSetOperator) GetStatus(namespace, name string) (*types.DaemonSetStatusInfo, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return &types.DaemonSetStatusInfo{
		DesiredNumberScheduled: daemonSet.Status.DesiredNumberScheduled,
		CurrentNumberScheduled: daemonSet.Status.CurrentNumberScheduled,
		NumberReady:            daemonSet.Status.NumberReady,
		NumberAvailable:        daemonSet.Status.NumberAvailable,
		NumberMisscheduled:     daemonSet.Status.NumberMisscheduled,
		UpdatedNumberScheduled: daemonSet.Status.UpdatedNumberScheduled,
	}, nil
}

func (d *daemonSetOperator) GetUpdateStrategy(namespace, name string) (*types.UpdateStrategyResponse, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.UpdateStrategyResponse{
		Type: string(daemonSet.Spec.UpdateStrategy.Type),
	}

	if daemonSet.Spec.UpdateStrategy.RollingUpdate != nil {
		rollingUpdate := &types.RollingUpdateConfig{}

		if daemonSet.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable != nil {
			rollingUpdate.MaxUnavailable = daemonSet.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.String()
		}
		if daemonSet.Spec.UpdateStrategy.RollingUpdate.MaxSurge != nil {
			rollingUpdate.MaxSurge = daemonSet.Spec.UpdateStrategy.RollingUpdate.MaxSurge.String()
		}

		response.RollingUpdate = rollingUpdate
	}

	return response, nil
}

func (d *daemonSetOperator) UpdateStrategy(req *types.UpdateStrategyRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	daemonSet, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	daemonSet.Spec.UpdateStrategy.Type = appsv1.DaemonSetUpdateStrategyType(req.Type)

	if req.Type == "RollingUpdate" && req.RollingUpdate != nil {
		if daemonSet.Spec.UpdateStrategy.RollingUpdate == nil {
			daemonSet.Spec.UpdateStrategy.RollingUpdate = &appsv1.RollingUpdateDaemonSet{}
		}

		if req.RollingUpdate.MaxUnavailable != "" {
			maxUnavailable := intstr.Parse(req.RollingUpdate.MaxUnavailable)
			// 验证 MaxUnavailable 值合法
			if maxUnavailable.Type == intstr.Int && maxUnavailable.IntVal < 0 {
				return fmt.Errorf("MaxUnavailable 不能为负数")
			}
			if maxUnavailable.Type == intstr.String && maxUnavailable.StrVal == "0%" {
				return fmt.Errorf("MaxUnavailable 不能为 0%%，这会阻塞更新")
			}
			daemonSet.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = &maxUnavailable
		}

		if req.RollingUpdate.MaxSurge != "" {
			maxSurge := intstr.Parse(req.RollingUpdate.MaxSurge)
			// 验证 MaxSurge 值合法
			if maxSurge.Type == intstr.Int && maxSurge.IntVal < 0 {
				return fmt.Errorf("MaxSurge 不能为负数")
			}
			daemonSet.Spec.UpdateStrategy.RollingUpdate.MaxSurge = &maxSurge
		}
	}

	_, err = d.Update(daemonSet)
	return err
}

func (d *daemonSetOperator) GetRevisions(namespace, name string) ([]types.RevisionInfo, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	revisionList, err := d.client.AppsV1().ControllerRevisions(namespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取ControllerRevision列表失败")
	}

	revisions := make([]types.RevisionInfo, 0)
	for i := range revisionList.Items {
		cr := &revisionList.Items[i]

		isOwned := false
		for _, owner := range cr.OwnerReferences {
			if owner.Kind == "DaemonSet" && owner.Name == daemonSet.Name && owner.UID == daemonSet.UID {
				isOwned = true
				break
			}
		}

		if !isOwned {
			continue
		}

		images := make([]string, 0)
		var revisionData appsv1.DaemonSet
		if err := json.Unmarshal(cr.Data.Raw, &revisionData); err == nil {
			for _, container := range revisionData.Spec.Template.Spec.Containers {
				images = append(images, container.Image)
			}
		}

		revisions = append(revisions, types.RevisionInfo{
			Revision:          cr.Revision,
			CreationTimestamp: cr.CreationTimestamp.UnixMilli(),
			Images:            images,
			Replicas:          0,
			Reason:            "DaemonSet Update",
		})
	}

	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].Revision > revisions[j].Revision
	})

	return revisions, nil
}

func (d *daemonSetOperator) GetConfigHistory(namespace, name string) ([]types.ConfigHistoryInfo, error) {
	return nil, fmt.Errorf("DaemonSet不支持业务层配置历史")
}

func (d *daemonSetOperator) Rollback(req *types.RollbackToRevisionRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	daemonSet, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	revisionList, err := d.client.AppsV1().ControllerRevisions(req.Namespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("获取ControllerRevision列表失败")
	}

	var targetRevision *appsv1.ControllerRevision
	for i := range revisionList.Items {
		cr := &revisionList.Items[i]

		isOwned := false
		for _, owner := range cr.OwnerReferences {
			if owner.Kind == "DaemonSet" && owner.Name == daemonSet.Name && owner.UID == daemonSet.UID {
				isOwned = true
				break
			}
		}

		if !isOwned {
			continue
		}

		if cr.Revision == req.Revision {
			targetRevision = cr
			break
		}
	}

	if targetRevision == nil {
		return fmt.Errorf("未找到目标版本: %d", req.Revision)
	}

	var revisionData appsv1.DaemonSet
	if err := json.Unmarshal(targetRevision.Data.Raw, &revisionData); err != nil {
		return fmt.Errorf("解析历史版本数据失败: %v", err)
	}

	daemonSet.Spec = revisionData.Spec
	_, err = d.Update(daemonSet)
	return err
}

func (d *daemonSetOperator) RollbackToConfig(req *types.RollbackToConfigRequest) error {
	return fmt.Errorf("DaemonSet不支持业务层配置历史回滚")
}

// ==================== 环境变量管理 ====================

// GetEnvVars 获取所有容器的环境变量
func (d *daemonSetOperator) GetEnvVars(namespace, name string) (*types.EnvVarsResponse, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return ConvertPodSpecToEnvVars(&daemonSet.Spec.Template.Spec), nil
}

// UpdateEnvVars 全量更新所有容器的环境变量
func (d *daemonSetOperator) UpdateEnvVars(req *types.UpdateEnvVarsRequest) error {
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}
	if req.Namespace == "" {
		return fmt.Errorf("命名空间不能为空")
	}
	if req.Name == "" {
		return fmt.Errorf("DaemonSet名称不能为空")
	}

	daemonSet, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	if err := ApplyEnvVarsToPodSpec(&daemonSet.Spec.Template.Spec, req.Containers); err != nil {
		return err
	}

	_, err = d.Update(daemonSet)
	return err
}

func (d *daemonSetOperator) GetPauseStatus(namespace, name string) (*types.PauseStatusResponse, error) {
	return &types.PauseStatusResponse{
		Paused:      false,
		SupportType: "none",
	}, nil
}

// ==================== 资源配额管理 ====================

// GetResources 获取所有容器的资源配额
func (d *daemonSetOperator) GetResources(namespace, name string) (*types.ResourcesResponse, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return ConvertPodSpecToResources(&daemonSet.Spec.Template.Spec), nil
}

// UpdateResources 全量更新所有容器的资源配额
func (d *daemonSetOperator) UpdateResources(req *types.UpdateResourcesRequest) error {
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}
	if req.Namespace == "" {
		return fmt.Errorf("命名空间不能为空")
	}
	if req.Name == "" {
		return fmt.Errorf("DaemonSet名称不能为空")
	}

	daemonSet, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	if err := ApplyResourcesToPodSpec(&daemonSet.Spec.Template.Spec, req.Containers); err != nil {
		return err
	}

	_, err = d.Update(daemonSet)
	return err
}

// ==================== 健康检查管理 ====================

// GetProbes 获取所有容器的健康检查配置
func (d *daemonSetOperator) GetProbes(namespace, name string) (*types.ProbesResponse, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return ConvertPodSpecToProbes(&daemonSet.Spec.Template.Spec), nil
}

// UpdateProbes 全量更新所有容器的健康检查配置
func (d *daemonSetOperator) UpdateProbes(req *types.UpdateProbesRequest) error {
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}
	if req.Namespace == "" {
		return fmt.Errorf("命名空间不能为空")
	}
	if req.Name == "" {
		return fmt.Errorf("DaemonSet名称不能为空")
	}

	daemonSet, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	if err := ApplyProbesToPodSpec(&daemonSet.Spec.Template.Spec, req.Containers); err != nil {
		return err
	}

	_, err = d.Update(daemonSet)
	return err
}

func (d *daemonSetOperator) Stop(namespace, name string) error {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	if daemonSet.Annotations == nil {
		daemonSet.Annotations = make(map[string]string)
	}

	if daemonSet.Spec.Template.Spec.NodeSelector != nil {
		nodeSelectorJSON, err := json.Marshal(daemonSet.Spec.Template.Spec.NodeSelector)
		if err != nil {
			return fmt.Errorf("序列化节点选择器失败: %v", err)
		}
		daemonSet.Annotations[AnnotationNodeSelector] = string(nodeSelectorJSON)
	} else {
		daemonSet.Annotations[AnnotationNodeSelector] = "{}"
	}

	if daemonSet.Spec.Template.Spec.NodeSelector == nil {
		daemonSet.Spec.Template.Spec.NodeSelector = make(map[string]string)
	}
	daemonSet.Spec.Template.Spec.NodeSelector["ikubeops.com/stopped"] = "true"

	_, err = d.Update(daemonSet)
	return err
}

func (d *daemonSetOperator) Start(namespace, name string) error {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	if daemonSet.Annotations != nil {
		if nodeSelectorStr, ok := daemonSet.Annotations[AnnotationNodeSelector]; ok {
			var nodeSelector map[string]string
			if err := json.Unmarshal([]byte(nodeSelectorStr), &nodeSelector); err != nil {
				return fmt.Errorf("反序列化节点选择器失败: %v", err)
			}

			if len(nodeSelector) > 0 {
				daemonSet.Spec.Template.Spec.NodeSelector = nodeSelector
			} else {
				daemonSet.Spec.Template.Spec.NodeSelector = nil
			}

			delete(daemonSet.Annotations, AnnotationNodeSelector)
		} else {
			if daemonSet.Spec.Template.Spec.NodeSelector != nil {
				delete(daemonSet.Spec.Template.Spec.NodeSelector, "ikubeops.com/stopped")
				if len(daemonSet.Spec.Template.Spec.NodeSelector) == 0 {
					daemonSet.Spec.Template.Spec.NodeSelector = nil
				}
			}
		}
	}

	_, err = d.Update(daemonSet)
	return err
}

func (d *daemonSetOperator) Restart(namespace, name string) error {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	if daemonSet.Spec.Template.Annotations == nil {
		daemonSet.Spec.Template.Annotations = make(map[string]string)
	}
	daemonSet.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = d.Update(daemonSet)
	return err
}

func (d *daemonSetOperator) GetPodLabels(namespace, name string) (map[string]string, error) {
	var daemonSet *appsv1.DaemonSet
	var err error

	if d.useInformer && d.daemonSetLister != nil {
		daemonSet, err = d.daemonSetLister.DaemonSets(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("DaemonSet %s/%s 不存在", namespace, name)
			}
			daemonSet, err = d.client.AppsV1().DaemonSets(namespace).Get(d.ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取DaemonSet失败")
			}
		}
	} else {
		daemonSet, err = d.client.AppsV1().DaemonSets(namespace).Get(d.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("DaemonSet %s/%s 不存在", namespace, name)
			}
			return nil, fmt.Errorf("获取DaemonSet失败")
		}
	}

	if daemonSet.Spec.Template.Labels == nil {
		return make(map[string]string), nil
	}

	labels := make(map[string]string)
	for k, v := range daemonSet.Spec.Template.Labels {
		labels[k] = v
	}
	return labels, nil
}

func (d *daemonSetOperator) GetPodSelectorLabels(namespace, name string) (map[string]string, error) {
	var daemonSet *appsv1.DaemonSet
	var err error

	if d.useInformer && d.daemonSetLister != nil {
		daemonSet, err = d.daemonSetLister.DaemonSets(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("DaemonSet %s/%s 不存在", namespace, name)
			}
			daemonSet, err = d.client.AppsV1().DaemonSets(namespace).Get(d.ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取DaemonSet失败")
			}
		}
	} else {
		daemonSet, err = d.client.AppsV1().DaemonSets(namespace).Get(d.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("DaemonSet %s/%s 不存在", namespace, name)
			}
			return nil, fmt.Errorf("获取DaemonSet失败")
		}
	}

	if daemonSet.Spec.Selector == nil || daemonSet.Spec.Selector.MatchLabels == nil {
		return make(map[string]string), nil
	}

	labels := make(map[string]string)
	for k, v := range daemonSet.Spec.Selector.MatchLabels {
		labels[k] = v
	}
	return labels, nil
}

func (d *daemonSetOperator) GetVersionStatus(namespace, name string) (*types.ResourceStatus, error) {
	var daemonSet *appsv1.DaemonSet
	var err error

	if d.useInformer && d.daemonSetLister != nil {
		daemonSet, err = d.daemonSetLister.DaemonSets(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("DaemonSet %s/%s 不存在", namespace, name)
			}
			daemonSet, err = d.client.AppsV1().DaemonSets(namespace).Get(d.ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取DaemonSet失败")
			}
		}
	} else {
		daemonSet, err = d.client.AppsV1().DaemonSets(namespace).Get(d.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("DaemonSet %s/%s 不存在", namespace, name)
			}
			return nil, fmt.Errorf("获取DaemonSet失败")
		}
	}

	status := &types.ResourceStatus{
		Ready: false,
	}

	if daemonSet.Spec.Template.Spec.NodeSelector != nil {
		if _, exists := daemonSet.Spec.Template.Spec.NodeSelector["ikubeops.com/stopped"]; exists {
			status.Status = types.StatusStopped
			status.Message = "已通过节点选择器停止"
			status.Ready = true
			return status, nil
		}
	}

	desiredNumber := daemonSet.Status.DesiredNumberScheduled
	currentNumber := daemonSet.Status.CurrentNumberScheduled
	numberReady := daemonSet.Status.NumberReady

	if desiredNumber == 0 {
		status.Status = types.StatusStopped
		status.Message = "没有匹配的节点"
		status.Ready = true
		return status, nil
	}

	if numberReady == 0 && currentNumber == 0 {
		age := time.Since(daemonSet.CreationTimestamp.Time)
		if age < 30*time.Second {
			status.Status = types.StatusCreating
			status.Message = "正在创建 Pod"
			return status, nil
		}
	}

	if numberReady == desiredNumber &&
		daemonSet.Status.NumberAvailable == desiredNumber &&
		daemonSet.Status.UpdatedNumberScheduled == desiredNumber {
		status.Status = types.StatusRunning
		status.Message = fmt.Sprintf("所有 Pod 运行正常 (%d/%d)", numberReady, desiredNumber)
		status.Ready = true
		return status, nil
	}

	if daemonSet.Status.UpdatedNumberScheduled < desiredNumber {
		status.Status = types.StatusRunning
		status.Message = fmt.Sprintf("正在更新中 (已更新: %d/%d, 就绪: %d/%d)",
			daemonSet.Status.UpdatedNumberScheduled, desiredNumber,
			numberReady, desiredNumber)
		return status, nil
	}

	if numberReady < desiredNumber {
		age := time.Since(daemonSet.CreationTimestamp.Time)
		if age > 5*time.Minute {
			status.Status = types.StatusError
			status.Message = fmt.Sprintf("部分 Pod 异常 (就绪: %d/%d)", numberReady, desiredNumber)
			return status, nil
		}
		status.Status = types.StatusCreating
		status.Message = fmt.Sprintf("正在启动 (就绪: %d/%d)", numberReady, desiredNumber)
		return status, nil
	}

	status.Status = types.StatusRunning
	status.Message = "运行中"
	status.Ready = true
	return status, nil
}

// ==================== 调度配置相关 ====================

// GetSchedulingConfig 获取 DaemonSet 调度配置
func (d *daemonSetOperator) GetSchedulingConfig(namespace, name string) (*types.SchedulingConfig, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, fmt.Errorf("获取 DaemonSet 失败: %w", err)
	}

	return types.ConvertPodSpecToSchedulingConfig(&daemonSet.Spec.Template.Spec), nil
}

// UpdateSchedulingConfig 更新 DaemonSet 调度配置
func (d *daemonSetOperator) UpdateSchedulingConfig(namespace, name string, req *types.UpdateSchedulingConfigRequest) error {
	if req == nil {
		return fmt.Errorf("调度配置请求不能为空")
	}

	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return fmt.Errorf("获取 DaemonSet 失败: %w", err)
	}

	if req.NodeName != nil && *req.NodeName != "" {
		logx.Infof("警告: DaemonSet %s/%s 设置 NodeName 将导致只在指定节点运行", namespace, name)
		req.NodeName = nil
	}

	types.ApplySchedulingConfigToPodSpec(&daemonSet.Spec.Template.Spec, req)

	_, err = d.Update(daemonSet)
	if err != nil {
		return fmt.Errorf("更新 DaemonSet 调度配置失败: %w", err)
	}

	return nil
}

// ==================== 存储配置管理 ====================

// GetStorageConfig 获取存储配置
func (d *daemonSetOperator) GetStorageConfig(namespace, name string) (*types.StorageConfig, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return ConvertPodSpecToStorageConfig(&daemonSet.Spec.Template.Spec), nil
}

// UpdateStorageConfig 更新存储配置（全量更新）
func (d *daemonSetOperator) UpdateStorageConfig(namespace, name string, config *types.UpdateStorageConfigRequest) error {
	if config == nil {
		return fmt.Errorf("存储配置不能为空")
	}

	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	// DaemonSet 不支持 VolumeClaimTemplates
	if len(config.VolumeClaimTemplates) > 0 {
		return fmt.Errorf("DaemonSet 不支持 VolumeClaimTemplates")
	}

	if err := ApplyStorageConfigToPodSpec(&daemonSet.Spec.Template.Spec, config); err != nil {
		return err
	}

	_, err = d.Update(daemonSet)
	return err
}

// ==================== Events 相关 ====================

func (d *daemonSetOperator) GetEvents(namespace, name string) ([]types.EventInfo, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	eventList, err := d.client.CoreV1().Events(namespace).List(d.ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=DaemonSet,involvedObject.uid=%s",
			name, daemonSet.UID),
	})
	if err != nil {
		return nil, fmt.Errorf("获取事件列表失败: %v", err)
	}

	events := make([]types.EventInfo, 0, len(eventList.Items))
	for i := range eventList.Items {
		events = append(events, types.ConvertK8sEventToEventInfo(&eventList.Items[i]))
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTimestamp > events[j].LastTimestamp
	})

	return events, nil
}

func (d *daemonSetOperator) GetDescribe(namespace, name string) (string, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return "", err
	}

	var buf strings.Builder

	buf.WriteString(fmt.Sprintf("Name:           %s\n", daemonSet.Name))
	buf.WriteString(fmt.Sprintf("Namespace:      %s\n", daemonSet.Namespace))

	if daemonSet.Spec.Selector != nil {
		buf.WriteString(fmt.Sprintf("Selector:       %s\n", metav1.FormatLabelSelector(daemonSet.Spec.Selector)))
	} else {
		buf.WriteString("Selector:       <none>\n")
	}

	buf.WriteString("Node-Selector:  ")
	if len(daemonSet.Spec.Template.Spec.NodeSelector) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range daemonSet.Spec.Template.Spec.NodeSelector {
			if !first {
				buf.WriteString(",")
			}
			buf.WriteString(fmt.Sprintf("%s=%s", k, v))
			first = false
		}
		buf.WriteString("\n")
	}

	buf.WriteString("Labels:         ")
	if len(daemonSet.Labels) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range daemonSet.Labels {
			if !first {
				buf.WriteString("                ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	}

	buf.WriteString("Annotations:    ")
	if len(daemonSet.Annotations) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range daemonSet.Annotations {
			if !first {
				buf.WriteString("                ")
			}
			buf.WriteString(fmt.Sprintf("%s: %s\n", k, v))
			first = false
		}
	}

	buf.WriteString(fmt.Sprintf("Desired Number of Nodes Scheduled: %d\n", daemonSet.Status.DesiredNumberScheduled))
	buf.WriteString(fmt.Sprintf("Current Number of Nodes Scheduled: %d\n", daemonSet.Status.CurrentNumberScheduled))
	buf.WriteString(fmt.Sprintf("Number of Nodes Scheduled with Up-to-date Pods: %d\n", daemonSet.Status.UpdatedNumberScheduled))
	buf.WriteString(fmt.Sprintf("Number of Nodes Scheduled with Available Pods: %d\n", daemonSet.Status.NumberAvailable))
	buf.WriteString(fmt.Sprintf("Number of Nodes Misscheduled: %d\n", daemonSet.Status.NumberMisscheduled))

	buf.WriteString(fmt.Sprintf("Pods Status:  %d Running / %d Ready / %d Updated / %d Unavailable\n",
		daemonSet.Status.CurrentNumberScheduled,
		daemonSet.Status.NumberReady,
		daemonSet.Status.UpdatedNumberScheduled,
		daemonSet.Status.NumberUnavailable))

	if daemonSet.Spec.UpdateStrategy.Type != "" {
		buf.WriteString(fmt.Sprintf("Update Strategy:  %s\n", daemonSet.Spec.UpdateStrategy.Type))
		if daemonSet.Spec.UpdateStrategy.RollingUpdate != nil {
			if daemonSet.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable != nil {
				buf.WriteString(fmt.Sprintf("  Max Unavailable:  %s\n",
					daemonSet.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.String()))
			}
			if daemonSet.Spec.UpdateStrategy.RollingUpdate.MaxSurge != nil {
				buf.WriteString(fmt.Sprintf("  Max Surge:  %s\n",
					daemonSet.Spec.UpdateStrategy.RollingUpdate.MaxSurge.String()))
			}
		}
	}

	if daemonSet.Spec.MinReadySeconds > 0 {
		buf.WriteString(fmt.Sprintf("Min Ready Seconds:  %d\n", daemonSet.Spec.MinReadySeconds))
	}

	buf.WriteString("Pod Template:\n")
	buf.WriteString("  Labels:  ")
	if len(daemonSet.Spec.Template.Labels) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range daemonSet.Spec.Template.Labels {
			if !first {
				buf.WriteString("           ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	}

	if len(daemonSet.Spec.Template.Annotations) > 0 {
		buf.WriteString("  Annotations:  ")
		first := true
		for k, v := range daemonSet.Spec.Template.Annotations {
			if !first {
				buf.WriteString("                ")
			}
			buf.WriteString(fmt.Sprintf("%s: %s\n", k, v))
			first = false
		}
	}

	if daemonSet.Spec.Template.Spec.ServiceAccountName != "" {
		buf.WriteString(fmt.Sprintf("  Service Account:  %s\n", daemonSet.Spec.Template.Spec.ServiceAccountName))
	} else {
		buf.WriteString("  Service Account:  default\n")
	}

	if len(daemonSet.Spec.Template.Spec.InitContainers) > 0 {
		buf.WriteString("  Init Containers:\n")
		for _, container := range daemonSet.Spec.Template.Spec.InitContainers {
			buf.WriteString(fmt.Sprintf("   %s:\n", container.Name))
			buf.WriteString(fmt.Sprintf("    Image:      %s\n", container.Image))

			if container.ImagePullPolicy != "" {
				buf.WriteString(fmt.Sprintf("    Image Pull Policy:  %s\n", container.ImagePullPolicy))
			}

			if len(container.Ports) > 0 {
				for _, port := range container.Ports {
					buf.WriteString(fmt.Sprintf("    Port:       %d/%s\n", port.ContainerPort, port.Protocol))
					if port.HostPort > 0 {
						buf.WriteString(fmt.Sprintf("    Host Port:  %d/%s\n", port.HostPort, port.Protocol))
					}
				}
			}

			if len(container.Resources.Limits) > 0 {
				buf.WriteString("    Limits:\n")
				if cpu := container.Resources.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
					buf.WriteString(fmt.Sprintf("      cpu:     %s\n", cpu.String()))
				}
				if mem := container.Resources.Limits.Memory(); mem != nil && !mem.IsZero() {
					buf.WriteString(fmt.Sprintf("      memory:  %s\n", mem.String()))
				}
				if storage := container.Resources.Limits.StorageEphemeral(); storage != nil && !storage.IsZero() {
					buf.WriteString(fmt.Sprintf("      ephemeral-storage:  %s\n", storage.String()))
				}
			}

			if len(container.Resources.Requests) > 0 {
				buf.WriteString("    Requests:\n")
				if cpu := container.Resources.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
					buf.WriteString(fmt.Sprintf("      cpu:     %s\n", cpu.String()))
				}
				if mem := container.Resources.Requests.Memory(); mem != nil && !mem.IsZero() {
					buf.WriteString(fmt.Sprintf("      memory:  %s\n", mem.String()))
				}
				if storage := container.Resources.Requests.StorageEphemeral(); storage != nil && !storage.IsZero() {
					buf.WriteString(fmt.Sprintf("      ephemeral-storage:  %s\n", storage.String()))
				}
			}

			if len(container.Env) > 0 {
				buf.WriteString("    Environment:\n")
				for _, env := range container.Env {
					d.formatEnvironment(&buf, env, "      ")
				}
			}

			if len(container.VolumeMounts) > 0 {
				buf.WriteString("    Mounts:\n")
				for _, mount := range container.VolumeMounts {
					buf.WriteString(fmt.Sprintf("      %s from %s (%s)\n", mount.MountPath, mount.Name, func() string {
						if mount.ReadOnly {
							return "ro"
						}
						return "rw"
					}()))
				}
			}
		}
	}

	buf.WriteString("  Containers:\n")
	for _, container := range daemonSet.Spec.Template.Spec.Containers {
		buf.WriteString(fmt.Sprintf("   %s:\n", container.Name))
		buf.WriteString(fmt.Sprintf("    Image:      %s\n", container.Image))

		if container.ImagePullPolicy != "" {
			buf.WriteString(fmt.Sprintf("    Image Pull Policy:  %s\n", container.ImagePullPolicy))
		}

		if len(container.Ports) > 0 {
			for _, port := range container.Ports {
				buf.WriteString(fmt.Sprintf("    Port:       %d/%s\n", port.ContainerPort, port.Protocol))
				if port.HostPort > 0 {
					buf.WriteString(fmt.Sprintf("    Host Port:  %d/%s\n", port.HostPort, port.Protocol))
				}
			}
		}

		if len(container.Command) > 0 {
			buf.WriteString("    Command:\n")
			for _, cmd := range container.Command {
				buf.WriteString(fmt.Sprintf("      %s\n", cmd))
			}
		}

		if len(container.Args) > 0 {
			buf.WriteString("    Args:\n")
			for _, arg := range container.Args {
				buf.WriteString(fmt.Sprintf("      %s\n", arg))
			}
		}

		if len(container.Resources.Limits) > 0 {
			buf.WriteString("    Limits:\n")
			if cpu := container.Resources.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
				buf.WriteString(fmt.Sprintf("      cpu:     %s\n", cpu.String()))
			}
			if mem := container.Resources.Limits.Memory(); mem != nil && !mem.IsZero() {
				buf.WriteString(fmt.Sprintf("      memory:  %s\n", mem.String()))
			}
			if storage := container.Resources.Limits.StorageEphemeral(); storage != nil && !storage.IsZero() {
				buf.WriteString(fmt.Sprintf("      ephemeral-storage:  %s\n", storage.String()))
			}
		}

		if len(container.Resources.Requests) > 0 {
			buf.WriteString("    Requests:\n")
			if cpu := container.Resources.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
				buf.WriteString(fmt.Sprintf("      cpu:     %s\n", cpu.String()))
			}
			if mem := container.Resources.Requests.Memory(); mem != nil && !mem.IsZero() {
				buf.WriteString(fmt.Sprintf("      memory:  %s\n", mem.String()))
			}
			if storage := container.Resources.Requests.StorageEphemeral(); storage != nil && !storage.IsZero() {
				buf.WriteString(fmt.Sprintf("      ephemeral-storage:  %s\n", storage.String()))
			}
		}

		if container.LivenessProbe != nil {
			buf.WriteString(fmt.Sprintf("    Liveness:   %s\n", d.formatProbeForDescribe(container.LivenessProbe)))
		}
		if container.ReadinessProbe != nil {
			buf.WriteString(fmt.Sprintf("    Readiness:  %s\n", d.formatProbeForDescribe(container.ReadinessProbe)))
		}
		if container.StartupProbe != nil {
			buf.WriteString(fmt.Sprintf("    Startup:    %s\n", d.formatProbeForDescribe(container.StartupProbe)))
		}

		if len(container.Env) > 0 {
			buf.WriteString("    Environment:\n")
			for _, env := range container.Env {
				d.formatEnvironment(&buf, env, "      ")
			}
		}

		if len(container.VolumeMounts) > 0 {
			buf.WriteString("    Mounts:\n")
			for _, mount := range container.VolumeMounts {
				buf.WriteString(fmt.Sprintf("      %s from %s (%s)\n", mount.MountPath, mount.Name, func() string {
					if mount.ReadOnly {
						return "ro"
					}
					return "rw"
				}()))
			}
		}
	}

	buf.WriteString("  Volumes:\n")
	if len(daemonSet.Spec.Template.Spec.Volumes) > 0 {
		for _, vol := range daemonSet.Spec.Template.Spec.Volumes {
			buf.WriteString(fmt.Sprintf("   %s:\n", vol.Name))
			if vol.ConfigMap != nil {
				buf.WriteString("    Type:       ConfigMap (a volume populated by a ConfigMap)\n")
				buf.WriteString(fmt.Sprintf("    Name:       %s\n", vol.ConfigMap.Name))
				if vol.ConfigMap.Optional != nil {
					buf.WriteString(fmt.Sprintf("    Optional:   %v\n", *vol.ConfigMap.Optional))
				}
			} else if vol.Secret != nil {
				buf.WriteString("    Type:       Secret (a volume populated by a Secret)\n")
				buf.WriteString(fmt.Sprintf("    SecretName: %s\n", vol.Secret.SecretName))
				if vol.Secret.Optional != nil {
					buf.WriteString(fmt.Sprintf("    Optional:   %v\n", *vol.Secret.Optional))
				}
			} else if vol.HostPath != nil {
				buf.WriteString("    Type:       HostPath (bare host directory volume)\n")
				buf.WriteString(fmt.Sprintf("    Path:       %s\n", vol.HostPath.Path))
				if vol.HostPath.Type != nil {
					buf.WriteString(fmt.Sprintf("    HostPathType: %s\n", *vol.HostPath.Type))
				}
			} else if vol.EmptyDir != nil {
				buf.WriteString("    Type:       EmptyDir (a temporary directory that shares a pod's lifetime)\n")
				if vol.EmptyDir.Medium != "" {
					buf.WriteString(fmt.Sprintf("    Medium:     %s\n", vol.EmptyDir.Medium))
				}
				if vol.EmptyDir.SizeLimit != nil && !vol.EmptyDir.SizeLimit.IsZero() {
					buf.WriteString(fmt.Sprintf("    SizeLimit:  %s\n", vol.EmptyDir.SizeLimit.String()))
				}
			} else if vol.PersistentVolumeClaim != nil {
				buf.WriteString("    Type:       PersistentVolumeClaim (a reference to a PersistentVolumeClaim in the same namespace)\n")
				buf.WriteString(fmt.Sprintf("    ClaimName:  %s\n", vol.PersistentVolumeClaim.ClaimName))
				buf.WriteString(fmt.Sprintf("    ReadOnly:   %v\n", vol.PersistentVolumeClaim.ReadOnly))
			} else if vol.Projected != nil {
				buf.WriteString("    Type:       Projected (a volume that contains injected data from multiple sources)\n")
			}
		}
	} else {
		buf.WriteString("   <none>\n")
	}

	if len(daemonSet.Spec.Template.Spec.Tolerations) > 0 {
		buf.WriteString("  Tolerations:\n")
		for _, tol := range daemonSet.Spec.Template.Spec.Tolerations {
			buf.WriteString("    ")
			if tol.Key != "" {
				buf.WriteString(tol.Key)
			}
			if tol.Operator == corev1.TolerationOpEqual {
				buf.WriteString(fmt.Sprintf("=%s", tol.Value))
			} else if tol.Operator == corev1.TolerationOpExists {
				if tol.Key != "" {
					buf.WriteString(" op=Exists")
				}
			}
			if tol.Effect != "" {
				buf.WriteString(fmt.Sprintf(":%s", tol.Effect))
			}
			if tol.TolerationSeconds != nil {
				buf.WriteString(fmt.Sprintf(" for %ds", *tol.TolerationSeconds))
			}
			buf.WriteString("\n")
		}
	}

	if len(daemonSet.Status.Conditions) > 0 {
		buf.WriteString("Conditions:\n")
		buf.WriteString("  Type           Status  Reason\n")
		buf.WriteString("  ----           ------  ------\n")
		for _, cond := range daemonSet.Status.Conditions {
			buf.WriteString(fmt.Sprintf("  %-14s %-7s %s\n",
				cond.Type, cond.Status, cond.Reason))
		}
	}

	buf.WriteString("Events:\n")
	events, err := d.GetEvents(namespace, name)
	if err == nil && len(events) > 0 {
		buf.WriteString("  Type    Reason          Age                From                   Message\n")
		buf.WriteString("  ----    ------          ----               ----                   -------\n")

		limit := 10
		if len(events) < limit {
			limit = len(events)
		}

		for i := 0; i < limit; i++ {
			event := events[i]

			var ageStr string
			if event.LastTimestamp > 0 {
				age := time.Since(time.UnixMilli(event.LastTimestamp)).Round(time.Second)
				ageStr = d.formatDurationForDescribe(age)
			} else {
				ageStr = "<unknown>"
			}

			buf.WriteString(fmt.Sprintf("  %-7s %-15s %-18s %-22s %s\n",
				event.Type, event.Reason, ageStr, event.Source, event.Message))
		}
	} else {
		buf.WriteString("  <none>\n")
	}

	return buf.String(), nil
}

func (d *daemonSetOperator) formatEnvironment(buf *strings.Builder, env corev1.EnvVar, indent string) {
	if env.ValueFrom != nil {
		if env.ValueFrom.FieldRef != nil {
			buf.WriteString(fmt.Sprintf("%s%s:   (%s)\n", indent, env.Name, env.ValueFrom.FieldRef.FieldPath))
		} else if env.ValueFrom.SecretKeyRef != nil {
			buf.WriteString(fmt.Sprintf("%s%s:  <set to the key '%s' in secret '%s'>",
				indent, env.Name, env.ValueFrom.SecretKeyRef.Key, env.ValueFrom.SecretKeyRef.Name))
			if env.ValueFrom.SecretKeyRef.Optional != nil {
				buf.WriteString(fmt.Sprintf("  Optional: %v", *env.ValueFrom.SecretKeyRef.Optional))
			}
			buf.WriteString("\n")
		} else if env.ValueFrom.ConfigMapKeyRef != nil {
			buf.WriteString(fmt.Sprintf("%s%s:  <set to the key '%s' in config map '%s'>",
				indent, env.Name, env.ValueFrom.ConfigMapKeyRef.Key, env.ValueFrom.ConfigMapKeyRef.Name))
			if env.ValueFrom.ConfigMapKeyRef.Optional != nil {
				buf.WriteString(fmt.Sprintf("  Optional: %v", *env.ValueFrom.ConfigMapKeyRef.Optional))
			}
			buf.WriteString("\n")
		} else if env.ValueFrom.ResourceFieldRef != nil {
			buf.WriteString(fmt.Sprintf("%s%s:  <set from resource: %s>\n",
				indent, env.Name, env.ValueFrom.ResourceFieldRef.Resource))
		} else {
			buf.WriteString(fmt.Sprintf("%s%s:  <set from source>\n", indent, env.Name))
		}
	} else {
		buf.WriteString(fmt.Sprintf("%s%s:  %s\n", indent, env.Name, env.Value))
	}
}

func (d *daemonSetOperator) formatProbeForDescribe(probe *corev1.Probe) string {
	if probe == nil {
		return ""
	}

	var parts []string

	if probe.HTTPGet != nil {
		host := probe.HTTPGet.Host
		if host == "" {
			host = "<node-ip>"
		}
		parts = append(parts, fmt.Sprintf("http-get %s:%s%s",
			host, probe.HTTPGet.Port.String(), probe.HTTPGet.Path))
	} else if probe.TCPSocket != nil {
		parts = append(parts, fmt.Sprintf("tcp-socket :%s", probe.TCPSocket.Port.String()))
	} else if probe.Exec != nil {
		parts = append(parts, "exec")
	} else if probe.GRPC != nil {
		parts = append(parts, fmt.Sprintf("grpc <pod>:%d", probe.GRPC.Port))
	}

	parts = append(parts, fmt.Sprintf("delay=%ds", probe.InitialDelaySeconds))
	parts = append(parts, fmt.Sprintf("timeout=%ds", probe.TimeoutSeconds))
	parts = append(parts, fmt.Sprintf("period=%ds", probe.PeriodSeconds))

	return strings.Join(parts, " ")
}

func (d *daemonSetOperator) formatDurationForDescribe(duration time.Duration) string {
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
}

func (d *daemonSetOperator) ListAll(namespace string) ([]appsv1.DaemonSet, error) {
	var daemonSets []*appsv1.DaemonSet
	var err error

	if d.useInformer && d.daemonSetLister != nil {
		if namespace == "" {
			daemonSets, err = d.daemonSetLister.List(labels.Everything())
		} else {
			daemonSets, err = d.daemonSetLister.DaemonSets(namespace).List(labels.Everything())
		}

		if err != nil {
			return d.listAllFromAPI(namespace)
		}
	} else {
		return d.listAllFromAPI(namespace)
	}

	result := make([]appsv1.DaemonSet, 0, len(daemonSets))
	for _, daemonSet := range daemonSets {
		if daemonSet != nil {
			result = append(result, *daemonSet)
		}
	}

	return result, nil
}

func (d *daemonSetOperator) listAllFromAPI(namespace string) ([]appsv1.DaemonSet, error) {
	daemonSetList, err := d.client.AppsV1().DaemonSets(namespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		if namespace == "" {
			return nil, fmt.Errorf("获取所有DaemonSet失败: %v", err)
		}
		return nil, fmt.Errorf("获取命名空间 %s 的DaemonSet失败: %v", namespace, err)
	}

	return daemonSetList.Items, nil
}

func (d *daemonSetOperator) GetResourceSummary(
	namespace string,
	name string,
	domainSuffix string,
	nodeLb []string,
	podOp types.PodOperator,
	svcOp types.ServiceOperator,
	ingressOp types.IngressOperator,
) (*types.WorkloadResourceSummary, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	selectorLabels := daemonSet.Spec.Selector.MatchLabels
	if len(selectorLabels) == 0 {
		return nil, fmt.Errorf("DaemonSet 没有选择器标签")
	}

	return getWorkloadResourceSummary(
		namespace,
		selectorLabels,
		domainSuffix,
		nodeLb,
		podOp,
		svcOp,
		ingressOp,
	)
}

// ==================== 高级配置管理 ====================

// GetAdvancedConfig 获取高级容器配置
func (d *daemonSetOperator) GetAdvancedConfig(namespace, name string) (*types.AdvancedConfigResponse, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, fmt.Errorf("获取 DaemonSet 失败: %w", err)
	}

	return ConvertPodSpecToAdvancedConfig(&daemonSet.Spec.Template.Spec), nil
}

// UpdateAdvancedConfig 更新高级容器配置（全量更新）
func (d *daemonSetOperator) UpdateAdvancedConfig(req *types.UpdateAdvancedConfigRequest) error {
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}
	if req.Namespace == "" {
		return fmt.Errorf("命名空间不能为空")
	}
	if req.Name == "" {
		return fmt.Errorf("DaemonSet名称不能为空")
	}

	daemonSet, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("获取 DaemonSet 失败: %w", err)
	}

	if err := ApplyAdvancedConfigToPodSpec(&daemonSet.Spec.Template.Spec, req); err != nil {
		return fmt.Errorf("应用高级配置失败: %w", err)
	}

	// DaemonSet 的 RestartPolicy 固定为 Always
	daemonSet.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyAlways

	_, err = d.Update(daemonSet)
	if err != nil {
		return fmt.Errorf("更新 DaemonSet 失败: %w", err)
	}

	return nil
}
