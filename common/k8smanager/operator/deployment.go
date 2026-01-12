package operator

import (
	"context"
	"fmt"
	"sort"
	"strconv"
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
	AnnotationReplicas = "ikubeops.com/replicas"
)

type deploymentOperator struct {
	BaseOperator
	client             kubernetes.Interface
	informerFactory    informers.SharedInformerFactory
	deploymentLister   v1.DeploymentLister
	deploymentInformer cache.SharedIndexInformer
}

func NewDeploymentOperator(ctx context.Context, client kubernetes.Interface) types.DeploymentOperator {
	return &deploymentOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

func NewDeploymentOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.DeploymentOperator {
	var deploymentLister v1.DeploymentLister
	var deploymentInformer cache.SharedIndexInformer

	if informerFactory != nil {
		deploymentInformer = informerFactory.Apps().V1().Deployments().Informer()
		deploymentLister = informerFactory.Apps().V1().Deployments().Lister()
	}

	return &deploymentOperator{
		BaseOperator:       NewBaseOperator(ctx, informerFactory != nil),
		client:             client,
		informerFactory:    informerFactory,
		deploymentLister:   deploymentLister,
		deploymentInformer: deploymentInformer,
	}
}

func (d *deploymentOperator) Create(deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	if deployment == nil || deployment.Name == "" || deployment.Namespace == "" {
		return nil, fmt.Errorf("Deployment对象、名称和命名空间不能为空")
	}
	injectCommonAnnotations(deployment)
	if deployment.Labels == nil {
		deployment.Labels = make(map[string]string)
	}
	if deployment.Annotations == nil {
		deployment.Annotations = make(map[string]string)
	}

	created, err := d.client.AppsV1().Deployments(deployment.Namespace).Create(d.ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建Deployment失败: %v", err)
	}

	return created, nil
}

func (d *deploymentOperator) Get(namespace, name string) (*appsv1.Deployment, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	if d.deploymentLister != nil {
		deployment, err := d.deploymentLister.Deployments(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Deployment %s/%s 不存在", namespace, name)
			}
			deployment, apiErr := d.client.AppsV1().Deployments(namespace).Get(d.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取Deployment失败")
			}
			deployment.TypeMeta = metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			}
			deployment.Name = name
			return deployment, nil
		}
		// 注入 apiversion
		deployment.TypeMeta = metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		}
		deployment.Name = name
		return deployment, nil
	}

	deployment, err := d.client.AppsV1().Deployments(namespace).Get(d.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("Deployment %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取Deployment失败")
	}
	deployment.TypeMeta = metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	}
	deployment.Name = name
	return deployment, nil
}

func (d *deploymentOperator) Update(deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	if deployment == nil || deployment.Name == "" || deployment.Namespace == "" {
		return nil, fmt.Errorf("Deployment对象、名称和命名空间不能为空")
	}

	updated, err := d.client.AppsV1().Deployments(deployment.Namespace).Update(d.ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新Deployment失败: %v", err)
	}

	return updated, nil
}

func (d *deploymentOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := d.client.AppsV1().Deployments(namespace).Delete(d.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除Deployment失败: %v", err)
	}

	return nil
}

func (d *deploymentOperator) List(namespace string, req types.ListRequest) (*types.ListDeploymentResponse, error) {
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

	var deployments []*appsv1.Deployment
	var err error

	if d.useInformer && d.deploymentLister != nil {
		deployments, err = d.deploymentLister.Deployments(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取Deployment列表失败")
		}
	} else {
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		deploymentList, err := d.client.AppsV1().Deployments(namespace).List(d.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取Deployment列表失败")
		}
		deployments = make([]*appsv1.Deployment, len(deploymentList.Items))
		for i := range deploymentList.Items {
			deployments[i] = &deploymentList.Items[i]
		}
	}

	if req.Search != "" {
		filtered := make([]*appsv1.Deployment, 0)
		searchLower := strings.ToLower(req.Search)
		for _, deploy := range deployments {
			if strings.Contains(strings.ToLower(deploy.Name), searchLower) {
				filtered = append(filtered, deploy)
			}
		}
		deployments = filtered
	}

	sort.Slice(deployments, func(i, j int) bool {
		var less bool
		switch req.SortBy {
		case "creationTime", "creationTimestamp":
			less = deployments[i].CreationTimestamp.Before(&deployments[j].CreationTimestamp)
		default:
			less = deployments[i].Name < deployments[j].Name
		}
		if req.SortDesc {
			return !less
		}
		return less
	})

	total := len(deployments)
	totalPages := (total + req.PageSize - 1) / req.PageSize
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	if start >= total {
		return &types.ListDeploymentResponse{
			ListResponse: types.ListResponse{
				Total:      total,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: totalPages,
			},
			Items: []types.DeploymentInfo{},
		}, nil
	}

	if end > total {
		end = total
	}

	pageDeployments := deployments[start:end]
	items := make([]types.DeploymentInfo, len(pageDeployments))
	for i, deploy := range pageDeployments {
		items[i] = d.convertToDeploymentInfo(deploy)
	}

	return &types.ListDeploymentResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: items,
	}, nil
}

func (d *deploymentOperator) convertToDeploymentInfo(deploy *appsv1.Deployment) types.DeploymentInfo {
	images := make([]string, 0)
	for _, container := range deploy.Spec.Template.Spec.Containers {
		images = append(images, container.Image)
	}

	replicas := int32(0)
	if deploy.Spec.Replicas != nil {
		replicas = *deploy.Spec.Replicas
	}

	return types.DeploymentInfo{
		Name:              deploy.Name,
		Namespace:         deploy.Namespace,
		Replicas:          replicas,
		ReadyReplicas:     deploy.Status.ReadyReplicas,
		AvailableReplicas: deploy.Status.AvailableReplicas,
		CreationTimestamp: deploy.CreationTimestamp.Time,
		Images:            images,
	}
}

func (d *deploymentOperator) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return d.client.AppsV1().Deployments(namespace).Watch(d.ctx, opts)
}

func (d *deploymentOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	if deployment.Labels == nil {
		deployment.Labels = make(map[string]string)
	}
	for k, v := range labels {
		deployment.Labels[k] = v
	}

	_, err = d.Update(deployment)
	return err
}

func (d *deploymentOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	if deployment.Annotations == nil {
		deployment.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		deployment.Annotations[k] = v
	}

	_, err = d.Update(deployment)
	return err
}

func (d *deploymentOperator) GetYaml(namespace, name string) (string, error) {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return "", err
	}

	deployment.TypeMeta = metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	}
	deployment.Name = name
	deployment.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(deployment)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

func (d *deploymentOperator) GetPods(namespace, name string) ([]types.PodDetailInfo, error) {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
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

func (d *deploymentOperator) convertToPodDetailInfo(pod *corev1.Pod) types.PodDetailInfo {
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

func (d *deploymentOperator) GetContainerImages(namespace, name string) (*types.ContainerInfoList, error) {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	result := &types.ContainerInfoList{
		InitContainers: make([]types.ContainerInfo, 0),
		Containers:     make([]types.ContainerInfo, 0),
	}

	for _, container := range deployment.Spec.Template.Spec.InitContainers {
		result.InitContainers = append(result.InitContainers, types.ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		})
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		result.Containers = append(result.Containers, types.ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		})
	}

	return result, nil
}

func (d *deploymentOperator) UpdateImage(req *types.UpdateImageRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.Image == "" {
		return fmt.Errorf("请求参数不完整")
	}

	deployment, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	updated := false

	// 1. 尝试更新 InitContainers
	for i := range deployment.Spec.Template.Spec.InitContainers {
		if req.ContainerName == "" || deployment.Spec.Template.Spec.InitContainers[i].Name == req.ContainerName {
			deployment.Spec.Template.Spec.InitContainers[i].Image = req.Image
			updated = true
			if req.ContainerName != "" {
				break
			}
		}
	}

	// 2. 如果没找到，尝试更新 Containers
	if !updated {
		for i := range deployment.Spec.Template.Spec.Containers {
			if req.ContainerName == "" || deployment.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
				deployment.Spec.Template.Spec.Containers[i].Image = req.Image
				updated = true
				if req.ContainerName != "" {
					break
				}
			}
		}
	}

	// 3. 如果还没找到，尝试更新 EphemeralContainers
	if !updated {
		for i := range deployment.Spec.Template.Spec.EphemeralContainers {
			if req.ContainerName == "" || deployment.Spec.Template.Spec.EphemeralContainers[i].Name == req.ContainerName {
				deployment.Spec.Template.Spec.EphemeralContainers[i].Image = req.Image
				updated = true
				if req.ContainerName != "" {
					break
				}
			}
		}
	}

	if !updated {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = d.Update(deployment)
	return err
}

func (d *deploymentOperator) UpdateImages(req *types.UpdateImagesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	deployment, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	// 更新 InitContainers
	for _, img := range req.Containers.InitContainers {
		for i := range deployment.Spec.Template.Spec.InitContainers {
			if deployment.Spec.Template.Spec.InitContainers[i].Name == img.Name {
				deployment.Spec.Template.Spec.InitContainers[i].Image = img.Image
				break
			}
		}
	}

	// 更新 Containers
	for _, img := range req.Containers.Containers {
		for i := range deployment.Spec.Template.Spec.Containers {
			if deployment.Spec.Template.Spec.Containers[i].Name == img.Name {
				deployment.Spec.Template.Spec.Containers[i].Image = img.Image
				break
			}
		}
	}

	// 更新 EphemeralContainers
	if req.Containers.EphemeralContainers != nil {
		for _, img := range req.Containers.EphemeralContainers {
			for i := range deployment.Spec.Template.Spec.EphemeralContainers {
				if deployment.Spec.Template.Spec.EphemeralContainers[i].Name == img.Name {
					deployment.Spec.Template.Spec.EphemeralContainers[i].Image = img.Image
					break
				}
			}
		}
	}

	_, err = d.Update(deployment)
	return err
}
func (d *deploymentOperator) GetReplicas(namespace, name string) (*types.ReplicasInfo, error) {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	replicas := int32(0)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	return &types.ReplicasInfo{
		Replicas:          replicas,
		AvailableReplicas: deployment.Status.AvailableReplicas,
		ReadyReplicas:     deployment.Status.ReadyReplicas,
		UpdatedReplicas:   deployment.Status.UpdatedReplicas,
		CurrentReplicas:   deployment.Status.Replicas,
	}, nil
}

func (d *deploymentOperator) Scale(req *types.ScaleRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	deployment, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	deployment.Spec.Replicas = &req.Replicas
	_, err = d.Update(deployment)
	return err
}

func (d *deploymentOperator) GetUpdateStrategy(namespace, name string) (*types.UpdateStrategyResponse, error) {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.UpdateStrategyResponse{
		Type: string(deployment.Spec.Strategy.Type),
	}

	if deployment.Spec.Strategy.RollingUpdate != nil {
		response.RollingUpdate = &types.RollingUpdateConfig{
			MaxUnavailable: deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.String(),
			MaxSurge:       deployment.Spec.Strategy.RollingUpdate.MaxSurge.String(),
		}
	}

	return response, nil
}

func (d *deploymentOperator) UpdateStrategy(req *types.UpdateStrategyRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	deployment, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	deployment.Spec.Strategy.Type = appsv1.DeploymentStrategyType(req.Type)

	if req.Type == "RollingUpdate" && req.RollingUpdate != nil {
		if deployment.Spec.Strategy.RollingUpdate == nil {
			deployment.Spec.Strategy.RollingUpdate = &appsv1.RollingUpdateDeployment{}
		}

		if req.RollingUpdate.MaxUnavailable != "" {
			maxUnavailable := intstr.Parse(req.RollingUpdate.MaxUnavailable)
			deployment.Spec.Strategy.RollingUpdate.MaxUnavailable = &maxUnavailable
		}

		if req.RollingUpdate.MaxSurge != "" {
			maxSurge := intstr.Parse(req.RollingUpdate.MaxSurge)
			deployment.Spec.Strategy.RollingUpdate.MaxSurge = &maxSurge
		}
	}

	_, err = d.Update(deployment)
	return err
}

func (d *deploymentOperator) GetRevisions(namespace, name string) ([]types.RevisionInfo, error) {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	rsList, err := d.client.AppsV1().ReplicaSets(namespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取ReplicaSet列表失败")
	}

	revisions := make([]types.RevisionInfo, 0)
	for i := range rsList.Items {
		rs := &rsList.Items[i]

		isOwned := false
		for _, owner := range rs.OwnerReferences {
			if owner.Kind == "Deployment" && owner.Name == deployment.Name && owner.UID == deployment.UID {
				isOwned = true
				break
			}
		}

		if !isOwned {
			continue
		}

		revisionStr := rs.Annotations["deployment.kubernetes.io/revision"]
		if revisionStr == "" {
			continue
		}

		revision, err := strconv.ParseInt(revisionStr, 10, 64)
		if err != nil {
			continue
		}

		images := make([]string, 0)
		for _, container := range rs.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}

		reason := rs.Annotations["kubernetes.io/change-cause"]
		if reason == "" {
			reason = "Unknown"
		}

		replicas := int32(0)
		if rs.Spec.Replicas != nil {
			replicas = *rs.Spec.Replicas
		}

		revisions = append(revisions, types.RevisionInfo{
			Revision:          revision,
			CreationTimestamp: rs.CreationTimestamp.UnixMilli(),
			Images:            images,
			Replicas:          replicas,
			Reason:            reason,
		})
	}

	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].Revision > revisions[j].Revision
	})

	return revisions, nil
}

func (d *deploymentOperator) Rollback(req *types.RollbackToRevisionRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	deployment, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	rsList, err := d.client.AppsV1().ReplicaSets(req.Namespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("获取ReplicaSet列表失败")
	}

	var targetRS *appsv1.ReplicaSet
	for i := range rsList.Items {
		rs := &rsList.Items[i]

		isOwned := false
		for _, owner := range rs.OwnerReferences {
			if owner.Kind == "Deployment" && owner.Name == req.Name && owner.UID == deployment.UID {
				isOwned = true
				break
			}
		}

		if !isOwned {
			continue
		}

		revisionStr := rs.Annotations["deployment.kubernetes.io/revision"]
		if revisionStr != "" {
			revision, _ := strconv.ParseInt(revisionStr, 10, 64)
			if revision == req.Revision {
				targetRS = rs
				break
			}
		}
	}

	if targetRS == nil {
		return fmt.Errorf("未找到目标版本")
	}

	deployment.Spec.Template = targetRS.Spec.Template
	_, err = d.Update(deployment)
	return err
}

func (d *deploymentOperator) GetEnvVars(namespace, name string) (*types.EnvVarsResponse, error) {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.EnvVarsResponse{
		Containers: make([]types.ContainerEnvVars, 0),
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
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

func (d *deploymentOperator) UpdateEnvVars(req *types.UpdateEnvVarsRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.ContainerName == "" {
		return fmt.Errorf("请求参数不完整")
	}

	deployment, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	found := false
	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
			envVars := make([]corev1.EnvVar, 0)
			for _, env := range req.Env {
				envVar := corev1.EnvVar{
					Name: env.Name,
				}

				switch env.Source.Type {
				case "value":
					envVar.Value = env.Source.Value
				case "configMapKeyRef":
					if env.Source.ConfigMapKeyRef != nil {
						envVar.ValueFrom = &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: env.Source.ConfigMapKeyRef.Name,
								},
								Key:      env.Source.ConfigMapKeyRef.Key,
								Optional: &env.Source.ConfigMapKeyRef.Optional,
							},
						}
					}
				case "secretKeyRef":
					if env.Source.SecretKeyRef != nil {
						envVar.ValueFrom = &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: env.Source.SecretKeyRef.Name,
								},
								Key:      env.Source.SecretKeyRef.Key,
								Optional: &env.Source.SecretKeyRef.Optional,
							},
						}
					}
				case "fieldRef":
					if env.Source.FieldRef != nil {
						envVar.ValueFrom = &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: env.Source.FieldRef.FieldPath,
							},
						}
					}
				case "resourceFieldRef":
					if env.Source.ResourceFieldRef != nil {
						var divisor *resource.Quantity
						if env.Source.ResourceFieldRef.Divisor != "" {
							q, err := resource.ParseQuantity(env.Source.ResourceFieldRef.Divisor)
							if err == nil {
								divisor = &q
							}
						}
						envVar.ValueFrom = &corev1.EnvVarSource{
							ResourceFieldRef: &corev1.ResourceFieldSelector{
								ContainerName: env.Source.ResourceFieldRef.ContainerName,
								Resource:      env.Source.ResourceFieldRef.Resource,
								Divisor:       *divisor,
							},
						}
					}
				}

				envVars = append(envVars, envVar)
			}

			deployment.Spec.Template.Spec.Containers[i].Env = envVars
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = d.Update(deployment)
	return err
}

func (d *deploymentOperator) GetPauseStatus(namespace, name string) (*types.PauseStatusResponse, error) {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return &types.PauseStatusResponse{
		Paused:      deployment.Spec.Paused,
		SupportType: "pause",
	}, nil
}

func (d *deploymentOperator) PauseRollout(namespace, name string) error {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	deployment.Spec.Paused = true
	_, err = d.Update(deployment)
	return err
}

func (d *deploymentOperator) ResumeRollout(namespace, name string) error {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	deployment.Spec.Paused = false
	_, err = d.Update(deployment)
	return err
}

func (d *deploymentOperator) GetResources(namespace, name string) (*types.ResourcesResponse, error) {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.ResourcesResponse{
		Containers: make([]types.ContainerResources, 0),
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
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

func (d *deploymentOperator) UpdateResources(req *types.UpdateResourcesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.ContainerName == "" {
		return fmt.Errorf("请求参数不完整")
	}

	deployment, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	found := false
	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
			if req.Resources.Limits.Cpu != "" {
				cpuLimit, err := resource.ParseQuantity(req.Resources.Limits.Cpu)
				if err != nil {
					return fmt.Errorf("解析CPU限制失败: %v", err)
				}
				if deployment.Spec.Template.Spec.Containers[i].Resources.Limits == nil {
					deployment.Spec.Template.Spec.Containers[i].Resources.Limits = corev1.ResourceList{}
				}
				deployment.Spec.Template.Spec.Containers[i].Resources.Limits[corev1.ResourceCPU] = cpuLimit
			}

			if req.Resources.Limits.Memory != "" {
				memLimit, err := resource.ParseQuantity(req.Resources.Limits.Memory)
				if err != nil {
					return fmt.Errorf("解析内存限制失败: %v", err)
				}
				if deployment.Spec.Template.Spec.Containers[i].Resources.Limits == nil {
					deployment.Spec.Template.Spec.Containers[i].Resources.Limits = corev1.ResourceList{}
				}
				deployment.Spec.Template.Spec.Containers[i].Resources.Limits[corev1.ResourceMemory] = memLimit
			}

			if req.Resources.Requests.Cpu != "" {
				cpuRequest, err := resource.ParseQuantity(req.Resources.Requests.Cpu)
				if err != nil {
					return fmt.Errorf("解析CPU请求失败: %v", err)
				}
				if deployment.Spec.Template.Spec.Containers[i].Resources.Requests == nil {
					deployment.Spec.Template.Spec.Containers[i].Resources.Requests = corev1.ResourceList{}
				}
				deployment.Spec.Template.Spec.Containers[i].Resources.Requests[corev1.ResourceCPU] = cpuRequest
			}

			if req.Resources.Requests.Memory != "" {
				memRequest, err := resource.ParseQuantity(req.Resources.Requests.Memory)
				if err != nil {
					return fmt.Errorf("解析内存请求失败: %v", err)
				}
				if deployment.Spec.Template.Spec.Containers[i].Resources.Requests == nil {
					deployment.Spec.Template.Spec.Containers[i].Resources.Requests = corev1.ResourceList{}
				}
				deployment.Spec.Template.Spec.Containers[i].Resources.Requests[corev1.ResourceMemory] = memRequest
			}

			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = d.Update(deployment)
	return err
}

func (d *deploymentOperator) GetProbes(namespace, name string) (*types.ProbesResponse, error) {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.ProbesResponse{
		Containers: make([]types.ContainerProbes, 0),
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
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

func (d *deploymentOperator) convertProbe(probe *corev1.Probe) *types.Probe {
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
			headers = append(headers, types.HTTPHeader{
				Name:  h.Name,
				Value: h.Value,
			})
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
		result.Exec = &types.ExecAction{
			Command: probe.Exec.Command,
		}
	} else if probe.GRPC != nil {
		result.Type = "grpc"
		service := ""
		if probe.GRPC.Service != nil {
			service = *probe.GRPC.Service
		}
		result.Grpc = &types.GRPCAction{
			Port:    probe.GRPC.Port,
			Service: service,
		}
	}

	return result
}

func (d *deploymentOperator) UpdateProbes(req *types.UpdateProbesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.ContainerName == "" {
		return fmt.Errorf("请求参数不完整")
	}

	deployment, err := d.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	found := false
	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
			if req.LivenessProbe != nil {
				deployment.Spec.Template.Spec.Containers[i].LivenessProbe = d.buildProbe(req.LivenessProbe)
			}
			if req.ReadinessProbe != nil {
				deployment.Spec.Template.Spec.Containers[i].ReadinessProbe = d.buildProbe(req.ReadinessProbe)
			}
			if req.StartupProbe != nil {
				deployment.Spec.Template.Spec.Containers[i].StartupProbe = d.buildProbe(req.StartupProbe)
			}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = d.Update(deployment)
	return err
}

func (d *deploymentOperator) buildProbe(probe *types.Probe) *corev1.Probe {
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
				headers = append(headers, corev1.HTTPHeader{
					Name:  h.Name,
					Value: h.Value,
				})
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
			result.Exec = &corev1.ExecAction{
				Command: probe.Exec.Command,
			}
		}
	case "grpc":
		if probe.Grpc != nil {
			service := probe.Grpc.Service
			result.GRPC = &corev1.GRPCAction{
				Port:    probe.Grpc.Port,
				Service: &service,
			}
		}
	}

	return result
}

func (d *deploymentOperator) Stop(namespace, name string) error {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	currentReplicas := int32(0)
	if deployment.Spec.Replicas != nil {
		currentReplicas = *deployment.Spec.Replicas
	}

	if deployment.Annotations == nil {
		deployment.Annotations = make(map[string]string)
	}
	deployment.Annotations[AnnotationReplicas] = strconv.Itoa(int(currentReplicas))

	zero := int32(0)
	deployment.Spec.Replicas = &zero

	_, err = d.Update(deployment)
	return err
}

func (d *deploymentOperator) Start(namespace, name string) error {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	replicas := int32(1)
	if deployment.Annotations != nil {
		if replicasStr, ok := deployment.Annotations[AnnotationReplicas]; ok {
			if r, err := strconv.Atoi(replicasStr); err == nil && r > 0 {
				replicas = int32(r)
			}
		}
	}

	deployment.Spec.Replicas = &replicas
	_, err = d.Update(deployment)
	return err
}

func (d *deploymentOperator) Restart(namespace, name string) error {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = d.Update(deployment)
	return err
}

// deployment.go - operator
func (d *deploymentOperator) GetPodLabels(namespace, name string) (map[string]string, error) {
	var deployment *appsv1.Deployment
	var err error

	if d.useInformer && d.deploymentLister != nil {
		deployment, err = d.deploymentLister.Deployments(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Deployment %s/%s 不存在", namespace, name)
			}
			deployment, err = d.client.AppsV1().Deployments(namespace).Get(d.ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取Deployment失败")
			}
		}
	} else {
		deployment, err = d.client.AppsV1().Deployments(namespace).Get(d.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Deployment %s/%s 不存在", namespace, name)
			}
			return nil, fmt.Errorf("获取Deployment失败")
		}
	}

	if deployment.Spec.Template.Labels == nil {
		return make(map[string]string), nil
	}

	labels := make(map[string]string)
	for k, v := range deployment.Spec.Template.Labels {
		labels[k] = v
	}
	return labels, nil
}

func (d *deploymentOperator) GetPodSelectorLabels(namespace, name string) (map[string]string, error) {
	var deployment *appsv1.Deployment
	var err error

	if d.useInformer && d.deploymentLister != nil {
		deployment, err = d.deploymentLister.Deployments(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("deployment %s/%s 不存在", namespace, name)
			}
			deployment, err = d.client.AppsV1().Deployments(namespace).Get(d.ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取Deployment失败")
			}
		}
	} else {
		deployment, err = d.client.AppsV1().Deployments(namespace).Get(d.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("deployment %s/%s 不存在", namespace, name)
			}
			return nil, fmt.Errorf("获取Deployment失败")
		}
	}

	if deployment.Spec.Selector == nil || deployment.Spec.Selector.MatchLabels == nil {
		return make(map[string]string), nil
	}

	labels := make(map[string]string)
	for k, v := range deployment.Spec.Selector.MatchLabels {
		labels[k] = v
	}
	return labels, nil
}

func (d *deploymentOperator) GetVersionStatus(namespace, name string) (*types.ResourceStatus, error) {
	var deployment *appsv1.Deployment
	var err error

	if d.useInformer && d.deploymentLister != nil {
		deployment, err = d.deploymentLister.Deployments(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Deployment %s/%s 不存在", namespace, name)
			}
			deployment, err = d.client.AppsV1().Deployments(namespace).Get(d.ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取Deployment失败")
			}
		}
	} else {
		deployment, err = d.client.AppsV1().Deployments(namespace).Get(d.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Deployment %s/%s 不存在", namespace, name)
			}
			return nil, fmt.Errorf("获取Deployment失败")
		}
	}

	replicas := int32(0)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	status := &types.ResourceStatus{
		Ready: false,
	}

	// 停止状态：副本数为 0
	if replicas == 0 {
		status.Status = types.StatusStopped
		status.Message = "副本数为 0，已停止"
		status.Ready = true
		return status, nil
	}

	// 创建中：刚创建，还没有可用副本
	if deployment.Status.AvailableReplicas == 0 && deployment.Status.Replicas == 0 {
		age := time.Since(deployment.CreationTimestamp.Time)
		if age < 30*time.Second {
			status.Status = types.StatusCreating
			status.Message = "正在创建 Pod"
			return status, nil
		}
	}

	// 运行中：可用副本数等于期望副本数
	if deployment.Status.AvailableReplicas == replicas &&
		deployment.Status.ReadyReplicas == replicas &&
		deployment.Status.UpdatedReplicas == replicas {
		status.Status = types.StatusRunning
		status.Message = fmt.Sprintf("所有副本运行正常 (%d/%d)", deployment.Status.AvailableReplicas, replicas)
		status.Ready = true
		return status, nil
	}

	// 更新中
	if deployment.Status.UpdatedReplicas < replicas {
		status.Status = types.StatusRunning
		status.Message = fmt.Sprintf("正在更新中 (已更新: %d/%d, 可用: %d/%d)",
			deployment.Status.UpdatedReplicas, replicas,
			deployment.Status.AvailableReplicas, replicas)
		return status, nil
	}

	// 异常：可用副本数少于期望副本数
	if deployment.Status.AvailableReplicas < replicas {
		age := time.Since(deployment.CreationTimestamp.Time)
		if age > 5*time.Minute {
			status.Status = types.StatusError
			status.Message = fmt.Sprintf("部分副本异常 (可用: %d/%d)", deployment.Status.AvailableReplicas, replicas)
			return status, nil
		}
		status.Status = types.StatusCreating
		status.Message = fmt.Sprintf("正在启动 (可用: %d/%d)", deployment.Status.AvailableReplicas, replicas)
		return status, nil
	}

	status.Status = types.StatusRunning
	status.Message = "运行中"
	status.Ready = true
	return status, nil
}

// ==================== 调度配置相关 ====================

// GetSchedulingConfig 获取调度配置
func (d *deploymentOperator) GetSchedulingConfig(namespace, name string) (*types.SchedulingConfig, error) {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return convertPodSpecToSchedulingConfig(&deployment.Spec.Template.Spec), nil
}

// UpdateSchedulingConfig 更新调度配置
func (d *deploymentOperator) UpdateSchedulingConfig(namespace, name string, config *types.UpdateSchedulingConfigRequest) error {
	if config == nil {
		return fmt.Errorf("调度配置不能为空")
	}

	deployment, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	// 更新 NodeSelector
	if config.NodeSelector != nil {
		deployment.Spec.Template.Spec.NodeSelector = config.NodeSelector
	}

	// 更新 NodeName
	if config.NodeName != "" {
		deployment.Spec.Template.Spec.NodeName = config.NodeName
	}

	// 更新 Affinity
	if config.Affinity != nil {
		deployment.Spec.Template.Spec.Affinity = convertAffinityConfigToK8s(config.Affinity)
	}

	// 更新 Tolerations
	if config.Tolerations != nil {
		deployment.Spec.Template.Spec.Tolerations = convertTolerationsConfigToK8s(config.Tolerations)
	}

	// 更新 TopologySpreadConstraints
	if config.TopologySpreadConstraints != nil {
		deployment.Spec.Template.Spec.TopologySpreadConstraints = convertTopologySpreadConstraintsToK8s(config.TopologySpreadConstraints)
	}

	// 更新 SchedulerName
	if config.SchedulerName != "" {
		deployment.Spec.Template.Spec.SchedulerName = config.SchedulerName
	}

	// 更新 PriorityClassName
	if config.PriorityClassName != "" {
		deployment.Spec.Template.Spec.PriorityClassName = config.PriorityClassName
	}

	// 更新 Priority
	if config.Priority != nil {
		deployment.Spec.Template.Spec.Priority = config.Priority
	}

	// 更新 RuntimeClassName
	if config.RuntimeClassName != nil {
		deployment.Spec.Template.Spec.RuntimeClassName = config.RuntimeClassName
	}

	_, err = d.Update(deployment)
	return err
}

// ==================== 存储配置相关 ====================

// GetStorageConfig 获取存储配置
func (d *deploymentOperator) GetStorageConfig(namespace, name string) (*types.StorageConfig, error) {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return convertPodSpecToStorageConfig(&deployment.Spec.Template.Spec), nil
}

// UpdateStorageConfig 更新存储配置
func (d *deploymentOperator) UpdateStorageConfig(namespace, name string, config *types.UpdateStorageConfigRequest) error {
	if config == nil {
		return fmt.Errorf("存储配置不能为空")
	}

	deployment, err := d.Get(namespace, name)
	if err != nil {
		return err
	}

	// 更新 Volumes
	if config.Volumes != nil {
		deployment.Spec.Template.Spec.Volumes = convertVolumesConfigToK8s(config.Volumes)
	}

	// 更新 VolumeMounts
	if config.VolumeMounts != nil {
		for _, vmConfig := range config.VolumeMounts {
			for i := range deployment.Spec.Template.Spec.Containers {
				if deployment.Spec.Template.Spec.Containers[i].Name == vmConfig.ContainerName {
					deployment.Spec.Template.Spec.Containers[i].VolumeMounts = convertVolumeMountsToK8s(vmConfig.Mounts)
					break
				}
			}
		}
	}

	_, err = d.Update(deployment)
	return err
}

// ==================== Events 相关 ====================

// GetEvents 获取 Deployment 的事件（已存在，但需要确保实现正确）
func (d *deploymentOperator) GetEvents(namespace, name string) ([]types.EventInfo, error) {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	eventList, err := d.client.CoreV1().Events(namespace).List(d.ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Deployment,involvedObject.uid=%s",
			name, deployment.UID),
	})
	if err != nil {
		return nil, fmt.Errorf("获取事件列表失败: %v", err)
	}

	events := make([]types.EventInfo, 0, len(eventList.Items))
	for i := range eventList.Items {
		events = append(events, types.ConvertK8sEventToEventInfo(&eventList.Items[i]))
	}

	// 按最后发生时间降序排序（最新的在前面）
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTimestamp > events[j].LastTimestamp
	})

	return events, nil
}

func (d *deploymentOperator) GetDescribe(namespace, name string) (string, error) {
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return "", err
	}

	var buf strings.Builder

	// ========== 基本信息 ==========
	buf.WriteString(fmt.Sprintf("Name:                   %s\n", deployment.Name))
	buf.WriteString(fmt.Sprintf("Namespace:              %s\n", deployment.Namespace))

	if !deployment.CreationTimestamp.IsZero() {
		buf.WriteString(fmt.Sprintf("CreationTimestamp:      %s\n", deployment.CreationTimestamp.Format(time.RFC1123)))
	} else {
		buf.WriteString("CreationTimestamp:      <unset>\n")
	}

	// Labels
	buf.WriteString("Labels:                 ")
	if len(deployment.Labels) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range deployment.Labels {
			if !first {
				buf.WriteString("                        ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	}

	// Annotations
	buf.WriteString("Annotations:            ")
	if len(deployment.Annotations) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range deployment.Annotations {
			if !first {
				buf.WriteString("                        ")
			}
			buf.WriteString(fmt.Sprintf("%s: %s\n", k, v))
			first = false
		}
	}

	buf.WriteString("Selector:               ")
	if deployment.Spec.Selector != nil && len(deployment.Spec.Selector.MatchLabels) > 0 {
		first := true
		for k, v := range deployment.Spec.Selector.MatchLabels {
			if !first {
				buf.WriteString(",")
			}
			buf.WriteString(fmt.Sprintf("%s=%s", k, v))
			first = false
		}
		buf.WriteString("\n")
	} else {
		buf.WriteString("<none>\n")
	}

	// ========== 副本信息 ==========
	replicas := int32(0)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	unavailable := replicas - deployment.Status.AvailableReplicas
	if unavailable < 0 {
		unavailable = 0
	}

	buf.WriteString(fmt.Sprintf("Replicas:               %d desired | %d updated | %d total | %d available | %d unavailable\n",
		replicas,
		deployment.Status.UpdatedReplicas,
		deployment.Status.Replicas,
		deployment.Status.AvailableReplicas,
		unavailable,
	))

	// ========== 更新策略 ==========
	buf.WriteString(fmt.Sprintf("StrategyType:           %s\n", deployment.Spec.Strategy.Type))
	buf.WriteString(fmt.Sprintf("MinReadySeconds:        %d\n", deployment.Spec.MinReadySeconds))

	if deployment.Spec.Strategy.RollingUpdate != nil {
		maxUnavailable := "25%"
		maxSurge := "25%"

		if deployment.Spec.Strategy.RollingUpdate.MaxUnavailable != nil {
			maxUnavailable = deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.String()
		}
		if deployment.Spec.Strategy.RollingUpdate.MaxSurge != nil {
			maxSurge = deployment.Spec.Strategy.RollingUpdate.MaxSurge.String()
		}

		buf.WriteString(fmt.Sprintf("RollingUpdateStrategy:  %s max unavailable, %s max surge\n",
			maxUnavailable, maxSurge))
	}

	if deployment.Spec.RevisionHistoryLimit != nil {
		buf.WriteString(fmt.Sprintf("Revision History Limit: %d\n", *deployment.Spec.RevisionHistoryLimit))
	}

	// ========== Pod Template ==========
	buf.WriteString("Pod Template:\n")

	// Pod Labels
	buf.WriteString("  Labels:  ")
	if len(deployment.Spec.Template.Labels) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range deployment.Spec.Template.Labels {
			if !first {
				buf.WriteString("           ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	}

	if len(deployment.Spec.Template.Annotations) > 0 {
		buf.WriteString("  Annotations:  ")
		first := true
		for k, v := range deployment.Spec.Template.Annotations {
			if !first {
				buf.WriteString("                ")
			}
			buf.WriteString(fmt.Sprintf("%s: %s\n", k, v))
			first = false
		}
	}

	if deployment.Spec.Template.Spec.ServiceAccountName != "" {
		buf.WriteString(fmt.Sprintf("  Service Account:  %s\n", deployment.Spec.Template.Spec.ServiceAccountName))
	} else {
		buf.WriteString("  Service Account:  default\n")
	}

	// Init Containers
	if len(deployment.Spec.Template.Spec.InitContainers) > 0 {
		buf.WriteString("  Init Containers:\n")
		for _, container := range deployment.Spec.Template.Spec.InitContainers {
			buf.WriteString(fmt.Sprintf("   %s:\n", container.Name))
			buf.WriteString(fmt.Sprintf("    Image:      %s\n", container.Image))

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
			}

			if len(container.Resources.Requests) > 0 {
				buf.WriteString("    Requests:\n")
				if cpu := container.Resources.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
					buf.WriteString(fmt.Sprintf("      cpu:     %s\n", cpu.String()))
				}
				if mem := container.Resources.Requests.Memory(); mem != nil && !mem.IsZero() {
					buf.WriteString(fmt.Sprintf("      memory:  %s\n", mem.String()))
				}
			}

			if len(container.Env) > 0 {
				buf.WriteString("    Environment:\n")
				for _, env := range container.Env {
					d.formatEnvironment(&buf, env)
				}
			}

			if len(container.VolumeMounts) > 0 {
				buf.WriteString("    Mounts:\n")
				for _, mount := range container.VolumeMounts {
					buf.WriteString(fmt.Sprintf("      %s from %s (%s)\n",
						mount.MountPath, mount.Name, func() string {
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
	for _, container := range deployment.Spec.Template.Spec.Containers {
		buf.WriteString(fmt.Sprintf("   %s:\n", container.Name))
		buf.WriteString(fmt.Sprintf("    Image:      %s\n", container.Image))

		if container.ImagePullPolicy != "" {
			buf.WriteString(fmt.Sprintf("    Image Pull Policy:  %s\n", container.ImagePullPolicy))
		}

		// Ports
		if len(container.Ports) > 0 {
			for _, port := range container.Ports {
				buf.WriteString(fmt.Sprintf("    Port:       %d/%s\n", port.ContainerPort, port.Protocol))
				if port.HostPort > 0 {
					buf.WriteString(fmt.Sprintf("    Host Port:  %d/%s\n", port.HostPort, port.Protocol))
				} else {
					buf.WriteString("    Host Port:  0/TCP\n")
				}
			}
		} else {
			buf.WriteString("    Port:       <none>\n")
			buf.WriteString("    Host Port:  <none>\n")
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

		buf.WriteString("    Limits:\n")
		if len(container.Resources.Limits) > 0 {
			if cpu := container.Resources.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
				buf.WriteString(fmt.Sprintf("      cpu:     %s\n", cpu.String()))
			}
			if mem := container.Resources.Limits.Memory(); mem != nil && !mem.IsZero() {
				buf.WriteString(fmt.Sprintf("      memory:  %s\n", mem.String()))
			}
			if storage := container.Resources.Limits.StorageEphemeral(); storage != nil && !storage.IsZero() {
				buf.WriteString(fmt.Sprintf("      ephemeral-storage:  %s\n", storage.String()))
			}
		} else {
			buf.WriteString("      <none>\n")
		}

		buf.WriteString("    Requests:\n")
		if len(container.Resources.Requests) > 0 {
			if cpu := container.Resources.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
				buf.WriteString(fmt.Sprintf("      cpu:     %s\n", cpu.String()))
			}
			if mem := container.Resources.Requests.Memory(); mem != nil && !mem.IsZero() {
				buf.WriteString(fmt.Sprintf("      memory:  %s\n", mem.String()))
			}
			if storage := container.Resources.Requests.StorageEphemeral(); storage != nil && !storage.IsZero() {
				buf.WriteString(fmt.Sprintf("      ephemeral-storage:  %s\n", storage.String()))
			}
		} else {
			buf.WriteString("      <none>\n")
		}

		buf.WriteString("    Liveness:   ")
		if container.LivenessProbe != nil {
			buf.WriteString(d.formatProbe(container.LivenessProbe))
		} else {
			buf.WriteString("<none>")
		}
		buf.WriteString("\n")

		buf.WriteString("    Readiness:  ")
		if container.ReadinessProbe != nil {
			buf.WriteString(d.formatProbe(container.ReadinessProbe))
		} else {
			buf.WriteString("<none>")
		}
		buf.WriteString("\n")

		if container.StartupProbe != nil {
			buf.WriteString("    Startup:    ")
			buf.WriteString(d.formatProbe(container.StartupProbe))
			buf.WriteString("\n")
		}

		// Environment
		buf.WriteString("    Environment:\n")
		if len(container.Env) > 0 {
			for _, env := range container.Env {
				d.formatEnvironment(&buf, env)
			}
		} else {
			buf.WriteString("      <none>\n")
		}

		// Mounts
		buf.WriteString("    Mounts:\n")
		if len(container.VolumeMounts) > 0 {
			for _, mount := range container.VolumeMounts {
				buf.WriteString(fmt.Sprintf("      %s from %s (%s)\n",
					mount.MountPath, mount.Name, func() string {
						if mount.ReadOnly {
							return "ro"
						}
						return "rw"
					}()))
			}
		} else {
			buf.WriteString("      <none>\n")
		}
	}

	buf.WriteString("  Volumes:\n")
	if len(deployment.Spec.Template.Spec.Volumes) > 0 {
		for _, vol := range deployment.Spec.Template.Spec.Volumes {
			buf.WriteString(fmt.Sprintf("   %s:\n", vol.Name))
			if vol.ConfigMap != nil {
				buf.WriteString("    Type:      ConfigMap (a volume populated by a ConfigMap)\n")
				buf.WriteString(fmt.Sprintf("    Name:      %s\n", vol.ConfigMap.Name))
				if vol.ConfigMap.Optional != nil {
					buf.WriteString(fmt.Sprintf("    Optional:  %v\n", *vol.ConfigMap.Optional))
				}
			} else if vol.Secret != nil {
				buf.WriteString("    Type:      Secret (a volume populated by a Secret)\n")
				buf.WriteString(fmt.Sprintf("    Name:      %s\n", vol.Secret.SecretName))
				if vol.Secret.Optional != nil {
					buf.WriteString(fmt.Sprintf("    Optional:  %v\n", *vol.Secret.Optional))
				}
			} else if vol.PersistentVolumeClaim != nil {
				buf.WriteString("    Type:      PersistentVolumeClaim (a reference to a PersistentVolumeClaim in the same namespace)\n")
				buf.WriteString(fmt.Sprintf("    ClaimName: %s\n", vol.PersistentVolumeClaim.ClaimName))
				buf.WriteString(fmt.Sprintf("    ReadOnly:  %v\n", vol.PersistentVolumeClaim.ReadOnly))
			} else if vol.EmptyDir != nil {
				buf.WriteString("    Type:      EmptyDir (a temporary directory that shares a pod's lifetime)\n")
				if vol.EmptyDir.Medium != "" {
					buf.WriteString(fmt.Sprintf("    Medium:    %s\n", vol.EmptyDir.Medium))
				}
				if vol.EmptyDir.SizeLimit != nil && !vol.EmptyDir.SizeLimit.IsZero() {
					buf.WriteString(fmt.Sprintf("    SizeLimit: %s\n", vol.EmptyDir.SizeLimit.String()))
				}
			} else if vol.HostPath != nil {
				buf.WriteString("    Type:      HostPath (bare host directory volume)\n")
				buf.WriteString(fmt.Sprintf("    Path:      %s\n", vol.HostPath.Path))
				if vol.HostPath.Type != nil {
					buf.WriteString(fmt.Sprintf("    HostPathType: %s\n", *vol.HostPath.Type))
				}
			} else if vol.Projected != nil {
				buf.WriteString("    Type:      Projected (a volume that contains injected data from multiple sources)\n")
			}
		}
	} else {
		buf.WriteString("   <none>\n")
	}

	// ========== Conditions ==========
	buf.WriteString("Conditions:\n")
	buf.WriteString("  Type           Status  Reason\n")
	buf.WriteString("  ----           ------  ------\n")
	if len(deployment.Status.Conditions) > 0 {
		for _, cond := range deployment.Status.Conditions {
			buf.WriteString(fmt.Sprintf("  %-14s %-7s %s\n",
				cond.Type, cond.Status, cond.Reason))
		}
	} else {
		buf.WriteString("  <none>\n")
	}

	// ========== ReplicaSets ==========
	rsList, err := d.client.AppsV1().ReplicaSets(namespace).List(d.ctx, metav1.ListOptions{})
	if err == nil {
		var oldRS []string
		var newRS string

		for i := range rsList.Items {
			rs := &rsList.Items[i]

			// 检查是否属于当前 Deployment
			isOwned := false
			for _, owner := range rs.OwnerReferences {
				if owner.Kind == "Deployment" && owner.Name == deployment.Name && owner.UID == deployment.UID {
					isOwned = true
					break
				}
			}

			if !isOwned {
				continue
			}

			rsReplicas := int32(0)
			if rs.Spec.Replicas != nil {
				rsReplicas = *rs.Spec.Replicas
			}

			rsInfo := fmt.Sprintf("%s (%d/%d replicas created)", rs.Name, rs.Status.Replicas, rsReplicas)

			// 判断是否为新的 ReplicaSet（有副本）
			if rsReplicas > 0 {
				newRS = rsInfo
			} else {
				oldRS = append(oldRS, rsInfo)
			}
		}

		buf.WriteString("OldReplicaSets:  ")
		if len(oldRS) > 0 {
			buf.WriteString(strings.Join(oldRS, ", "))
			buf.WriteString("\n")
		} else {
			buf.WriteString("<none>\n")
		}

		buf.WriteString("NewReplicaSet:   ")
		if newRS != "" {
			buf.WriteString(newRS)
			buf.WriteString("\n")
		} else {
			buf.WriteString("<none>\n")
		}
	}

	// ========== Events ==========
	buf.WriteString("Events:\n")
	events, err := d.GetEvents(namespace, name)
	if err == nil && len(events) > 0 {
		buf.WriteString("  Type    Reason              Age                    From                   Message\n")
		buf.WriteString("  ----    ------              ----                   ----                   -------\n")

		// 只显示最近 10 条
		limit := 10
		if len(events) < limit {
			limit = len(events)
		}

		for i := 0; i < limit; i++ {
			event := events[i]

			var ageStr string
			if event.LastTimestamp > 0 {
				age := time.Since(time.UnixMilli(event.LastTimestamp)).Round(time.Second)
				ageStr = d.formatDuration(age)
			} else {
				ageStr = "<unknown>"
			}

			buf.WriteString(fmt.Sprintf("  %-7s %-19s %-22s %-22s %s\n",
				event.Type,
				event.Reason,
				ageStr,
				event.Source,
				event.Message,
			))
		}
	} else {
		buf.WriteString("  <none>\n")
	}

	return buf.String(), nil
}

func (d *deploymentOperator) formatEnvironment(buf *strings.Builder, env corev1.EnvVar) {
	if env.ValueFrom != nil {
		if env.ValueFrom.ConfigMapKeyRef != nil {
			buf.WriteString(fmt.Sprintf("      %s:  <set to the key '%s' in config map '%s'>",
				env.Name, env.ValueFrom.ConfigMapKeyRef.Key, env.ValueFrom.ConfigMapKeyRef.Name))
			if env.ValueFrom.ConfigMapKeyRef.Optional != nil {
				buf.WriteString(fmt.Sprintf("  Optional: %v", *env.ValueFrom.ConfigMapKeyRef.Optional))
			}
			buf.WriteString("\n")
		} else if env.ValueFrom.SecretKeyRef != nil {
			buf.WriteString(fmt.Sprintf("      %s:  <set to the key '%s' in secret '%s'>",
				env.Name, env.ValueFrom.SecretKeyRef.Key, env.ValueFrom.SecretKeyRef.Name))
			if env.ValueFrom.SecretKeyRef.Optional != nil {
				buf.WriteString(fmt.Sprintf("  Optional: %v", *env.ValueFrom.SecretKeyRef.Optional))
			}
			buf.WriteString("\n")
		} else if env.ValueFrom.FieldRef != nil {
			buf.WriteString(fmt.Sprintf("      %s:   (%s)\n", env.Name, env.ValueFrom.FieldRef.FieldPath))
		} else if env.ValueFrom.ResourceFieldRef != nil {
			buf.WriteString(fmt.Sprintf("      %s:  <set from resource: %s>\n",
				env.Name, env.ValueFrom.ResourceFieldRef.Resource))
		} else {
			buf.WriteString(fmt.Sprintf("      %s:  <set from source>\n", env.Name))
		}
	} else {
		buf.WriteString(fmt.Sprintf("      %s:  %s\n", env.Name, env.Value))
	}
}

// formatProbe 格式化探针信息
func (d *deploymentOperator) formatProbe(probe *corev1.Probe) string {
	var parts []string

	if probe.HTTPGet != nil {
		parts = append(parts, fmt.Sprintf("http-get %s:%d%s",
			probe.HTTPGet.Host, probe.HTTPGet.Port.IntVal, probe.HTTPGet.Path))
	} else if probe.TCPSocket != nil {
		parts = append(parts, fmt.Sprintf("tcp-socket :%d", probe.TCPSocket.Port.IntVal))
	} else if probe.Exec != nil {
		parts = append(parts, fmt.Sprintf("exec %v", probe.Exec.Command))
	} else if probe.GRPC != nil {
		parts = append(parts, fmt.Sprintf("grpc :%d", probe.GRPC.Port))
	}

	parts = append(parts, fmt.Sprintf("delay=%ds", probe.InitialDelaySeconds))
	parts = append(parts, fmt.Sprintf("timeout=%ds", probe.TimeoutSeconds))
	parts = append(parts, fmt.Sprintf("period=%ds", probe.PeriodSeconds))
	parts = append(parts, fmt.Sprintf("success=%d", probe.SuccessThreshold))
	parts = append(parts, fmt.Sprintf("failure=%d", probe.FailureThreshold))

	return strings.Join(parts, " ")
}

// formatDuration 格式化时间间隔
func (d *deploymentOperator) formatDuration(duration time.Duration) string {
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

// ListAll 获取所有 Deployment（优先使用 informer）
func (d *deploymentOperator) ListAll(namespace string) ([]appsv1.Deployment, error) {
	var deployments []*appsv1.Deployment
	var err error

	// 优先使用 informer
	if d.useInformer && d.deploymentLister != nil {
		if namespace == "" {
			// 获取所有 namespace 的 Deployment
			deployments, err = d.deploymentLister.List(labels.Everything())
		} else {
			// 获取指定 namespace 的 Deployment
			deployments, err = d.deploymentLister.Deployments(namespace).List(labels.Everything())
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
	result := make([]appsv1.Deployment, 0, len(deployments))
	for _, deployment := range deployments {
		if deployment != nil {
			result = append(result, *deployment)
		}
	}

	return result, nil
}

// listAllFromAPI 通过 API 直接获取 Deployment 列表（内部辅助方法）
func (d *deploymentOperator) listAllFromAPI(namespace string) ([]appsv1.Deployment, error) {
	deploymentList, err := d.client.AppsV1().Deployments(namespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		if namespace == "" {
			return nil, fmt.Errorf("获取所有Deployment失败: %v", err)
		}
		return nil, fmt.Errorf("获取命名空间 %s 的Deployment失败: %v", namespace, err)
	}

	return deploymentList.Items, nil
}

// GetResourceSummary 获取 Deployment 的资源摘要信息
// 包括关联的 Pod、Service、Ingress 等信息
func (d *deploymentOperator) GetResourceSummary(
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

	// 1. 获取 Deployment
	deployment, err := d.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	// 获取 Pod 选择器标签
	selectorLabels := deployment.Spec.Selector.MatchLabels
	if len(selectorLabels) == 0 {
		return nil, fmt.Errorf("Deployment 没有选择器标签")
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
