package operator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/yaml"
)

type cronJobOperator struct {
	BaseOperator
	client          kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	cronJobLister   v1.CronJobLister
	cronJobInformer cache.SharedIndexInformer
}

func NewCronJobOperator(ctx context.Context, client kubernetes.Interface) types.CronJobOperator {
	return &cronJobOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

func NewCronJobOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.CronJobOperator {
	var cronJobLister v1.CronJobLister
	var cronJobInformer cache.SharedIndexInformer

	if informerFactory != nil {
		cronJobInformer = informerFactory.Batch().V1().CronJobs().Informer()
		cronJobLister = informerFactory.Batch().V1().CronJobs().Lister()
	}

	return &cronJobOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		cronJobLister:   cronJobLister,
		cronJobInformer: cronJobInformer,
	}
}

func (c *cronJobOperator) Create(cronJob *batchv1.CronJob) (*batchv1.CronJob, error) {
	if cronJob == nil || cronJob.Name == "" || cronJob.Namespace == "" {
		return nil, fmt.Errorf("CronJob对象、名称和命名空间不能为空")
	}

	if cronJob.Labels == nil {
		cronJob.Labels = make(map[string]string)
	}
	if cronJob.Annotations == nil {
		cronJob.Annotations = make(map[string]string)
	}

	created, err := c.client.BatchV1().CronJobs(cronJob.Namespace).Create(c.ctx, cronJob, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建CronJob失败: %v", err)
	}

	return created, nil
}

func (c *cronJobOperator) Get(namespace, name string) (*batchv1.CronJob, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	if c.cronJobLister != nil {
		cronJob, err := c.cronJobLister.CronJobs(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("CronJob %s/%s 不存在", namespace, name)
			}
			cronJob, apiErr := c.client.BatchV1().CronJobs(namespace).Get(c.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取CronJob失败")
			}
			return cronJob, nil
		}
		return cronJob, nil
	}

	cronJob, err := c.client.BatchV1().CronJobs(namespace).Get(c.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("CronJob %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取CronJob失败")
	}

	return cronJob, nil
}

func (c *cronJobOperator) Update(cronJob *batchv1.CronJob) (*batchv1.CronJob, error) {
	if cronJob == nil || cronJob.Name == "" || cronJob.Namespace == "" {
		return nil, fmt.Errorf("CronJob对象、名称和命名空间不能为空")
	}

	updated, err := c.client.BatchV1().CronJobs(cronJob.Namespace).Update(c.ctx, cronJob, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新CronJob失败: %v", err)
	}

	return updated, nil
}

func (c *cronJobOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := c.client.BatchV1().CronJobs(namespace).Delete(c.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除CronJob失败: %v", err)
	}

	return nil
}

func (c *cronJobOperator) List(namespace string, req types.ListRequest) (*types.ListCronJobResponse, error) {
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

	var cronJobs []*batchv1.CronJob
	var err error

	if c.useInformer && c.cronJobLister != nil {
		cronJobs, err = c.cronJobLister.CronJobs(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取CronJob列表失败")
		}
	} else {
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		cronJobList, err := c.client.BatchV1().CronJobs(namespace).List(c.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取CronJob列表失败")
		}
		cronJobs = make([]*batchv1.CronJob, len(cronJobList.Items))
		for i := range cronJobList.Items {
			cronJobs[i] = &cronJobList.Items[i]
		}
	}

	if req.Search != "" {
		filtered := make([]*batchv1.CronJob, 0)
		searchLower := strings.ToLower(req.Search)
		for _, cj := range cronJobs {
			if strings.Contains(strings.ToLower(cj.Name), searchLower) {
				filtered = append(filtered, cj)
			}
		}
		cronJobs = filtered
	}

	sort.Slice(cronJobs, func(i, j int) bool {
		var less bool
		switch req.SortBy {
		case "creationTime", "creationTimestamp":
			less = cronJobs[i].CreationTimestamp.Before(&cronJobs[j].CreationTimestamp)
		case "schedule":
			less = cronJobs[i].Spec.Schedule < cronJobs[j].Spec.Schedule
		default:
			less = cronJobs[i].Name < cronJobs[j].Name
		}
		if req.SortDesc {
			return !less
		}
		return less
	})

	total := len(cronJobs)
	totalPages := (total + req.PageSize - 1) / req.PageSize
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	if start >= total {
		return &types.ListCronJobResponse{
			ListResponse: types.ListResponse{
				Total:      total,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: totalPages,
			},
			Items: []types.CronJobInfo{},
		}, nil
	}

	if end > total {
		end = total
	}

	pageCronJobs := cronJobs[start:end]
	items := make([]types.CronJobInfo, len(pageCronJobs))
	for i, cj := range pageCronJobs {
		items[i] = c.convertToCronJobInfo(cj)
	}

	return &types.ListCronJobResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: items,
	}, nil
}

func (c *cronJobOperator) convertToCronJobInfo(cronJob *batchv1.CronJob) types.CronJobInfo {
	images := make([]string, 0)
	for _, container := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
		images = append(images, container.Image)
	}

	suspend := false
	if cronJob.Spec.Suspend != nil {
		suspend = *cronJob.Spec.Suspend
	}

	timezone := ""
	if cronJob.Spec.TimeZone != nil {
		timezone = *cronJob.Spec.TimeZone
	}

	var lastScheduleTime *time.Time
	if cronJob.Status.LastScheduleTime != nil {
		lastScheduleTime = &cronJob.Status.LastScheduleTime.Time
	}

	var lastSuccessfulTime *time.Time
	if cronJob.Status.LastSuccessfulTime != nil {
		lastSuccessfulTime = &cronJob.Status.LastSuccessfulTime.Time
	}

	var nextScheduleTime *time.Time
	if !suspend {
		if schedule, err := cron.ParseStandard(cronJob.Spec.Schedule); err == nil {
			next := schedule.Next(time.Now())
			nextScheduleTime = &next
		}
	}

	return types.CronJobInfo{
		Name:               cronJob.Name,
		Namespace:          cronJob.Namespace,
		Schedule:           cronJob.Spec.Schedule,
		Timezone:           timezone,
		Suspend:            suspend,
		Active:             len(cronJob.Status.Active),
		LastScheduleTime:   lastScheduleTime,
		NextScheduleTime:   nextScheduleTime,
		LastSuccessfulTime: lastSuccessfulTime,
		CreationTimestamp:  cronJob.CreationTimestamp.Time,
		Images:             images,
	}
}

func (c *cronJobOperator) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.client.BatchV1().CronJobs(namespace).Watch(c.ctx, opts)
}

func (c *cronJobOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	if cronJob.Labels == nil {
		cronJob.Labels = make(map[string]string)
	}
	for k, v := range labels {
		cronJob.Labels[k] = v
	}

	_, err = c.Update(cronJob)
	return err
}

func (c *cronJobOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	if cronJob.Annotations == nil {
		cronJob.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		cronJob.Annotations[k] = v
	}

	_, err = c.Update(cronJob)
	return err
}

func (c *cronJobOperator) GetYaml(namespace, name string) (string, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return "", err
	}

	// 设置 TypeMeta
	cronJob.TypeMeta = metav1.TypeMeta{
		APIVersion: "batch/v1",
		Kind:       "CronJob",
	}
	cronJob.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(cronJob)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

func (c *cronJobOperator) GetPods(namespace, name string) ([]types.PodDetailInfo, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	jobList, err := c.client.BatchV1().Jobs(namespace).List(c.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取Job列表失败: %v", err)
	}

	allPods := make([]types.PodDetailInfo, 0)

	for i := range jobList.Items {
		job := &jobList.Items[i]

		isOwned := false
		for _, owner := range job.OwnerReferences {
			if owner.Kind == "CronJob" && owner.Name == cronJob.Name && owner.UID == cronJob.UID {
				isOwned = true
				break
			}
		}

		if !isOwned {
			continue
		}

		labelSelector := metav1.FormatLabelSelector(job.Spec.Selector)
		podList, err := c.client.CoreV1().Pods(namespace).List(c.ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			continue
		}

		for j := range podList.Items {
			pod := &podList.Items[j]
			allPods = append(allPods, c.convertToPodDetailInfo(pod))
		}
	}

	return allPods, nil
}

func (c *cronJobOperator) convertToPodDetailInfo(pod *corev1.Pod) types.PodDetailInfo {
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

func (c *cronJobOperator) GetContainerImages(namespace, name string) (*types.ContainerInfoList, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	result := &types.ContainerInfoList{
		InitContainers: make([]types.ContainerInfo, 0),
		Containers:     make([]types.ContainerInfo, 0),
	}

	for _, container := range cronJob.Spec.JobTemplate.Spec.Template.Spec.InitContainers {
		result.InitContainers = append(result.InitContainers, types.ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		})
	}

	for _, container := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
		result.Containers = append(result.Containers, types.ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		})
	}

	return result, nil
}

func (c *cronJobOperator) UpdateImage(req *types.UpdateImageRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.Image == "" {
		return fmt.Errorf("请求参数不完整")
	}

	cronJob, err := c.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	updated := false
	for i := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
		if req.ContainerName == "" || cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
			cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Image = req.Image
			updated = true
			if req.ContainerName != "" {
				break
			}
		}
	}

	if !updated {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = c.Update(cronJob)
	return err
}

func (c *cronJobOperator) UpdateImages(req *types.UpdateImagesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	cronJob, err := c.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	for _, img := range req.Containers.InitContainers {
		for i := range cronJob.Spec.JobTemplate.Spec.Template.Spec.InitContainers {
			if cronJob.Spec.JobTemplate.Spec.Template.Spec.InitContainers[i].Name == img.Name {
				cronJob.Spec.JobTemplate.Spec.Template.Spec.InitContainers[i].Image = img.Image
				break
			}
		}
	}

	for _, img := range req.Containers.Containers {
		for i := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
			if cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Name == img.Name {
				cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Image = img.Image
				break
			}
		}
	}

	_, err = c.Update(cronJob)
	return err
}

func (c *cronJobOperator) GetScheduleConfig(namespace, name string) (*types.CronJobScheduleConfig, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	suspend := false
	if cronJob.Spec.Suspend != nil {
		suspend = *cronJob.Spec.Suspend
	}

	timezone := ""
	if cronJob.Spec.TimeZone != nil {
		timezone = *cronJob.Spec.TimeZone
	}

	config := &types.CronJobScheduleConfig{
		Schedule:                   cronJob.Spec.Schedule,
		Timezone:                   timezone,
		ConcurrencyPolicy:          string(cronJob.Spec.ConcurrencyPolicy),
		Suspend:                    suspend,
		SuccessfulJobsHistoryLimit: *cronJob.Spec.SuccessfulJobsHistoryLimit,
		FailedJobsHistoryLimit:     *cronJob.Spec.FailedJobsHistoryLimit,
	}

	if cronJob.Spec.StartingDeadlineSeconds != nil {
		config.StartingDeadlineSeconds = *cronJob.Spec.StartingDeadlineSeconds
	}

	return config, nil
}

func (c *cronJobOperator) UpdateScheduleConfig(req *types.UpdateCronJobScheduleRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	cronJob, err := c.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	if req.Schedule != "" {
		if _, err := cron.ParseStandard(req.Schedule); err != nil {
			return fmt.Errorf("无效的Cron表达式: %v", err)
		}
		cronJob.Spec.Schedule = req.Schedule
	}

	if req.Timezone != "" {
		cronJob.Spec.TimeZone = &req.Timezone
	}

	if req.ConcurrencyPolicy != "" {
		cronJob.Spec.ConcurrencyPolicy = batchv1.ConcurrencyPolicy(req.ConcurrencyPolicy)
	}

	if req.SuccessfulJobsHistoryLimit != nil {
		cronJob.Spec.SuccessfulJobsHistoryLimit = req.SuccessfulJobsHistoryLimit
	}
	if req.FailedJobsHistoryLimit != nil {
		cronJob.Spec.FailedJobsHistoryLimit = req.FailedJobsHistoryLimit
	}

	if req.StartingDeadlineSeconds != nil {
		cronJob.Spec.StartingDeadlineSeconds = req.StartingDeadlineSeconds
	}

	_, err = c.Update(cronJob)
	return err
}

func (c *cronJobOperator) GetEnvVars(namespace, name string) (*types.EnvVarsResponse, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.EnvVarsResponse{
		Containers: make([]types.ContainerEnvVars, 0),
	}

	for _, container := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
		envVars := make([]types.EnvVar, 0)
		for _, env := range container.Env {
			envVar := types.EnvVar{
				Name:   env.Name,
				Source: types.EnvVarSource{Type: "value", Value: env.Value},
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

func (c *cronJobOperator) UpdateEnvVars(req *types.UpdateEnvVarsRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.ContainerName == "" {
		return fmt.Errorf("请求参数不完整")
	}

	cronJob, err := c.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	found := false
	for i := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
		if cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
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

			cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Env = envVars
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = c.Update(cronJob)
	return err
}

func (c *cronJobOperator) GetPauseStatus(namespace, name string) (*types.PauseStatusResponse, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	paused := false
	if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
		paused = true
	}

	return &types.PauseStatusResponse{
		Paused:      paused,
		SupportType: "suspend",
	}, nil
}

func (c *cronJobOperator) Suspend(namespace, name string) error {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	suspend := true
	cronJob.Spec.Suspend = &suspend

	_, err = c.Update(cronJob)
	return err
}

func (c *cronJobOperator) Resume(namespace, name string) error {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	suspend := false
	cronJob.Spec.Suspend = &suspend

	_, err = c.Update(cronJob)
	return err
}

func (c *cronJobOperator) GetResources(namespace, name string) (*types.ResourcesResponse, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.ResourcesResponse{
		Containers: make([]types.ContainerResources, 0),
	}

	for _, container := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
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

func (c *cronJobOperator) UpdateResources(req *types.UpdateResourcesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.ContainerName == "" {
		return fmt.Errorf("请求参数不完整")
	}

	cronJob, err := c.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	found := false
	for i := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
		if cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
			if req.Resources.Limits.Cpu != "" {
				cpuLimit, err := resource.ParseQuantity(req.Resources.Limits.Cpu)
				if err != nil {
					return fmt.Errorf("解析CPU限制失败: %v", err)
				}
				if cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Resources.Limits == nil {
					cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Resources.Limits = corev1.ResourceList{}
				}
				cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Resources.Limits[corev1.ResourceCPU] = cpuLimit
			}

			if req.Resources.Limits.Memory != "" {
				memLimit, err := resource.ParseQuantity(req.Resources.Limits.Memory)
				if err != nil {
					return fmt.Errorf("解析内存限制失败: %v", err)
				}
				if cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Resources.Limits == nil {
					cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Resources.Limits = corev1.ResourceList{}
				}
				cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Resources.Limits[corev1.ResourceMemory] = memLimit
			}

			if req.Resources.Requests.Cpu != "" {
				cpuRequest, err := resource.ParseQuantity(req.Resources.Requests.Cpu)
				if err != nil {
					return fmt.Errorf("解析CPU请求失败: %v", err)
				}
				if cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Resources.Requests == nil {
					cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Resources.Requests = corev1.ResourceList{}
				}
				cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Resources.Requests[corev1.ResourceCPU] = cpuRequest
			}

			if req.Resources.Requests.Memory != "" {
				memRequest, err := resource.ParseQuantity(req.Resources.Requests.Memory)
				if err != nil {
					return fmt.Errorf("解析内存请求失败: %v", err)
				}
				if cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Resources.Requests == nil {
					cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Resources.Requests = corev1.ResourceList{}
				}
				cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Resources.Requests[corev1.ResourceMemory] = memRequest
			}

			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = c.Update(cronJob)
	return err
}

func (c *cronJobOperator) GetProbes(namespace, name string) (*types.ProbesResponse, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.ProbesResponse{
		Containers: make([]types.ContainerProbes, 0),
	}

	for _, container := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
		containerProbes := types.ContainerProbes{
			ContainerName: container.Name,
		}

		if container.LivenessProbe != nil {
			containerProbes.LivenessProbe = c.convertProbe(container.LivenessProbe)
		}
		if container.ReadinessProbe != nil {
			containerProbes.ReadinessProbe = c.convertProbe(container.ReadinessProbe)
		}
		if container.StartupProbe != nil {
			containerProbes.StartupProbe = c.convertProbe(container.StartupProbe)
		}

		response.Containers = append(response.Containers, containerProbes)
	}

	return response, nil
}

func (c *cronJobOperator) convertProbe(probe *corev1.Probe) *types.Probe {
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

func (c *cronJobOperator) UpdateProbes(req *types.UpdateProbesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.ContainerName == "" {
		return fmt.Errorf("请求参数不完整")
	}

	cronJob, err := c.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	found := false
	for i := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
		if cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
			if req.LivenessProbe != nil {
				cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].LivenessProbe = c.buildProbe(req.LivenessProbe)
			}
			if req.ReadinessProbe != nil {
				cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].ReadinessProbe = c.buildProbe(req.ReadinessProbe)
			}
			if req.StartupProbe != nil {
				cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].StartupProbe = c.buildProbe(req.StartupProbe)
			}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = c.Update(cronJob)
	return err
}

func (c *cronJobOperator) buildProbe(probe *types.Probe) *corev1.Probe {
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

func (c *cronJobOperator) Stop(namespace, name string) error {
	return c.Suspend(namespace, name)
}

func (c *cronJobOperator) Start(namespace, name string) error {
	return c.Resume(namespace, name)
}

func (c *cronJobOperator) TriggerJob(req *types.TriggerCronJobRequest) (*batchv1.Job, error) {
	if req == nil || req.CronJobName == "" || req.Namespace == "" {
		return nil, fmt.Errorf("请求参数不完整")
	}

	cronJob, err := c.Get(req.Namespace, req.CronJobName)
	if err != nil {
		return nil, err
	}

	jobName := req.JobName
	if jobName == "" {
		jobName = fmt.Sprintf("%s-manual-%d", req.CronJobName, time.Now().Unix())
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: req.Namespace,
			Labels: map[string]string{
				"cronjob":    req.CronJobName,
				"manual-run": "true",
			},
			Annotations: map[string]string{
				"cronjob.kubernetes.io/instantiate": "manual",
				"triggered-at":                      time.Now().Format(time.RFC3339),
			},
		},
		Spec: cronJob.Spec.JobTemplate.Spec,
	}

	createdJob, err := c.client.BatchV1().Jobs(req.Namespace).Create(c.ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("手动触发Job失败: %v", err)
	}

	return createdJob, nil
}

// GetJobHistory 获取 CronJob 的所有 Job 历史记录（包括手动触发和定时触发）
func (c *cronJobOperator) GetJobHistory(namespace, name string) (*types.CronJobHistoryResponse, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	// 获取该命名空间下所有 Job
	jobList, err := c.client.BatchV1().Jobs(namespace).List(c.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取Job列表失败: %v", err)
	}

	history := make([]types.CronJobHistoryItem, 0)

	for i := range jobList.Items {
		job := &jobList.Items[i]

		// 判断 Job 是否属于该 CronJob - 使用多种通用方式
		belongsToCronJob := false

		// 🔥 方式1：通过 OwnerReferences（标准方式，K8s 自动创建的 Job）
		for _, owner := range job.OwnerReferences {
			if owner.Kind == "CronJob" && owner.Name == cronJob.Name && owner.UID == cronJob.UID {
				belongsToCronJob = true
				break
			}
		}

		// 🔥 方式2：通过 Job 名称前缀匹配（通用方式，适用于手动触发或其他方式创建的 Job）
		// Job 名称格式：{cronjob-name}-* 或 {cronjob-name}*
		if !belongsToCronJob {
			// 检查 Job 名称是否以 CronJob 名称开头
			if strings.HasPrefix(job.Name, cronJob.Name+"-") || strings.HasPrefix(job.Name, cronJob.Name) {
				belongsToCronJob = true
			}
		}

		// 不属于该 CronJob，跳过
		if !belongsToCronJob {
			continue
		}

		// 计算 Job 状态
		status := "Running"
		if job.Spec.Suspend != nil && *job.Spec.Suspend {
			status = "Suspended"
		} else if job.Status.CompletionTime != nil {
			// Job 已完成
			completions := int32(1)
			if job.Spec.Completions != nil {
				completions = *job.Spec.Completions
			}
			if job.Status.Succeeded >= completions {
				status = "Completed"
			} else {
				status = "Failed"
			}
		} else if job.Status.Active == 0 && job.Status.Failed > 0 {
			// 没有活跃 Pod 但有失败记录
			status = "Failed"
		}

		// 计算时间和持续时间
		var startTime int64
		var completionTime int64
		var duration string

		if job.Status.StartTime != nil {
			startTime = job.Status.StartTime.Unix()
			if job.Status.CompletionTime != nil {
				completionTime = job.Status.CompletionTime.Unix()
				d := job.Status.CompletionTime.Time.Sub(job.Status.StartTime.Time)
				duration = formatDuration(d)
			} else {
				// 还在运行中，计算当前持续时间
				d := time.Since(job.Status.StartTime.Time)
				duration = formatDuration(d)
			}
		}

		// 获取完成数配置
		completions := int32(1)
		if job.Spec.Completions != nil {
			completions = *job.Spec.Completions
		}

		// 构建历史项
		history = append(history, types.CronJobHistoryItem{
			Name:              job.Name,
			Status:            status,
			Completions:       completions,
			Succeeded:         job.Status.Succeeded,
			Active:            job.Status.Active,
			Failed:            job.Status.Failed,
			StartTime:         startTime,
			CompletionTime:    completionTime,
			Duration:          duration,
			CreationTimestamp: job.CreationTimestamp.Unix(),
		})
	}

	// 按创建时间降序排序（最新的在前面）
	sort.Slice(history, func(i, j int) bool {
		return history[i].CreationTimestamp > history[j].CreationTimestamp
	})

	return &types.CronJobHistoryResponse{
		Jobs: history,
	}, nil
}

// formatDuration 格式化持续时间为易读格式（如 "2m30s", "1h5m"）
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		if seconds > 0 {
			return fmt.Sprintf("%dm%ds", minutes, seconds)
		}
		return fmt.Sprintf("%dm", minutes)
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dh", hours)
}

// ==================== 手动触发 CronJob 的实现 ====================

// TriggerOnce 手动触发一次 CronJob（创建 Job）
// TriggerOnce 手动触发一次 CronJob（创建 Job）
func (c *cronJobOperator) TriggerOnce(namespace, name string) error {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	// 生成 Job 名称（格式：cronjob-name-manual-timestamp）
	jobName := fmt.Sprintf("%s-manual-%d", cronJob.Name, time.Now().Unix())

	// 创建 Job（基于 CronJob 的 JobTemplate）
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels:    cronJob.Spec.JobTemplate.Labels,
			Annotations: map[string]string{
				"cronjob.kubernetes.io/instantiate": "manual",
				"manual-trigger-time":               time.Now().Format(time.RFC3339),
			},
			// 🔥 添加 OwnerReferences，让 Job 归属于 CronJob
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "batch/v1",
					Kind:       "CronJob",
					Name:       cronJob.Name,
					UID:        cronJob.UID,
					Controller: func() *bool { b := false; return &b }(), // 不设置为 controller
				},
			},
		},
		Spec: cronJob.Spec.JobTemplate.Spec,
	}

	// 合并 CronJob JobTemplate 中的 Labels
	if cronJob.Spec.JobTemplate.Labels != nil {
		for k, v := range cronJob.Spec.JobTemplate.Labels {
			if job.Labels == nil {
				job.Labels = make(map[string]string)
			}
			if k != "cronjob" && k != "manual-run" {
				job.Labels[k] = v
			}
		}
	}

	// 创建 Job
	_, err = c.client.BatchV1().Jobs(namespace).Create(c.ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("手动触发 CronJob 失败: %v", err)
	}

	return nil
}

func (c *cronJobOperator) GetNextScheduleTime(namespace, name string) (*time.Time, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
		return nil, nil
	}

	schedule, err := cron.ParseStandard(cronJob.Spec.Schedule)
	if err != nil {
		return nil, fmt.Errorf("无效的Cron表达式: %v", err)
	}

	nextTime := schedule.Next(time.Now())
	return &nextTime, nil
}

func (c *cronJobOperator) GetScheduleExpression(namespace, name string) (string, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return "", err
	}

	return cronJob.Spec.Schedule, nil
}

// cronjob.go - operator
func (c *cronJobOperator) GetPodLabels(namespace, name string) (map[string]string, error) {
	var cronJob *batchv1.CronJob
	var err error

	if c.useInformer && c.cronJobLister != nil {
		cronJob, err = c.cronJobLister.CronJobs(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("CronJob %s/%s 不存在", namespace, name)
			}
			cronJob, err = c.client.BatchV1().CronJobs(namespace).Get(c.ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取CronJob失败")
			}
		}
	} else {
		cronJob, err = c.client.BatchV1().CronJobs(namespace).Get(c.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("CronJob %s/%s 不存在", namespace, name)
			}
			return nil, fmt.Errorf("获取CronJob失败")
		}
	}

	if cronJob.Spec.JobTemplate.Spec.Template.Labels == nil {
		return make(map[string]string), nil
	}

	labels := make(map[string]string)
	for k, v := range cronJob.Spec.JobTemplate.Spec.Template.Labels {
		labels[k] = v
	}
	return labels, nil
}

func (c *cronJobOperator) GetPodSelectorLabels(namespace, name string) (map[string]string, error) {
	var cronJob *batchv1.CronJob
	var err error

	if c.useInformer && c.cronJobLister != nil {
		cronJob, err = c.cronJobLister.CronJobs(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("CronJob %s/%s 不存在", namespace, name)
			}
			cronJob, err = c.client.BatchV1().CronJobs(namespace).Get(c.ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取CronJob失败")
			}
		}
	} else {
		cronJob, err = c.client.BatchV1().CronJobs(namespace).Get(c.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("CronJob %s/%s 不存在", namespace, name)
			}
			return nil, fmt.Errorf("获取CronJob失败")
		}
	}

	if cronJob.Spec.JobTemplate.Spec.Selector == nil ||
		cronJob.Spec.JobTemplate.Spec.Selector.MatchLabels == nil {
		return make(map[string]string), nil
	}

	labels := make(map[string]string)
	for k, v := range cronJob.Spec.JobTemplate.Spec.Selector.MatchLabels {
		labels[k] = v
	}
	return labels, nil
}

func (c *cronJobOperator) GetVersionStatus(namespace, name string) (*types.ResourceStatus, error) {
	var cronJob *batchv1.CronJob
	var err error

	if c.useInformer && c.cronJobLister != nil {
		cronJob, err = c.cronJobLister.CronJobs(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("CronJob %s/%s 不存在", namespace, name)
			}
			cronJob, err = c.client.BatchV1().CronJobs(namespace).Get(c.ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取CronJob失败")
			}
		}
	} else {
		cronJob, err = c.client.BatchV1().CronJobs(namespace).Get(c.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("CronJob %s/%s 不存在", namespace, name)
			}
			return nil, fmt.Errorf("获取CronJob失败")
		}
	}

	status := &types.ResourceStatus{
		Ready: false,
	}

	// 挂起状态
	if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
		status.Status = types.StatusStopped
		status.Message = "CronJob 已挂起"
		status.Ready = true
		return status, nil
	}

	// 有活跃的 Job
	if len(cronJob.Status.Active) > 0 {
		status.Status = types.StatusRunning
		status.Message = fmt.Sprintf("有 %d 个活跃 Job 正在运行", len(cronJob.Status.Active))
		return status, nil
	}

	// 运行中但没有活跃 Job
	status.Status = types.StatusRunning
	status.Ready = true

	if cronJob.Status.LastScheduleTime != nil {
		lastSchedule := cronJob.Status.LastScheduleTime.Time
		timeSince := time.Since(lastSchedule)
		status.Message = fmt.Sprintf("上次调度: %s 前", timeSince.Round(time.Second))
	} else {
		status.Message = "等待下次调度"
	}

	return status, nil
}

// ==================== 调度配置相关 ====================

// GetSchedulingConfig 获取调度配置
func (c *cronJobOperator) GetSchedulingConfig(namespace, name string) (*types.SchedulingConfig, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return convertPodSpecToSchedulingConfig(&cronJob.Spec.JobTemplate.Spec.Template.Spec), nil
}

// UpdateSchedulingConfig 更新调度配置
func (c *cronJobOperator) UpdateSchedulingConfig(namespace, name string, config *types.UpdateSchedulingConfigRequest) error {
	if config == nil {
		return fmt.Errorf("调度配置不能为空")
	}

	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	// 更新 NodeSelector
	if config.NodeSelector != nil {
		cronJob.Spec.JobTemplate.Spec.Template.Spec.NodeSelector = config.NodeSelector
	}

	// 更新 NodeName
	if config.NodeName != "" {
		cronJob.Spec.JobTemplate.Spec.Template.Spec.NodeName = config.NodeName
	}

	// 更新 Affinity
	if config.Affinity != nil {
		cronJob.Spec.JobTemplate.Spec.Template.Spec.Affinity = convertAffinityConfigToK8s(config.Affinity)
	}

	// 更新 Tolerations
	if config.Tolerations != nil {
		cronJob.Spec.JobTemplate.Spec.Template.Spec.Tolerations = convertTolerationsConfigToK8s(config.Tolerations)
	}

	// 更新 TopologySpreadConstraints
	if config.TopologySpreadConstraints != nil {
		cronJob.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints = convertTopologySpreadConstraintsToK8s(config.TopologySpreadConstraints)
	}

	// 更新 SchedulerName
	if config.SchedulerName != "" {
		cronJob.Spec.JobTemplate.Spec.Template.Spec.SchedulerName = config.SchedulerName
	}

	// 更新 PriorityClassName
	if config.PriorityClassName != "" {
		cronJob.Spec.JobTemplate.Spec.Template.Spec.PriorityClassName = config.PriorityClassName
	}

	// 更新 Priority
	if config.Priority != nil {
		cronJob.Spec.JobTemplate.Spec.Template.Spec.Priority = config.Priority
	}

	// 更新 RuntimeClassName
	if config.RuntimeClassName != nil {
		cronJob.Spec.JobTemplate.Spec.Template.Spec.RuntimeClassName = config.RuntimeClassName
	}

	_, err = c.Update(cronJob)
	return err
}

// ==================== 存储配置相关 ====================

// GetStorageConfig 获取存储配置
func (c *cronJobOperator) GetStorageConfig(namespace, name string) (*types.StorageConfig, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return convertPodSpecToStorageConfig(&cronJob.Spec.JobTemplate.Spec.Template.Spec), nil
}

// UpdateStorageConfig 更新存储配置
func (c *cronJobOperator) UpdateStorageConfig(namespace, name string, config *types.UpdateStorageConfigRequest) error {
	if config == nil {
		return fmt.Errorf("存储配置不能为空")
	}

	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	// 更新 Volumes
	if config.Volumes != nil {
		cronJob.Spec.JobTemplate.Spec.Template.Spec.Volumes = convertVolumesConfigToK8s(config.Volumes)
	}

	// 更新 VolumeMounts
	if config.VolumeMounts != nil {
		for _, vmConfig := range config.VolumeMounts {
			for i := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
				if cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Name == vmConfig.ContainerName {
					cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].VolumeMounts = convertVolumeMountsToK8s(vmConfig.Mounts)
					break
				}
			}
		}
	}

	_, err = c.Update(cronJob)
	return err
}

// ==================== Events 相关 ====================

// GetEvents 获取 CronJob 的事件（已存在，确保实现正确）
func (c *cronJobOperator) GetEvents(namespace, name string) ([]types.EventInfo, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	eventList, err := c.client.CoreV1().Events(namespace).List(c.ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=CronJob,involvedObject.uid=%s",
			name, cronJob.UID),
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

func (c *cronJobOperator) GetDescribe(namespace, name string) (string, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return "", err
	}

	var buf strings.Builder

	// ========== 基本信息 ==========
	buf.WriteString(fmt.Sprintf("Name:                          %s\n", cronJob.Name))
	buf.WriteString(fmt.Sprintf("Namespace:                     %s\n", cronJob.Namespace))

	// Labels
	buf.WriteString("Labels:                        ")
	if len(cronJob.Labels) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range cronJob.Labels {
			if !first {
				buf.WriteString("                               ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	}

	// Annotations
	buf.WriteString("Annotations:                   ")
	if len(cronJob.Annotations) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range cronJob.Annotations {
			if !first {
				buf.WriteString("                               ")
			}
			buf.WriteString(fmt.Sprintf("%s: %s\n", k, v))
			first = false
		}
	}

	buf.WriteString(fmt.Sprintf("Schedule:                      %s\n", cronJob.Spec.Schedule))

	// 修复：TimeZone 可能为 nil
	if cronJob.Spec.TimeZone != nil && *cronJob.Spec.TimeZone != "" {
		buf.WriteString(fmt.Sprintf("Time Zone:                     %s\n", *cronJob.Spec.TimeZone))
	}

	buf.WriteString(fmt.Sprintf("Concurrency Policy:            %s\n", cronJob.Spec.ConcurrencyPolicy))

	suspend := false
	if cronJob.Spec.Suspend != nil {
		suspend = *cronJob.Spec.Suspend
	}
	buf.WriteString(fmt.Sprintf("Suspend:                       %v\n", suspend))

	// 修复：SuccessfulJobsHistoryLimit 可能为 nil
	if cronJob.Spec.SuccessfulJobsHistoryLimit != nil {
		buf.WriteString(fmt.Sprintf("Successful Job History Limit:  %d\n", *cronJob.Spec.SuccessfulJobsHistoryLimit))
	} else {
		buf.WriteString("Successful Job History Limit:  3 (default)\n")
	}

	// 修复：FailedJobsHistoryLimit 可能为 nil
	if cronJob.Spec.FailedJobsHistoryLimit != nil {
		buf.WriteString(fmt.Sprintf("Failed Job History Limit:      %d\n", *cronJob.Spec.FailedJobsHistoryLimit))
	} else {
		buf.WriteString("Failed Job History Limit:      1 (default)\n")
	}

	// 修复：StartingDeadlineSeconds 可能为 nil
	if cronJob.Spec.StartingDeadlineSeconds != nil {
		buf.WriteString(fmt.Sprintf("Starting Deadline Seconds:     %d\n", *cronJob.Spec.StartingDeadlineSeconds))
	}

	// 修复：LastScheduleTime 可能为 nil
	if cronJob.Status.LastScheduleTime != nil {
		buf.WriteString(fmt.Sprintf("Last Schedule Time:            %s\n", cronJob.Status.LastScheduleTime.Format(time.RFC1123)))
	} else {
		buf.WriteString("Last Schedule Time:            <none>\n")
	}

	// 修复：LastSuccessfulTime 可能为 nil
	if cronJob.Status.LastSuccessfulTime != nil {
		buf.WriteString(fmt.Sprintf("Last Successful Time:          %s\n", cronJob.Status.LastSuccessfulTime.Format(time.RFC1123)))
	} else {
		buf.WriteString("Last Successful Time:          <none>\n")
	}

	buf.WriteString(fmt.Sprintf("Active Jobs:                   %d\n", len(cronJob.Status.Active)))

	// 修复：显示 Active Jobs 列表
	if len(cronJob.Status.Active) > 0 {
		buf.WriteString("Active Job References:\n")
		for _, ref := range cronJob.Status.Active {
			buf.WriteString(fmt.Sprintf("  - %s\n", ref.Name))
		}
	}

	// ========== Job Template ==========
	buf.WriteString("Job Template:\n")
	buf.WriteString("  Metadata:\n")
	buf.WriteString("    Labels:  ")
	if len(cronJob.Spec.JobTemplate.Labels) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range cronJob.Spec.JobTemplate.Labels {
			if !first {
				buf.WriteString("             ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	}

	// 修复：添加 Annotations 支持
	if len(cronJob.Spec.JobTemplate.Annotations) > 0 {
		buf.WriteString("    Annotations:  ")
		first := true
		for k, v := range cronJob.Spec.JobTemplate.Annotations {
			if !first {
				buf.WriteString("                  ")
			}
			buf.WriteString(fmt.Sprintf("%s: %s\n", k, v))
			first = false
		}
	}

	jobSpec := cronJob.Spec.JobTemplate.Spec

	parallelism := int32(1)
	if jobSpec.Parallelism != nil {
		parallelism = *jobSpec.Parallelism
	}

	completions := int32(1)
	if jobSpec.Completions != nil {
		completions = *jobSpec.Completions
	}

	buf.WriteString("  Spec:\n")
	buf.WriteString(fmt.Sprintf("    Parallelism:  %d\n", parallelism))
	buf.WriteString(fmt.Sprintf("    Completions:  %d\n", completions))

	// 修复：BackoffLimit 可能为 nil
	if jobSpec.BackoffLimit != nil {
		buf.WriteString(fmt.Sprintf("    Backoff Limit:  %d\n", *jobSpec.BackoffLimit))
	} else {
		buf.WriteString("    Backoff Limit:  6 (default)\n")
	}

	// 修复：添加 ActiveDeadlineSeconds
	if jobSpec.ActiveDeadlineSeconds != nil {
		buf.WriteString(fmt.Sprintf("    Active Deadline Seconds:  %d\n", *jobSpec.ActiveDeadlineSeconds))
	}

	// 修复：添加 CompletionMode
	if jobSpec.CompletionMode != nil {
		buf.WriteString(fmt.Sprintf("    Completion Mode:  %s\n", *jobSpec.CompletionMode))
	}

	// Pod Template
	buf.WriteString("    Pod Template:\n")
	buf.WriteString("      Labels:  ")
	if len(jobSpec.Template.Labels) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range jobSpec.Template.Labels {
			if !first {
				buf.WriteString("               ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	}

	// 修复：ServiceAccountName 可能为空
	if jobSpec.Template.Spec.ServiceAccountName != "" {
		buf.WriteString(fmt.Sprintf("      Service Account:  %s\n", jobSpec.Template.Spec.ServiceAccountName))
	} else {
		buf.WriteString("      Service Account:  default\n")
	}

	// 修复：添加 RestartPolicy
	if jobSpec.Template.Spec.RestartPolicy != "" {
		buf.WriteString(fmt.Sprintf("      Restart Policy:   %s\n", jobSpec.Template.Spec.RestartPolicy))
	}

	// Init Containers
	if len(jobSpec.Template.Spec.InitContainers) > 0 {
		buf.WriteString("      Init Containers:\n")
		for _, container := range jobSpec.Template.Spec.InitContainers {
			buf.WriteString(fmt.Sprintf("       %s:\n", container.Name))
			buf.WriteString(fmt.Sprintf("        Image:      %s\n", container.Image))

			// 修复：ImagePullPolicy
			if container.ImagePullPolicy != "" {
				buf.WriteString(fmt.Sprintf("        Image Pull Policy:  %s\n", container.ImagePullPolicy))
			}

			if len(container.Ports) > 0 {
				for _, port := range container.Ports {
					buf.WriteString(fmt.Sprintf("        Port:       %d/%s\n", port.ContainerPort, port.Protocol))
				}
			}

			// 修复：检查 Limits 是否存在
			if len(container.Resources.Limits) > 0 {
				buf.WriteString("        Limits:\n")
				if cpu := container.Resources.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
					buf.WriteString(fmt.Sprintf("          cpu:     %s\n", cpu.String()))
				}
				if mem := container.Resources.Limits.Memory(); mem != nil && !mem.IsZero() {
					buf.WriteString(fmt.Sprintf("          memory:  %s\n", mem.String()))
				}
				if storage := container.Resources.Limits.StorageEphemeral(); storage != nil && !storage.IsZero() {
					buf.WriteString(fmt.Sprintf("          ephemeral-storage:  %s\n", storage.String()))
				}
			}

			// 修复：检查 Requests 是否存在
			if len(container.Resources.Requests) > 0 {
				buf.WriteString("        Requests:\n")
				if cpu := container.Resources.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
					buf.WriteString(fmt.Sprintf("          cpu:     %s\n", cpu.String()))
				}
				if mem := container.Resources.Requests.Memory(); mem != nil && !mem.IsZero() {
					buf.WriteString(fmt.Sprintf("          memory:  %s\n", mem.String()))
				}
				if storage := container.Resources.Requests.StorageEphemeral(); storage != nil && !storage.IsZero() {
					buf.WriteString(fmt.Sprintf("          ephemeral-storage:  %s\n", storage.String()))
				}
			}

			// 修复：Environment 详细处理
			if len(container.Env) > 0 {
				buf.WriteString("        Environment:\n")
				for _, env := range container.Env {
					c.formatEnvironment(&buf, env, "          ")
				}
			}

			if len(container.VolumeMounts) > 0 {
				buf.WriteString("        Mounts:\n")
				for _, mount := range container.VolumeMounts {
					buf.WriteString(fmt.Sprintf("          %s from %s (%s)\n", mount.MountPath, mount.Name, func() string {
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
	buf.WriteString("      Containers:\n")
	for _, container := range jobSpec.Template.Spec.Containers {
		buf.WriteString(fmt.Sprintf("       %s:\n", container.Name))
		buf.WriteString(fmt.Sprintf("        Image:      %s\n", container.Image))

		// 修复：ImagePullPolicy
		if container.ImagePullPolicy != "" {
			buf.WriteString(fmt.Sprintf("        Image Pull Policy:  %s\n", container.ImagePullPolicy))
		}

		if len(container.Ports) > 0 {
			for _, port := range container.Ports {
				buf.WriteString(fmt.Sprintf("        Port:       %d/%s\n", port.ContainerPort, port.Protocol))
			}
		}

		// 修复：Command 和 Args
		if len(container.Command) > 0 {
			buf.WriteString("        Command:\n")
			for _, cmd := range container.Command {
				buf.WriteString(fmt.Sprintf("          %s\n", cmd))
			}
		}

		if len(container.Args) > 0 {
			buf.WriteString("        Args:\n")
			for _, arg := range container.Args {
				buf.WriteString(fmt.Sprintf("          %s\n", arg))
			}
		}

		// 修复：检查 Limits 是否存在
		if len(container.Resources.Limits) > 0 {
			buf.WriteString("        Limits:\n")
			if cpu := container.Resources.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
				buf.WriteString(fmt.Sprintf("          cpu:     %s\n", cpu.String()))
			}
			if mem := container.Resources.Limits.Memory(); mem != nil && !mem.IsZero() {
				buf.WriteString(fmt.Sprintf("          memory:  %s\n", mem.String()))
			}
			if storage := container.Resources.Limits.StorageEphemeral(); storage != nil && !storage.IsZero() {
				buf.WriteString(fmt.Sprintf("          ephemeral-storage:  %s\n", storage.String()))
			}
		}

		// 修复：检查 Requests 是否存在
		if len(container.Resources.Requests) > 0 {
			buf.WriteString("        Requests:\n")
			if cpu := container.Resources.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
				buf.WriteString(fmt.Sprintf("          cpu:     %s\n", cpu.String()))
			}
			if mem := container.Resources.Requests.Memory(); mem != nil && !mem.IsZero() {
				buf.WriteString(fmt.Sprintf("          memory:  %s\n", mem.String()))
			}
			if storage := container.Resources.Requests.StorageEphemeral(); storage != nil && !storage.IsZero() {
				buf.WriteString(fmt.Sprintf("          ephemeral-storage:  %s\n", storage.String()))
			}
		}

		// 修复：添加 Probes 支持
		if container.LivenessProbe != nil {
			buf.WriteString(fmt.Sprintf("        Liveness:   %s\n", c.formatProbeForDescribe(container.LivenessProbe)))
		}
		if container.ReadinessProbe != nil {
			buf.WriteString(fmt.Sprintf("        Readiness:  %s\n", c.formatProbeForDescribe(container.ReadinessProbe)))
		}
		if container.StartupProbe != nil {
			buf.WriteString(fmt.Sprintf("        Startup:    %s\n", c.formatProbeForDescribe(container.StartupProbe)))
		}

		// 修复：Environment 详细处理
		if len(container.Env) > 0 {
			buf.WriteString("        Environment:\n")
			for _, env := range container.Env {
				c.formatEnvironment(&buf, env, "          ")
			}
		}

		if len(container.VolumeMounts) > 0 {
			buf.WriteString("        Mounts:\n")
			for _, mount := range container.VolumeMounts {
				buf.WriteString(fmt.Sprintf("          %s from %s (%s)\n", mount.MountPath, mount.Name, func() string {
					if mount.ReadOnly {
						return "ro"
					}
					return "rw"
				}()))
			}
		}
	}

	// Volumes - 修复：增加更多 Volume 类型支持
	buf.WriteString("      Volumes:\n")
	if len(jobSpec.Template.Spec.Volumes) > 0 {
		for _, vol := range jobSpec.Template.Spec.Volumes {
			buf.WriteString(fmt.Sprintf("       %s:\n", vol.Name))
			if vol.ConfigMap != nil {
				buf.WriteString("        Type:       ConfigMap (a volume populated by a ConfigMap)\n")
				buf.WriteString(fmt.Sprintf("        Name:       %s\n", vol.ConfigMap.Name))
				if vol.ConfigMap.Optional != nil {
					buf.WriteString(fmt.Sprintf("        Optional:   %v\n", *vol.ConfigMap.Optional))
				}
			} else if vol.Secret != nil {
				buf.WriteString("        Type:       Secret (a volume populated by a Secret)\n")
				buf.WriteString(fmt.Sprintf("        SecretName: %s\n", vol.Secret.SecretName))
				if vol.Secret.Optional != nil {
					buf.WriteString(fmt.Sprintf("        Optional:   %v\n", *vol.Secret.Optional))
				}
			} else if vol.EmptyDir != nil {
				buf.WriteString("        Type:       EmptyDir (a temporary directory that shares a pod's lifetime)\n")
				if vol.EmptyDir.Medium != "" {
					buf.WriteString(fmt.Sprintf("        Medium:     %s\n", vol.EmptyDir.Medium))
				}
				if vol.EmptyDir.SizeLimit != nil && !vol.EmptyDir.SizeLimit.IsZero() {
					buf.WriteString(fmt.Sprintf("        SizeLimit:  %s\n", vol.EmptyDir.SizeLimit.String()))
				}
			} else if vol.HostPath != nil {
				buf.WriteString("        Type:       HostPath (bare host directory volume)\n")
				buf.WriteString(fmt.Sprintf("        Path:       %s\n", vol.HostPath.Path))
				if vol.HostPath.Type != nil {
					buf.WriteString(fmt.Sprintf("        HostPathType: %s\n", *vol.HostPath.Type))
				}
			} else if vol.PersistentVolumeClaim != nil {
				buf.WriteString("        Type:       PersistentVolumeClaim (a reference to a PersistentVolumeClaim in the same namespace)\n")
				buf.WriteString(fmt.Sprintf("        ClaimName:  %s\n", vol.PersistentVolumeClaim.ClaimName))
				buf.WriteString(fmt.Sprintf("        ReadOnly:   %v\n", vol.PersistentVolumeClaim.ReadOnly))
			} else if vol.Projected != nil {
				buf.WriteString("        Type:       Projected (a volume that contains injected data from multiple sources)\n")
			}
		}
	} else {
		buf.WriteString("       <none>\n")
	}

	// ========== Events ==========
	buf.WriteString("Events:\n")
	events, err := c.GetEvents(namespace, name)
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
				ageStr = c.formatDurationForDescribe(age)
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
func (c *cronJobOperator) formatEnvironment(buf *strings.Builder, env corev1.EnvVar, indent string) {
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
func (c *cronJobOperator) formatProbeForDescribe(probe *corev1.Probe) string {
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

func (c *cronJobOperator) formatDurationForDescribe(duration time.Duration) string {
	if duration < 0 {
		return "0s"
	}
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

// GetJobSpec 获取 Job Spec 配置
func (c *cronJobOperator) GetJobSpec(namespace, name string) (*types.JobSpecConfig, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	jobSpec := cronJob.Spec.JobTemplate.Spec

	// 默认值处理
	completionMode := "NonIndexed"
	if jobSpec.CompletionMode != nil {
		completionMode = string(*jobSpec.CompletionMode)
	}

	suspend := false
	if jobSpec.Suspend != nil {
		suspend = *jobSpec.Suspend
	}

	podReplacementPolicy := "TerminatingOrFailed"
	if jobSpec.PodReplacementPolicy != nil {
		podReplacementPolicy = string(*jobSpec.PodReplacementPolicy)
	}

	config := &types.JobSpecConfig{
		Parallelism:             jobSpec.Parallelism,
		Completions:             jobSpec.Completions,
		BackoffLimit:            jobSpec.BackoffLimit,
		ActiveDeadlineSeconds:   jobSpec.ActiveDeadlineSeconds,
		TTLSecondsAfterFinished: jobSpec.TTLSecondsAfterFinished,
		CompletionMode:          completionMode,
		Suspend:                 suspend,
		PodReplacementPolicy:    podReplacementPolicy,
		BackoffLimitPerIndex:    jobSpec.BackoffLimitPerIndex,
		MaxFailedIndexes:        jobSpec.MaxFailedIndexes,
	}

	// 转换 PodFailurePolicy
	if jobSpec.PodFailurePolicy != nil {
		config.PodFailurePolicy = c.convertPodFailurePolicyFromK8s(jobSpec.PodFailurePolicy)
	}

	return config, nil
}

// UpdateJobSpec 更新 Job Spec 配置
func (c *cronJobOperator) UpdateJobSpec(req *types.UpdateJobSpecRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	cronJob, err := c.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	// 更新基础字段
	if req.Parallelism != nil {
		cronJob.Spec.JobTemplate.Spec.Parallelism = req.Parallelism
	}

	if req.Completions != nil {
		cronJob.Spec.JobTemplate.Spec.Completions = req.Completions
	}

	if req.BackoffLimit != nil {
		cronJob.Spec.JobTemplate.Spec.BackoffLimit = req.BackoffLimit
	}

	if req.ActiveDeadlineSeconds != nil {
		cronJob.Spec.JobTemplate.Spec.ActiveDeadlineSeconds = req.ActiveDeadlineSeconds
	}

	if req.TTLSecondsAfterFinished != nil {
		cronJob.Spec.JobTemplate.Spec.TTLSecondsAfterFinished = req.TTLSecondsAfterFinished
	}

	// 更新完成模式
	if req.CompletionMode != nil {
		mode := batchv1.CompletionMode(*req.CompletionMode)
		cronJob.Spec.JobTemplate.Spec.CompletionMode = &mode
	}

	// 更新暂停标志
	if req.Suspend != nil {
		cronJob.Spec.JobTemplate.Spec.Suspend = req.Suspend
	}

	// 更新 Pod 替换策略
	if req.PodReplacementPolicy != nil {
		policy := batchv1.PodReplacementPolicy(*req.PodReplacementPolicy)
		cronJob.Spec.JobTemplate.Spec.PodReplacementPolicy = &policy
	}

	// 更新 Indexed 模式专用字段
	if req.BackoffLimitPerIndex != nil {
		cronJob.Spec.JobTemplate.Spec.BackoffLimitPerIndex = req.BackoffLimitPerIndex
	}

	if req.MaxFailedIndexes != nil {
		cronJob.Spec.JobTemplate.Spec.MaxFailedIndexes = req.MaxFailedIndexes
	}

	// 更新 PodFailurePolicy
	if req.PodFailurePolicy != nil {
		cronJob.Spec.JobTemplate.Spec.PodFailurePolicy = c.convertPodFailurePolicyToK8s(req.PodFailurePolicy)
	}

	_, err = c.Update(cronJob)
	return err
}

// convertPodFailurePolicyFromK8s 从 K8s 格式转换为内部格式
func (c *cronJobOperator) convertPodFailurePolicyFromK8s(policy *batchv1.PodFailurePolicy) *types.PodFailurePolicyConfig {
	if policy == nil {
		return nil
	}

	config := &types.PodFailurePolicyConfig{
		Rules: make([]types.PodFailurePolicyRule, 0),
	}

	for _, rule := range policy.Rules {
		policyRule := types.PodFailurePolicyRule{
			Action: string(rule.Action),
		}

		if rule.OnExitCodes != nil {
			policyRule.OnExitCodes = &types.PodFailurePolicyOnExitCodesRequirement{
				ContainerName: rule.OnExitCodes.ContainerName,
				Operator:      string(rule.OnExitCodes.Operator),
				Values:        rule.OnExitCodes.Values,
			}
		}

		if len(rule.OnPodConditions) > 0 {
			policyRule.OnPodConditions = make([]types.PodFailurePolicyOnPodConditionsPattern, 0)
			for _, condition := range rule.OnPodConditions {
				policyRule.OnPodConditions = append(policyRule.OnPodConditions, types.PodFailurePolicyOnPodConditionsPattern{
					Type:   string(condition.Type),
					Status: string(condition.Status),
				})
			}
		}

		config.Rules = append(config.Rules, policyRule)
	}

	return config
}

// convertPodFailurePolicyToK8s 从内部格式转换为 K8s 格式
func (c *cronJobOperator) convertPodFailurePolicyToK8s(config *types.PodFailurePolicyConfig) *batchv1.PodFailurePolicy {
	if config == nil {
		return nil
	}

	policy := &batchv1.PodFailurePolicy{
		Rules: make([]batchv1.PodFailurePolicyRule, 0),
	}

	for _, rule := range config.Rules {
		k8sRule := batchv1.PodFailurePolicyRule{
			Action: batchv1.PodFailurePolicyAction(rule.Action),
		}

		if rule.OnExitCodes != nil {
			k8sRule.OnExitCodes = &batchv1.PodFailurePolicyOnExitCodesRequirement{
				ContainerName: rule.OnExitCodes.ContainerName,
				Operator:      batchv1.PodFailurePolicyOnExitCodesOperator(rule.OnExitCodes.Operator),
				Values:        rule.OnExitCodes.Values,
			}
		}

		if len(rule.OnPodConditions) > 0 {
			k8sRule.OnPodConditions = make([]batchv1.PodFailurePolicyOnPodConditionsPattern, 0)
			for _, condition := range rule.OnPodConditions {
				k8sRule.OnPodConditions = append(k8sRule.OnPodConditions, batchv1.PodFailurePolicyOnPodConditionsPattern{
					Type:   corev1.PodConditionType(condition.Type),
					Status: corev1.ConditionStatus(condition.Status),
				})
			}
		}

		policy.Rules = append(policy.Rules, k8sRule)
	}

	return policy
}

// ==================== 下次执行时间 ====================

// GetNextScheduleTimeInfo 获取下次调度时间详细信息
func (c *cronJobOperator) GetNextScheduleTimeInfo(namespace, name string) (*types.NextScheduleTimeResponse, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.NextScheduleTimeResponse{
		Schedule:    cronJob.Spec.Schedule,
		CurrentTime: time.Now(),
		IsSuspended: false,
	}

	// 获取时区
	if cronJob.Spec.TimeZone != nil {
		response.Timezone = *cronJob.Spec.TimeZone
	}

	// 检查是否挂起
	if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
		response.IsSuspended = true
		return response, nil
	}

	// 解析 Cron 表达式
	schedule, err := cron.ParseStandard(cronJob.Spec.Schedule)
	if err != nil {
		return nil, fmt.Errorf("解析Cron表达式失败: %v", err)
	}

	// 计算下次调度时间
	now := time.Now()

	// 如果配置了时区，使用指定时区
	if response.Timezone != "" {
		loc, err := time.LoadLocation(response.Timezone)
		if err == nil {
			now = now.In(loc)
			response.CurrentTime = now
		}
	}

	nextTime := schedule.Next(now)
	response.NextScheduleTime = &nextTime

	return response, nil
}

// ListAll 获取所有 CronJob（优先使用 informer）
func (c *cronJobOperator) ListAll(namespace string) ([]batchv1.CronJob, error) {
	var cronJobs []*batchv1.CronJob
	var err error

	// 优先使用 informer
	if c.useInformer && c.cronJobLister != nil {
		if namespace == "" {
			// 获取所有 namespace 的 CronJob
			cronJobs, err = c.cronJobLister.List(labels.Everything())
		} else {
			// 获取指定 namespace 的 CronJob
			cronJobs, err = c.cronJobLister.CronJobs(namespace).List(labels.Everything())
		}

		if err != nil {
			// informer 失败，降级到 API 调用
			return c.listAllFromAPI(namespace)
		}
	} else {
		// 直接使用 API 调用
		return c.listAllFromAPI(namespace)
	}

	// 转换为非指针切片
	result := make([]batchv1.CronJob, 0, len(cronJobs))
	for _, cronJob := range cronJobs {
		if cronJob != nil {
			result = append(result, *cronJob)
		}
	}

	return result, nil
}

// listAllFromAPI 通过 API 直接获取 CronJob 列表（内部辅助方法）
func (c *cronJobOperator) listAllFromAPI(namespace string) ([]batchv1.CronJob, error) {
	cronJobList, err := c.client.BatchV1().CronJobs(namespace).List(c.ctx, metav1.ListOptions{})
	if err != nil {
		if namespace == "" {
			return nil, fmt.Errorf("获取所有CronJob失败: %v", err)
		}
		return nil, fmt.Errorf("获取命名空间 %s 的CronJob失败: %v", namespace, err)
	}

	return cronJobList.Items, nil
}

// GetResourceSummary 获取 CronJob 的资源摘要信息
func (c *cronJobOperator) GetResourceSummary(
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

	// 1. 获取 CronJob
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	// 获取 Pod 模板标签（CronJob 创建的 Job 会继承这些标签）
	podLabels := cronJob.Spec.JobTemplate.Spec.Template.Labels
	if len(podLabels) == 0 {
		return nil, fmt.Errorf("CronJob 没有 Pod 标签")
	}

	// 使用通用辅助函数获取摘要
	return getWorkloadResourceSummary(
		namespace,
		podLabels,
		domainSuffix,
		nodeLb,
		podOp,
		svcOp,
		ingressOp,
	)
}
