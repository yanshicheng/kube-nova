package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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

	// 设置 TypeMeta
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

func (d *daemonSetOperator) GetContainerImages(namespace, name string) (*types.ContainerInfoList, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	result := &types.ContainerInfoList{
		InitContainers: make([]types.ContainerInfo, 0),
		Containers:     make([]types.ContainerInfo, 0),
	}

	for _, container := range daemonSet.Spec.Template.Spec.InitContainers {
		result.InitContainers = append(result.InitContainers, types.ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		})
	}

	for _, container := range daemonSet.Spec.Template.Spec.Containers {
		result.Containers = append(result.Containers, types.ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		})
	}

	return result, nil
}
func (d *daemonSetOperator) UpdateImage(req *types.UpdateImageRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.Image == "" {
		return fmt.Errorf("请求参数不完整")
	}

	daemonSet, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	updateContainer := func(containers []corev1.Container) bool {
		for i := range containers {
			if req.ContainerName == "" || containers[i].Name == req.ContainerName {
				containers[i].Image = req.Image
				return true
			}
		}
		return false
	}

	updateEphemeralContainer := func(containers []corev1.EphemeralContainer) bool {
		for i := range containers {
			if req.ContainerName == "" || containers[i].Name == req.ContainerName {
				containers[i].Image = req.Image
				return true
			}
		}
		return false
	}

	updated := updateContainer(daemonSet.Spec.Template.Spec.InitContainers) ||
		updateContainer(daemonSet.Spec.Template.Spec.Containers) ||
		updateEphemeralContainer(daemonSet.Spec.Template.Spec.EphemeralContainers)

	if !updated {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = d.Update(daemonSet)
	return err
}

func (d *daemonSetOperator) UpdateImages(req *types.UpdateImagesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	daemonSet, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	// 更新 InitContainers
	for _, img := range req.Containers.InitContainers {
		for i := range daemonSet.Spec.Template.Spec.InitContainers {
			if daemonSet.Spec.Template.Spec.InitContainers[i].Name == img.Name {
				daemonSet.Spec.Template.Spec.InitContainers[i].Image = img.Image
				break
			}
		}
	}

	// 更新 Containers
	for _, img := range req.Containers.Containers {
		for i := range daemonSet.Spec.Template.Spec.Containers {
			if daemonSet.Spec.Template.Spec.Containers[i].Name == img.Name {
				daemonSet.Spec.Template.Spec.Containers[i].Image = img.Image
				break
			}
		}
	}

	// 更新 EphemeralContainers
	if req.Containers.EphemeralContainers != nil {
		for _, img := range req.Containers.EphemeralContainers {
			for i := range daemonSet.Spec.Template.Spec.EphemeralContainers {
				if daemonSet.Spec.Template.Spec.EphemeralContainers[i].Name == img.Name {
					daemonSet.Spec.Template.Spec.EphemeralContainers[i].Image = img.Image
					break
				}
			}
		}
	}

	_, err = d.Update(daemonSet)
	return err
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
			daemonSet.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = &maxUnavailable
		}

		if req.RollingUpdate.MaxSurge != "" {
			maxSurge := intstr.Parse(req.RollingUpdate.MaxSurge)
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

func (d *daemonSetOperator) GetEnvVars(namespace, name string) (*types.EnvVarsResponse, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.EnvVarsResponse{
		Containers: make([]types.ContainerEnvVars, 0),
	}

	for _, container := range daemonSet.Spec.Template.Spec.Containers {
		envVars := make([]types.EnvVar, 0)
		for _, env := range container.Env {
			envVar := types.EnvVar{
				Name: env.Name,
				Source: types.EnvVarSource{
					Type:  "value",
					Value: env.Value,
				},
			}

			if env.ValueFrom != nil {
				if env.ValueFrom.ConfigMapKeyRef != nil {
					envVar.Source.Type = "configMapKeyRef"
					envVar.Source.ConfigMapKeyRef = &types.ConfigMapKeySelector{
						Name:     env.ValueFrom.ConfigMapKeyRef.Name,
						Key:      env.ValueFrom.ConfigMapKeyRef.Key,
						Optional: env.ValueFrom.ConfigMapKeyRef.Optional != nil && *env.ValueFrom.ConfigMapKeyRef.Optional,
					}
				} else if env.ValueFrom.SecretKeyRef != nil {
					envVar.Source.Type = "secretKeyRef"
					envVar.Source.SecretKeyRef = &types.SecretKeySelector{
						Name:     env.ValueFrom.SecretKeyRef.Name,
						Key:      env.ValueFrom.SecretKeyRef.Key,
						Optional: env.ValueFrom.SecretKeyRef.Optional != nil && *env.ValueFrom.SecretKeyRef.Optional,
					}
				} else if env.ValueFrom.FieldRef != nil {
					envVar.Source.Type = "fieldRef"
					envVar.Source.FieldRef = &types.ObjectFieldSelector{
						FieldPath: env.ValueFrom.FieldRef.FieldPath,
					}
				} else if env.ValueFrom.ResourceFieldRef != nil {
					envVar.Source.Type = "resourceFieldRef"
					divisor := ""
					if env.ValueFrom.ResourceFieldRef.Divisor.String() != "" {
						divisor = env.ValueFrom.ResourceFieldRef.Divisor.String()
					}
					envVar.Source.ResourceFieldRef = &types.ResourceFieldSelector{
						ContainerName: env.ValueFrom.ResourceFieldRef.ContainerName,
						Resource:      env.ValueFrom.ResourceFieldRef.Resource,
						Divisor:       divisor,
					}
				}
			}

			envVars = append(envVars, envVar)
		}

		response.Containers = append(response.Containers, types.ContainerEnvVars{
			ContainerName: container.Name,
			Env:           envVars,
		})
	}

	return response, nil
}

func (d *daemonSetOperator) UpdateEnvVars(req *types.UpdateEnvVarsRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.ContainerName == "" {
		return fmt.Errorf("请求参数不完整")
	}

	daemonSet, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	found := false
	for i := range daemonSet.Spec.Template.Spec.Containers {
		if daemonSet.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
			envVars := make([]corev1.EnvVar, 0)
			for _, env := range req.Env {
				envVar := corev1.EnvVar{Name: env.Name}

				switch env.Source.Type {
				case "value":
					envVar.Value = env.Source.Value
				case "configMapKeyRef":
					if env.Source.ConfigMapKeyRef != nil {
						envVar.ValueFrom = &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: env.Source.ConfigMapKeyRef.Name},
								Key:                  env.Source.ConfigMapKeyRef.Key,
								Optional:             &env.Source.ConfigMapKeyRef.Optional,
							},
						}
					}
				case "secretKeyRef":
					if env.Source.SecretKeyRef != nil {
						envVar.ValueFrom = &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: env.Source.SecretKeyRef.Name},
								Key:                  env.Source.SecretKeyRef.Key,
								Optional:             &env.Source.SecretKeyRef.Optional,
							},
						}
					}
				case "fieldRef":
					if env.Source.FieldRef != nil {
						envVar.ValueFrom = &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{FieldPath: env.Source.FieldRef.FieldPath},
						}
					}
				case "resourceFieldRef":
					if env.Source.ResourceFieldRef != nil {
						var divisor resource.Quantity
						if env.Source.ResourceFieldRef.Divisor != "" {
							divisor, _ = resource.ParseQuantity(env.Source.ResourceFieldRef.Divisor)
						}
						envVar.ValueFrom = &corev1.EnvVarSource{
							ResourceFieldRef: &corev1.ResourceFieldSelector{
								ContainerName: env.Source.ResourceFieldRef.ContainerName,
								Resource:      env.Source.ResourceFieldRef.Resource,
								Divisor:       divisor,
							},
						}
					}
				}

				envVars = append(envVars, envVar)
			}

			daemonSet.Spec.Template.Spec.Containers[i].Env = envVars
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
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

func (d *daemonSetOperator) GetResources(namespace, name string) (*types.ResourcesResponse, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.ResourcesResponse{
		Containers: make([]types.ContainerResources, 0),
	}

	for _, container := range daemonSet.Spec.Template.Spec.Containers {
		resources := types.ContainerResources{
			ContainerName: container.Name,
			Resources: types.ResourceRequirements{
				Limits: types.ResourceList{
					Cpu:    container.Resources.Limits.Cpu().String(),
					Memory: container.Resources.Limits.Memory().String(),
				},
				Requests: types.ResourceList{
					Cpu:    container.Resources.Requests.Cpu().String(),
					Memory: container.Resources.Requests.Memory().String(),
				},
			},
		}
		response.Containers = append(response.Containers, resources)
	}

	return response, nil
}

func (d *daemonSetOperator) UpdateResources(req *types.UpdateResourcesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.ContainerName == "" {
		return fmt.Errorf("请求参数不完整")
	}

	daemonSet, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	found := false
	for i := range daemonSet.Spec.Template.Spec.Containers {
		if daemonSet.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
			if req.Resources.Limits.Cpu != "" {
				cpuLimit, err := resource.ParseQuantity(req.Resources.Limits.Cpu)
				if err != nil {
					return fmt.Errorf("解析CPU限制失败: %v", err)
				}
				if daemonSet.Spec.Template.Spec.Containers[i].Resources.Limits == nil {
					daemonSet.Spec.Template.Spec.Containers[i].Resources.Limits = corev1.ResourceList{}
				}
				daemonSet.Spec.Template.Spec.Containers[i].Resources.Limits[corev1.ResourceCPU] = cpuLimit
			}

			if req.Resources.Limits.Memory != "" {
				memLimit, err := resource.ParseQuantity(req.Resources.Limits.Memory)
				if err != nil {
					return fmt.Errorf("解析内存限制失败: %v", err)
				}
				if daemonSet.Spec.Template.Spec.Containers[i].Resources.Limits == nil {
					daemonSet.Spec.Template.Spec.Containers[i].Resources.Limits = corev1.ResourceList{}
				}
				daemonSet.Spec.Template.Spec.Containers[i].Resources.Limits[corev1.ResourceMemory] = memLimit
			}

			if req.Resources.Requests.Cpu != "" {
				cpuRequest, err := resource.ParseQuantity(req.Resources.Requests.Cpu)
				if err != nil {
					return fmt.Errorf("解析CPU请求失败: %v", err)
				}
				if daemonSet.Spec.Template.Spec.Containers[i].Resources.Requests == nil {
					daemonSet.Spec.Template.Spec.Containers[i].Resources.Requests = corev1.ResourceList{}
				}
				daemonSet.Spec.Template.Spec.Containers[i].Resources.Requests[corev1.ResourceCPU] = cpuRequest
			}

			if req.Resources.Requests.Memory != "" {
				memRequest, err := resource.ParseQuantity(req.Resources.Requests.Memory)
				if err != nil {
					return fmt.Errorf("解析内存请求失败: %v", err)
				}
				if daemonSet.Spec.Template.Spec.Containers[i].Resources.Requests == nil {
					daemonSet.Spec.Template.Spec.Containers[i].Resources.Requests = corev1.ResourceList{}
				}
				daemonSet.Spec.Template.Spec.Containers[i].Resources.Requests[corev1.ResourceMemory] = memRequest
			}

			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = d.Update(daemonSet)
	return err
}

func (d *daemonSetOperator) GetProbes(namespace, name string) (*types.ProbesResponse, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.ProbesResponse{
		Containers: make([]types.ContainerProbes, 0),
	}

	for _, container := range daemonSet.Spec.Template.Spec.Containers {
		containerProbes := types.ContainerProbes{
			ContainerName: container.Name,
		}

		if container.LivenessProbe != nil {
			containerProbes.LivenessProbe = d.convertProbe(container.LivenessProbe)
		}
		if container.ReadinessProbe != nil {
			containerProbes.ReadinessProbe = d.convertProbe(container.ReadinessProbe)
		}
		if container.StartupProbe != nil {
			containerProbes.StartupProbe = d.convertProbe(container.StartupProbe)
		}

		response.Containers = append(response.Containers, containerProbes)
	}

	return response, nil
}

func (d *daemonSetOperator) convertProbe(probe *corev1.Probe) *types.Probe {
	result := &types.Probe{
		InitialDelaySeconds: probe.InitialDelaySeconds,
		TimeoutSeconds:      probe.TimeoutSeconds,
		PeriodSeconds:       probe.PeriodSeconds,
		SuccessThreshold:    probe.SuccessThreshold,
		FailureThreshold:    probe.FailureThreshold,
	}

	if probe.HTTPGet != nil {
		result.Type = "httpGet"
		headers := make([]types.HTTPHeader, 0)
		for _, h := range probe.HTTPGet.HTTPHeaders {
			headers = append(headers, types.HTTPHeader{Name: h.Name, Value: h.Value})
		}
		result.HttpGet = &types.HTTPGetAction{
			Path:        probe.HTTPGet.Path,
			Port:        probe.HTTPGet.Port.IntVal,
			Host:        probe.HTTPGet.Host,
			Scheme:      string(probe.HTTPGet.Scheme),
			HttpHeaders: headers,
		}
	} else if probe.TCPSocket != nil {
		result.Type = "tcpSocket"
		result.TcpSocket = &types.TCPSocketAction{
			Port: probe.TCPSocket.Port.IntVal,
			Host: probe.TCPSocket.Host,
		}
	} else if probe.Exec != nil {
		result.Type = "exec"
		result.Exec = &types.ExecAction{Command: probe.Exec.Command}
	} else if probe.GRPC != nil {
		result.Type = "grpc"
		service := ""
		if probe.GRPC.Service != nil {
			service = *probe.GRPC.Service
		}
		result.Grpc = &types.GRPCAction{Port: probe.GRPC.Port, Service: service}
	}

	return result
}

func (d *daemonSetOperator) UpdateProbes(req *types.UpdateProbesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.ContainerName == "" {
		return fmt.Errorf("请求参数不完整")
	}

	daemonSet, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	found := false
	for i := range daemonSet.Spec.Template.Spec.Containers {
		if daemonSet.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
			if req.LivenessProbe != nil {
				daemonSet.Spec.Template.Spec.Containers[i].LivenessProbe = d.buildProbe(req.LivenessProbe)
			}
			if req.ReadinessProbe != nil {
				daemonSet.Spec.Template.Spec.Containers[i].ReadinessProbe = d.buildProbe(req.ReadinessProbe)
			}
			if req.StartupProbe != nil {
				daemonSet.Spec.Template.Spec.Containers[i].StartupProbe = d.buildProbe(req.StartupProbe)
			}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = d.Update(daemonSet)
	return err
}

func (d *daemonSetOperator) buildProbe(probe *types.Probe) *corev1.Probe {
	result := &corev1.Probe{
		InitialDelaySeconds: probe.InitialDelaySeconds,
		TimeoutSeconds:      probe.TimeoutSeconds,
		PeriodSeconds:       probe.PeriodSeconds,
		SuccessThreshold:    probe.SuccessThreshold,
		FailureThreshold:    probe.FailureThreshold,
	}

	switch probe.Type {
	case "httpGet":
		if probe.HttpGet != nil {
			headers := make([]corev1.HTTPHeader, 0)
			for _, h := range probe.HttpGet.HttpHeaders {
				headers = append(headers, corev1.HTTPHeader{Name: h.Name, Value: h.Value})
			}
			result.HTTPGet = &corev1.HTTPGetAction{
				Path:        probe.HttpGet.Path,
				Port:        intstr.FromInt(int(probe.HttpGet.Port)),
				Host:        probe.HttpGet.Host,
				Scheme:      corev1.URIScheme(probe.HttpGet.Scheme),
				HTTPHeaders: headers,
			}
		}
	case "tcpSocket":
		if probe.TcpSocket != nil {
			result.TCPSocket = &corev1.TCPSocketAction{
				Port: intstr.FromInt(int(probe.TcpSocket.Port)),
				Host: probe.TcpSocket.Host,
			}
		}
	case "exec":
		if probe.Exec != nil {
			result.Exec = &corev1.ExecAction{Command: probe.Exec.Command}
		}
	case "grpc":
		if probe.Grpc != nil {
			service := probe.Grpc.Service
			result.GRPC = &corev1.GRPCAction{Port: probe.Grpc.Port, Service: &service}
		}
	}

	return result
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

// daemonset.go - operator
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

	// 检查是否通过节点选择器停止
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

	// 没有期望的 Pod
	if desiredNumber == 0 {
		status.Status = types.StatusStopped
		status.Message = "没有匹配的节点"
		status.Ready = true
		return status, nil
	}

	// 创建中：刚创建，还没有就绪的 Pod
	if numberReady == 0 && currentNumber == 0 {
		age := time.Since(daemonSet.CreationTimestamp.Time)
		if age < 30*time.Second {
			status.Status = types.StatusCreating
			status.Message = "正在创建 Pod"
			return status, nil
		}
	}

	// 运行中：就绪数等于期望数
	if numberReady == desiredNumber &&
		daemonSet.Status.NumberAvailable == desiredNumber &&
		daemonSet.Status.UpdatedNumberScheduled == desiredNumber {
		status.Status = types.StatusRunning
		status.Message = fmt.Sprintf("所有 Pod 运行正常 (%d/%d)", numberReady, desiredNumber)
		status.Ready = true
		return status, nil
	}

	// 更新中
	if daemonSet.Status.UpdatedNumberScheduled < desiredNumber {
		status.Status = types.StatusRunning
		status.Message = fmt.Sprintf("正在更新中 (已更新: %d/%d, 就绪: %d/%d)",
			daemonSet.Status.UpdatedNumberScheduled, desiredNumber,
			numberReady, desiredNumber)
		return status, nil
	}

	// 异常：就绪数少于期望数
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

// GetSchedulingConfig 获取调度配置
func (d *daemonSetOperator) GetSchedulingConfig(namespace, name string) (*types.SchedulingConfig, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return convertPodSpecToSchedulingConfig(&daemonSet.Spec.Template.Spec), nil
}

// UpdateSchedulingConfig 更新调度配置
func (d *daemonSetOperator) UpdateSchedulingConfig(namespace, name string, config *types.UpdateSchedulingConfigRequest) error {
	if config == nil {
		return fmt.Errorf("调度配置不能为空")
	}

	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	// 更新 NodeSelector
	if config.NodeSelector != nil {
		daemonSet.Spec.Template.Spec.NodeSelector = config.NodeSelector
	}

	// 更新 NodeName
	if config.NodeName != "" {
		daemonSet.Spec.Template.Spec.NodeName = config.NodeName
	}

	// 更新 Affinity
	if config.Affinity != nil {
		daemonSet.Spec.Template.Spec.Affinity = convertAffinityConfigToK8s(config.Affinity)
	}

	// 更新 Tolerations
	if config.Tolerations != nil {
		daemonSet.Spec.Template.Spec.Tolerations = convertTolerationsConfigToK8s(config.Tolerations)
	}

	// 更新 TopologySpreadConstraints
	if config.TopologySpreadConstraints != nil {
		daemonSet.Spec.Template.Spec.TopologySpreadConstraints = convertTopologySpreadConstraintsToK8s(config.TopologySpreadConstraints)
	}

	// 更新 SchedulerName
	if config.SchedulerName != "" {
		daemonSet.Spec.Template.Spec.SchedulerName = config.SchedulerName
	}

	// 更新 PriorityClassName
	if config.PriorityClassName != "" {
		daemonSet.Spec.Template.Spec.PriorityClassName = config.PriorityClassName
	}

	// 更新 Priority
	if config.Priority != nil {
		daemonSet.Spec.Template.Spec.Priority = config.Priority
	}

	// 更新 RuntimeClassName
	if config.RuntimeClassName != nil {
		daemonSet.Spec.Template.Spec.RuntimeClassName = config.RuntimeClassName
	}

	_, err = d.Update(daemonSet)
	return err
}

// ==================== 存储配置相关 ====================

// GetStorageConfig 获取存储配置
func (d *daemonSetOperator) GetStorageConfig(namespace, name string) (*types.StorageConfig, error) {
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return convertPodSpecToStorageConfig(&daemonSet.Spec.Template.Spec), nil
}

// UpdateStorageConfig 更新存储配置
func (d *daemonSetOperator) UpdateStorageConfig(namespace, name string, config *types.UpdateStorageConfigRequest) error {
	if config == nil {
		return fmt.Errorf("存储配置不能为空")
	}

	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	// 更新 Volumes
	if config.Volumes != nil {
		daemonSet.Spec.Template.Spec.Volumes = convertVolumesConfigToK8s(config.Volumes)
	}

	// 更新 VolumeMounts
	if config.VolumeMounts != nil {
		for _, vmConfig := range config.VolumeMounts {
			for i := range daemonSet.Spec.Template.Spec.Containers {
				if daemonSet.Spec.Template.Spec.Containers[i].Name == vmConfig.ContainerName {
					daemonSet.Spec.Template.Spec.Containers[i].VolumeMounts = convertVolumeMountsToK8s(vmConfig.Mounts)
					break
				}
			}
		}
	}

	_, err = d.Update(daemonSet)
	return err
}

// ==================== Events 相关 ====================

// GetEvents 获取 DaemonSet 的事件（已存在，确保实现正确）
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

	// 按最后发生时间降序排序
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

	// ========== 基本信息 ==========
	buf.WriteString(fmt.Sprintf("Name:           %s\n", daemonSet.Name))
	buf.WriteString(fmt.Sprintf("Namespace:      %s\n", daemonSet.Namespace))

	// 修复：Selector 可能为 nil
	if daemonSet.Spec.Selector != nil {
		buf.WriteString(fmt.Sprintf("Selector:       %s\n", metav1.FormatLabelSelector(daemonSet.Spec.Selector)))
	} else {
		buf.WriteString("Selector:       <none>\n")
	}

	// 修复：NodeSelector 显示优化
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

	// Labels
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

	// Annotations
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

	// 修复：添加 Pods Status 信息
	buf.WriteString(fmt.Sprintf("Pods Status:  %d Running / %d Ready / %d Updated / %d Unavailable\n",
		daemonSet.Status.CurrentNumberScheduled,
		daemonSet.Status.NumberReady,
		daemonSet.Status.UpdatedNumberScheduled,
		daemonSet.Status.NumberUnavailable))

	// 修复：添加 Update Strategy 信息
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

	// 修复：添加 MinReadySeconds
	if daemonSet.Spec.MinReadySeconds > 0 {
		buf.WriteString(fmt.Sprintf("Min Ready Seconds:  %d\n", daemonSet.Spec.MinReadySeconds))
	}

	// ========== Pod Template ==========
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

	// 修复：Annotations 可能存在
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

	// 修复：ServiceAccountName 可能为空
	if daemonSet.Spec.Template.Spec.ServiceAccountName != "" {
		buf.WriteString(fmt.Sprintf("  Service Account:  %s\n", daemonSet.Spec.Template.Spec.ServiceAccountName))
	} else {
		buf.WriteString("  Service Account:  default\n")
	}

	// Init Containers
	if len(daemonSet.Spec.Template.Spec.InitContainers) > 0 {
		buf.WriteString("  Init Containers:\n")
		for _, container := range daemonSet.Spec.Template.Spec.InitContainers {
			buf.WriteString(fmt.Sprintf("   %s:\n", container.Name))
			buf.WriteString(fmt.Sprintf("    Image:      %s\n", container.Image))

			// 修复：ImagePullPolicy
			if container.ImagePullPolicy != "" {
				buf.WriteString(fmt.Sprintf("    Image Pull Policy:  %s\n", container.ImagePullPolicy))
			}

			// 修复：Ports 可能为空
			if len(container.Ports) > 0 {
				for _, port := range container.Ports {
					buf.WriteString(fmt.Sprintf("    Port:       %d/%s\n", port.ContainerPort, port.Protocol))
					if port.HostPort > 0 {
						buf.WriteString(fmt.Sprintf("    Host Port:  %d/%s\n", port.HostPort, port.Protocol))
					}
				}
			}

			// 修复：检查 Limits 是否存在
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

			// 修复：检查 Requests 是否存在
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

			// 修复：Environment 详细处理
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

	// Containers
	buf.WriteString("  Containers:\n")
	for _, container := range daemonSet.Spec.Template.Spec.Containers {
		buf.WriteString(fmt.Sprintf("   %s:\n", container.Name))
		buf.WriteString(fmt.Sprintf("    Image:      %s\n", container.Image))

		// 修复：ImagePullPolicy
		if container.ImagePullPolicy != "" {
			buf.WriteString(fmt.Sprintf("    Image Pull Policy:  %s\n", container.ImagePullPolicy))
		}

		// 修复：Ports 可能为空，添加 HostPort 显示
		if len(container.Ports) > 0 {
			for _, port := range container.Ports {
				buf.WriteString(fmt.Sprintf("    Port:       %d/%s\n", port.ContainerPort, port.Protocol))
				if port.HostPort > 0 {
					buf.WriteString(fmt.Sprintf("    Host Port:  %d/%s\n", port.HostPort, port.Protocol))
				}
			}
		}

		// 修复：Command 和 Args
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

		// 修复：检查 Limits 是否存在
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

		// 修复：检查 Requests 是否存在
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

		// 修复：添加 Probes 支持
		if container.LivenessProbe != nil {
			buf.WriteString(fmt.Sprintf("    Liveness:   %s\n", d.formatProbeForDescribe(container.LivenessProbe)))
		}
		if container.ReadinessProbe != nil {
			buf.WriteString(fmt.Sprintf("    Readiness:  %s\n", d.formatProbeForDescribe(container.ReadinessProbe)))
		}
		if container.StartupProbe != nil {
			buf.WriteString(fmt.Sprintf("    Startup:    %s\n", d.formatProbeForDescribe(container.StartupProbe)))
		}

		// 修复：Environment 详细处理
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

	// Volumes - 修复：增加更多 Volume 类型支持
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

	// 修复：添加 Tolerations 显示
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

	// 修复：添加 Conditions
	if len(daemonSet.Status.Conditions) > 0 {
		buf.WriteString("Conditions:\n")
		buf.WriteString("  Type           Status  Reason\n")
		buf.WriteString("  ----           ------  ------\n")
		for _, cond := range daemonSet.Status.Conditions {
			buf.WriteString(fmt.Sprintf("  %-14s %-7s %s\n",
				cond.Type, cond.Status, cond.Reason))
		}
	}

	// ========== Events ==========
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

			// 修复：时间戳可能为 0
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

// 修复：添加 Environment 格式化辅助函数
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

// 修复：添加 Probe 格式化辅助函数
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

// ListAll 获取所有 DaemonSet（优先使用 informer）
func (d *daemonSetOperator) ListAll(namespace string) ([]appsv1.DaemonSet, error) {
	var daemonSets []*appsv1.DaemonSet
	var err error

	// 优先使用 informer
	if d.useInformer && d.daemonSetLister != nil {
		if namespace == "" {
			// 获取所有 namespace 的 DaemonSet
			daemonSets, err = d.daemonSetLister.List(labels.Everything())
		} else {
			// 获取指定 namespace 的 DaemonSet
			daemonSets, err = d.daemonSetLister.DaemonSets(namespace).List(labels.Everything())
		}

		if err != nil {
			// informer 失败，降级到 API 调用
			return d.listAllFromAPI(namespace)
		}
	} else {
		// 直接使用 API 调用
		return d.listAllFromAPI(namespace)
	}

	// 转换为非指针切片
	result := make([]appsv1.DaemonSet, 0, len(daemonSets))
	for _, daemonSet := range daemonSets {
		if daemonSet != nil {
			result = append(result, *daemonSet)
		}
	}

	return result, nil
}

// listAllFromAPI 通过 API 直接获取 DaemonSet 列表（内部辅助方法）
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

// GetResourceSummary 获取 DaemonSet 的资源摘要信息
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

	// 1. 获取 DaemonSet
	daemonSet, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	// 获取 Pod 选择器标签
	selectorLabels := daemonSet.Spec.Selector.MatchLabels
	if len(selectorLabels) == 0 {
		return nil, fmt.Errorf("DaemonSet 没有选择器标签")
	}

	// 使用通用辅助函数获取摘要
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
