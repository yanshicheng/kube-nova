package operator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

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

type jobOperator struct {
	BaseOperator
	client          kubernetes.Interface
	informerFactory informers.SharedInformerFactory
	jobLister       v1.JobLister
	jobInformer     cache.SharedIndexInformer
}

func NewJobOperator(ctx context.Context, client kubernetes.Interface) types.JobOperator {
	return &jobOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

func NewJobOperatorWithInformer(
	ctx context.Context,
	client kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
) types.JobOperator {
	var jobLister v1.JobLister
	var jobInformer cache.SharedIndexInformer

	if informerFactory != nil {
		jobInformer = informerFactory.Batch().V1().Jobs().Informer()
		jobLister = informerFactory.Batch().V1().Jobs().Lister()
	}

	return &jobOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		jobLister:       jobLister,
		jobInformer:     jobInformer,
	}
}

func (j *jobOperator) Create(job *batchv1.Job) (*batchv1.Job, error) {
	if job == nil || job.Name == "" || job.Namespace == "" {
		return nil, fmt.Errorf("Job对象、名称和命名空间不能为空")
	}

	if job.Labels == nil {
		job.Labels = make(map[string]string)
	}
	if job.Annotations == nil {
		job.Annotations = make(map[string]string)
	}

	created, err := j.client.BatchV1().Jobs(job.Namespace).Create(j.ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("创建Job失败: %v", err)
	}

	return created, nil
}

func (j *jobOperator) Get(namespace, name string) (*batchv1.Job, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	if j.jobLister != nil {
		job, err := j.jobLister.Jobs(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Job %s/%s 不存在", namespace, name)
			}
			job, apiErr := j.client.BatchV1().Jobs(namespace).Get(j.ctx, name, metav1.GetOptions{})
			if apiErr != nil {
				return nil, fmt.Errorf("获取Job失败")
			}
			return job, nil
		}
		return job, nil
	}

	job, err := j.client.BatchV1().Jobs(namespace).Get(j.ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("Job %s/%s 不存在", namespace, name)
		}
		return nil, fmt.Errorf("获取Job失败")
	}

	return job, nil
}

func (j *jobOperator) Update(job *batchv1.Job) (*batchv1.Job, error) {
	if job == nil || job.Name == "" || job.Namespace == "" {
		return nil, fmt.Errorf("Job对象、名称和命名空间不能为空")
	}

	updated, err := j.client.BatchV1().Jobs(job.Namespace).Update(j.ctx, job, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("更新Job失败: %v", err)
	}

	return updated, nil
}

func (j *jobOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	propagationPolicy := metav1.DeletePropagationOrphan
	err := j.client.BatchV1().Jobs(namespace).Delete(j.ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除Job失败: %v", err)
	}

	return nil
}

func (j *jobOperator) DeleteWithPods(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	propagationPolicy := metav1.DeletePropagationBackground
	err := j.client.BatchV1().Jobs(namespace).Delete(j.ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除Job及其Pods失败: %v", err)
	}

	return nil
}

func (j *jobOperator) List(namespace string, req types.ListRequest) (*types.ListJobResponse, error) {
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
		req.SortBy = "creationTime"
	}

	var selector labels.Selector = labels.Everything()
	if req.Labels != "" {
		parsedSelector, err := labels.Parse(req.Labels)
		if err != nil {
			return nil, fmt.Errorf("解析标签选择器失败")
		}
		selector = parsedSelector
	}

	var jobs []*batchv1.Job
	var err error

	if j.useInformer && j.jobLister != nil {
		jobs, err = j.jobLister.Jobs(namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("获取Job列表失败")
		}
	} else {
		listOpts := metav1.ListOptions{LabelSelector: selector.String()}
		jobList, err := j.client.BatchV1().Jobs(namespace).List(j.ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("获取Job列表失败")
		}
		jobs = make([]*batchv1.Job, len(jobList.Items))
		for i := range jobList.Items {
			jobs[i] = &jobList.Items[i]
		}
	}

	if req.Search != "" {
		filtered := make([]*batchv1.Job, 0)
		searchLower := strings.ToLower(req.Search)
		for _, job := range jobs {
			if strings.Contains(strings.ToLower(job.Name), searchLower) {
				filtered = append(filtered, job)
			}
		}
		jobs = filtered
	}

	sort.Slice(jobs, func(i, j int) bool {
		var less bool
		switch req.SortBy {
		case "creationTime", "creationTimestamp":
			less = jobs[i].CreationTimestamp.Before(&jobs[j].CreationTimestamp)
		case "startTime":
			if jobs[i].Status.StartTime == nil {
				return false
			}
			if jobs[j].Status.StartTime == nil {
				return true
			}
			less = jobs[i].Status.StartTime.Before(jobs[j].Status.StartTime)
		default:
			less = jobs[i].Name < jobs[j].Name
		}
		if req.SortDesc {
			return !less
		}
		return less
	})

	total := len(jobs)
	totalPages := (total + req.PageSize - 1) / req.PageSize
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	if start >= total {
		return &types.ListJobResponse{
			ListResponse: types.ListResponse{
				Total:      total,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: totalPages,
			},
			Items: []types.JobInfo{},
		}, nil
	}

	if end > total {
		end = total
	}

	pageJobs := jobs[start:end]
	items := make([]types.JobInfo, len(pageJobs))
	for i, job := range pageJobs {
		items[i] = j.convertToJobInfo(job)
	}

	return &types.ListJobResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: items,
	}, nil
}

func (j *jobOperator) convertToJobInfo(job *batchv1.Job) types.JobInfo {

	completions := int32(1)
	if job.Spec.Completions != nil {
		completions = *job.Spec.Completions
	}

	parallelism := int32(1)
	if job.Spec.Parallelism != nil {
		parallelism = *job.Spec.Parallelism
	}

	var duration string
	var startTime *time.Time
	var completionTime *time.Time

	if job.Status.StartTime != nil {
		startTime = &job.Status.StartTime.Time
		if job.Status.CompletionTime != nil {
			completionTime = &job.Status.CompletionTime.Time
			duration = completionTime.Sub(*startTime).Round(time.Second).String()
		} else {
			duration = time.Since(*startTime).Round(time.Second).String()
		}
	}

	status := "Running"
	if job.Spec.Suspend != nil && *job.Spec.Suspend {
		status = "Suspended"
	} else if job.Status.CompletionTime != nil {
		status = "Completed"
	} else if job.Status.Failed > 0 {
		status = "Failed"
	}

	return types.JobInfo{
		Name:              job.Name,
		Namespace:         job.Namespace,
		Completions:       completions,
		Parallelism:       parallelism,
		Succeeded:         job.Status.Succeeded,
		Failed:            job.Status.Failed,
		Active:            job.Status.Active,
		StartTime:         startTime,
		CompletionTime:    completionTime,
		Duration:          duration,
		Status:            status,
		CreationTimestamp: job.CreationTimestamp.Time,
	}
}

func (j *jobOperator) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return j.client.BatchV1().Jobs(namespace).Watch(j.ctx, opts)
}

func (j *jobOperator) UpdateLabels(namespace, name string, labels map[string]string) error {
	job, err := j.Get(namespace, name)
	if err != nil {
		return err
	}

	if job.Labels == nil {
		job.Labels = make(map[string]string)
	}
	for k, v := range labels {
		job.Labels[k] = v
	}

	_, err = j.Update(job)
	return err
}

func (j *jobOperator) UpdateAnnotations(namespace, name string, annotations map[string]string) error {
	job, err := j.Get(namespace, name)
	if err != nil {
		return err
	}

	if job.Annotations == nil {
		job.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		job.Annotations[k] = v
	}

	_, err = j.Update(job)
	return err
}

func (j *jobOperator) GetYaml(namespace, name string) (string, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return "", err
	}

	// 设置 TypeMeta
	job.TypeMeta = metav1.TypeMeta{
		APIVersion: "batch/v1",
		Kind:       "Job",
	}
	job.ManagedFields = nil

	yamlBytes, err := yaml.Marshal(job)
	if err != nil {
		return "", fmt.Errorf("转换为YAML失败: %v", err)
	}

	return string(yamlBytes), nil
}

func (j *jobOperator) GetPods(namespace, name string) ([]types.PodDetailInfo, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	labelSelector := metav1.FormatLabelSelector(job.Spec.Selector)
	podList, err := j.client.CoreV1().Pods(namespace).List(j.ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("获取Pod列表失败: %v", err)
	}

	pods := make([]types.PodDetailInfo, 0, len(podList.Items))
	for i := range podList.Items {
		pod := &podList.Items[i]
		pods = append(pods, j.convertToPodDetailInfo(pod))
	}

	return pods, nil
}

func (j *jobOperator) convertToPodDetailInfo(pod *corev1.Pod) types.PodDetailInfo {
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

func (j *jobOperator) GetContainerImages(namespace, name string) (*types.ContainerInfoList, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	result := &types.ContainerInfoList{
		InitContainers: make([]types.ContainerInfo, 0),
		Containers:     make([]types.ContainerInfo, 0),
	}

	for _, container := range job.Spec.Template.Spec.InitContainers {
		result.InitContainers = append(result.InitContainers, types.ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		})
	}

	for _, container := range job.Spec.Template.Spec.Containers {
		result.Containers = append(result.Containers, types.ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		})
	}

	return result, nil
}

func (j *jobOperator) UpdateImage(req *types.UpdateImageRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.Image == "" {
		return fmt.Errorf("请求参数不完整")
	}

	job, err := j.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	updated := false
	for i := range job.Spec.Template.Spec.Containers {
		if req.ContainerName == "" || job.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
			job.Spec.Template.Spec.Containers[i].Image = req.Image
			updated = true
			if req.ContainerName != "" {
				break
			}
		}
	}

	if !updated {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = j.Update(job)
	return err
}

func (j *jobOperator) UpdateImages(req *types.UpdateImagesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	job, err := j.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	for _, img := range req.Containers.InitContainers {
		for i := range job.Spec.Template.Spec.InitContainers {
			if job.Spec.Template.Spec.InitContainers[i].Name == img.Name {
				job.Spec.Template.Spec.InitContainers[i].Image = img.Image
				break
			}
		}
	}

	for _, img := range req.Containers.Containers {
		for i := range job.Spec.Template.Spec.Containers {
			if job.Spec.Template.Spec.Containers[i].Name == img.Name {
				job.Spec.Template.Spec.Containers[i].Image = img.Image
				break
			}
		}
	}

	_, err = j.Update(job)
	return err
}

func (j *jobOperator) GetStatus(namespace, name string) (*types.JobStatusInfo, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	var duration string
	var startTime *time.Time
	var completionTime *time.Time

	if job.Status.StartTime != nil {
		startTime = &job.Status.StartTime.Time
		if job.Status.CompletionTime != nil {
			completionTime = &job.Status.CompletionTime.Time
			duration = completionTime.Sub(*startTime).Round(time.Second).String()
		} else {
			duration = time.Since(*startTime).Round(time.Second).String()
		}
	}

	status := "Running"
	if job.Spec.Suspend != nil && *job.Spec.Suspend {
		status = "Suspended"
	} else if job.Status.CompletionTime != nil {
		status = "Completed"
	} else if job.Status.Failed > 0 {
		status = "Failed"
	}

	return &types.JobStatusInfo{
		Active:         job.Status.Active,
		Succeeded:      job.Status.Succeeded,
		Failed:         job.Status.Failed,
		StartTime:      startTime,
		CompletionTime: completionTime,
		Duration:       duration,
		Status:         status,
	}, nil
}

func (j *jobOperator) GetParallelismConfig(namespace, name string) (*types.JobParallelismConfig, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	config := &types.JobParallelismConfig{}

	if job.Spec.Parallelism != nil {
		config.Parallelism = *job.Spec.Parallelism
	}

	if job.Spec.Completions != nil {
		config.Completions = *job.Spec.Completions
	}

	if job.Spec.BackoffLimit != nil {
		config.BackoffLimit = *job.Spec.BackoffLimit
	}

	if job.Spec.ActiveDeadlineSeconds != nil {
		config.ActiveDeadlineSeconds = *job.Spec.ActiveDeadlineSeconds
	}

	return config, nil
}

func (j *jobOperator) UpdateParallelismConfig(req *types.UpdateJobParallelismRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	job, err := j.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	if req.Parallelism != nil {
		job.Spec.Parallelism = req.Parallelism
	}
	if req.Completions != nil {
		job.Spec.Completions = req.Completions
	}
	if req.BackoffLimit != nil {
		job.Spec.BackoffLimit = req.BackoffLimit
	}
	if req.ActiveDeadlineSeconds != nil {
		job.Spec.ActiveDeadlineSeconds = req.ActiveDeadlineSeconds
	}

	_, err = j.Update(job)
	return err
}

func (j *jobOperator) GetEnvVars(namespace, name string) (*types.EnvVarsResponse, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.EnvVarsResponse{
		Containers: make([]types.ContainerEnvVars, 0),
	}

	for _, container := range job.Spec.Template.Spec.Containers {
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

func (j *jobOperator) UpdateEnvVars(req *types.UpdateEnvVarsRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.ContainerName == "" {
		return fmt.Errorf("请求参数不完整")
	}

	job, err := j.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	found := false
	for i := range job.Spec.Template.Spec.Containers {
		if job.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
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

			job.Spec.Template.Spec.Containers[i].Env = envVars
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = j.Update(job)
	return err
}

func (j *jobOperator) GetPauseStatus(namespace, name string) (*types.PauseStatusResponse, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	paused := false
	if job.Spec.Suspend != nil && *job.Spec.Suspend {
		paused = true
	}

	return &types.PauseStatusResponse{
		Paused:      paused,
		SupportType: "suspend",
	}, nil
}

func (j *jobOperator) Suspend(namespace, name string) error {
	job, err := j.Get(namespace, name)
	if err != nil {
		return err
	}

	suspend := true
	job.Spec.Suspend = &suspend

	_, err = j.Update(job)
	return err
}

func (j *jobOperator) Resume(namespace, name string) error {
	job, err := j.Get(namespace, name)
	if err != nil {
		return err
	}

	suspend := false
	job.Spec.Suspend = &suspend

	_, err = j.Update(job)
	return err
}

func (j *jobOperator) GetResources(namespace, name string) (*types.ResourcesResponse, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.ResourcesResponse{
		Containers: make([]types.ContainerResources, 0),
	}

	for _, container := range job.Spec.Template.Spec.Containers {
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

func (j *jobOperator) UpdateResources(req *types.UpdateResourcesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.ContainerName == "" {
		return fmt.Errorf("请求参数不完整")
	}

	job, err := j.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	found := false
	for i := range job.Spec.Template.Spec.Containers {
		if job.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
			if req.Resources.Limits.Cpu != "" {
				cpuLimit, err := resource.ParseQuantity(req.Resources.Limits.Cpu)
				if err != nil {
					return fmt.Errorf("解析CPU限制失败: %v", err)
				}
				if job.Spec.Template.Spec.Containers[i].Resources.Limits == nil {
					job.Spec.Template.Spec.Containers[i].Resources.Limits = corev1.ResourceList{}
				}
				job.Spec.Template.Spec.Containers[i].Resources.Limits[corev1.ResourceCPU] = cpuLimit
			}

			if req.Resources.Limits.Memory != "" {
				memLimit, err := resource.ParseQuantity(req.Resources.Limits.Memory)
				if err != nil {
					return fmt.Errorf("解析内存限制失败: %v", err)
				}
				if job.Spec.Template.Spec.Containers[i].Resources.Limits == nil {
					job.Spec.Template.Spec.Containers[i].Resources.Limits = corev1.ResourceList{}
				}
				job.Spec.Template.Spec.Containers[i].Resources.Limits[corev1.ResourceMemory] = memLimit
			}

			if req.Resources.Requests.Cpu != "" {
				cpuRequest, err := resource.ParseQuantity(req.Resources.Requests.Cpu)
				if err != nil {
					return fmt.Errorf("解析CPU请求失败: %v", err)
				}
				if job.Spec.Template.Spec.Containers[i].Resources.Requests == nil {
					job.Spec.Template.Spec.Containers[i].Resources.Requests = corev1.ResourceList{}
				}
				job.Spec.Template.Spec.Containers[i].Resources.Requests[corev1.ResourceCPU] = cpuRequest
			}

			if req.Resources.Requests.Memory != "" {
				memRequest, err := resource.ParseQuantity(req.Resources.Requests.Memory)
				if err != nil {
					return fmt.Errorf("解析内存请求失败: %v", err)
				}
				if job.Spec.Template.Spec.Containers[i].Resources.Requests == nil {
					job.Spec.Template.Spec.Containers[i].Resources.Requests = corev1.ResourceList{}
				}
				job.Spec.Template.Spec.Containers[i].Resources.Requests[corev1.ResourceMemory] = memRequest
			}

			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = j.Update(job)
	return err
}

func (j *jobOperator) GetProbes(namespace, name string) (*types.ProbesResponse, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	response := &types.ProbesResponse{
		Containers: make([]types.ContainerProbes, 0),
	}

	for _, container := range job.Spec.Template.Spec.Containers {
		containerProbes := types.ContainerProbes{
			ContainerName: container.Name,
		}

		if container.LivenessProbe != nil {
			containerProbes.LivenessProbe = j.convertProbe(container.LivenessProbe)
		}
		if container.ReadinessProbe != nil {
			containerProbes.ReadinessProbe = j.convertProbe(container.ReadinessProbe)
		}
		if container.StartupProbe != nil {
			containerProbes.StartupProbe = j.convertProbe(container.StartupProbe)
		}

		response.Containers = append(response.Containers, containerProbes)
	}

	return response, nil
}

func (j *jobOperator) convertProbe(probe *corev1.Probe) *types.Probe {
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

func (j *jobOperator) UpdateProbes(req *types.UpdateProbesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" || req.ContainerName == "" {
		return fmt.Errorf("请求参数不完整")
	}

	job, err := j.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	found := false
	for i := range job.Spec.Template.Spec.Containers {
		if job.Spec.Template.Spec.Containers[i].Name == req.ContainerName {
			if req.LivenessProbe != nil {
				job.Spec.Template.Spec.Containers[i].LivenessProbe = j.buildProbe(req.LivenessProbe)
			}
			if req.ReadinessProbe != nil {
				job.Spec.Template.Spec.Containers[i].ReadinessProbe = j.buildProbe(req.ReadinessProbe)
			}
			if req.StartupProbe != nil {
				job.Spec.Template.Spec.Containers[i].StartupProbe = j.buildProbe(req.StartupProbe)
			}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("未找到容器: %s", req.ContainerName)
	}

	_, err = j.Update(job)
	return err
}

func (j *jobOperator) buildProbe(probe *types.Probe) *corev1.Probe {
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

func (j *jobOperator) Stop(namespace, name string) error {
	if err := j.Suspend(namespace, name); err != nil {
		return err
	}

	job, err := j.Get(namespace, name)
	if err != nil {
		return err
	}

	labelSelector := metav1.FormatLabelSelector(job.Spec.Selector)
	podList, err := j.client.CoreV1().Pods(namespace).List(j.ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("获取Job的Pods失败: %v", err)
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
			_ = j.client.CoreV1().Pods(namespace).Delete(j.ctx, pod.Name, metav1.DeleteOptions{})
		}
	}

	return nil
}

func (j *jobOperator) Recreate(namespace, name string) (*batchv1.Job, error) {
	oldJob, err := j.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	newJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      oldJob.Labels,
			Annotations: oldJob.Annotations,
		},
		Spec: oldJob.Spec,
	}

	if err := j.DeleteWithPods(namespace, name); err != nil {
		return nil, fmt.Errorf("删除旧Job失败: %v", err)
	}

	time.Sleep(2 * time.Second)

	created, err := j.Create(newJob)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (j *jobOperator) IsCompleted(namespace, name string) (bool, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return false, err
	}

	return job.Status.CompletionTime != nil, nil
}

func (j *jobOperator) IsFailed(namespace, name string) (bool, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return false, err
	}

	return job.Status.Failed > 0 && job.Status.CompletionTime == nil, nil
}

func (j *jobOperator) IsSuspended(namespace, name string) (bool, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return false, err
	}

	return job.Spec.Suspend != nil && *job.Spec.Suspend, nil
}

// job.go - operator
func (j *jobOperator) GetPodLabels(namespace, name string) (map[string]string, error) {
	var job *batchv1.Job
	var err error

	if j.useInformer && j.jobLister != nil {
		job, err = j.jobLister.Jobs(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Job %s/%s 不存在", namespace, name)
			}
			job, err = j.client.BatchV1().Jobs(namespace).Get(j.ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取Job失败")
			}
		}
	} else {
		job, err = j.client.BatchV1().Jobs(namespace).Get(j.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Job %s/%s 不存在", namespace, name)
			}
			return nil, fmt.Errorf("获取Job失败")
		}
	}

	if job.Spec.Template.Labels == nil {
		return make(map[string]string), nil
	}

	labels := make(map[string]string)
	for k, v := range job.Spec.Template.Labels {
		labels[k] = v
	}
	return labels, nil
}

func (j *jobOperator) GetPodSelectorLabels(namespace, name string) (map[string]string, error) {
	var job *batchv1.Job
	var err error

	if j.useInformer && j.jobLister != nil {
		job, err = j.jobLister.Jobs(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("job %s/%s 不存在", namespace, name)
			}
			job, err = j.client.BatchV1().Jobs(namespace).Get(j.ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取Job失败")
			}
		}
	} else {
		job, err = j.client.BatchV1().Jobs(namespace).Get(j.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Job %s/%s 不存在", namespace, name)
			}
			return nil, fmt.Errorf("获取Job失败")
		}
	}

	if job.Spec.Selector == nil || job.Spec.Selector.MatchLabels == nil {
		return make(map[string]string), nil
	}

	labels := make(map[string]string)
	for k, v := range job.Spec.Selector.MatchLabels {
		labels[k] = v
	}
	return labels, nil
}

func (j *jobOperator) GetVersionStatus(namespace, name string) (*types.ResourceStatus, error) {
	var job *batchv1.Job
	var err error

	if j.useInformer && j.jobLister != nil {
		job, err = j.jobLister.Jobs(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Job %s/%s 不存在", namespace, name)
			}
			job, err = j.client.BatchV1().Jobs(namespace).Get(j.ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("获取Job失败")
			}
		}
	} else {
		job, err = j.client.BatchV1().Jobs(namespace).Get(j.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, fmt.Errorf("Job %s/%s 不存在", namespace, name)
			}
			return nil, fmt.Errorf("获取Job失败")
		}
	}

	status := &types.ResourceStatus{
		Ready: false,
	}

	// 挂起状态
	if job.Spec.Suspend != nil && *job.Spec.Suspend {
		status.Status = types.StatusStopped
		status.Message = "Job 已挂起"
		status.Ready = true
		return status, nil
	}

	completions := int32(1)
	if job.Spec.Completions != nil {
		completions = *job.Spec.Completions
	}

	// 已完成
	if job.Status.CompletionTime != nil {
		if job.Status.Succeeded >= completions {
			status.Status = types.StatusRunning
			status.Message = fmt.Sprintf("Job 已成功完成 (%d/%d)", job.Status.Succeeded, completions)
			status.Ready = true
			return status, nil
		}
	}

	// 失败
	if job.Status.Failed > 0 {
		backoffLimit := int32(6)
		if job.Spec.BackoffLimit != nil {
			backoffLimit = *job.Spec.BackoffLimit
		}
		if job.Status.Failed >= backoffLimit {
			status.Status = types.StatusError
			status.Message = fmt.Sprintf("Job 失败次数超过限制 (%d/%d)", job.Status.Failed, backoffLimit)
			return status, nil
		}
	}

	// 运行中
	if job.Status.Active > 0 {
		status.Status = types.StatusRunning
		status.Message = fmt.Sprintf("Job 运行中 (活跃: %d, 成功: %d/%d, 失败: %d)",
			job.Status.Active, job.Status.Succeeded, completions, job.Status.Failed)
		return status, nil
	}

	// 创建中
	if job.Status.Active == 0 && job.Status.Succeeded == 0 {
		age := time.Since(job.CreationTimestamp.Time)
		if age < 30*time.Second {
			status.Status = types.StatusCreating
			status.Message = "正在创建 Pod"
			return status, nil
		}
		status.Status = types.StatusError
		status.Message = "Job 没有活跃的 Pod"
		return status, nil
	}

	status.Status = types.StatusRunning
	status.Message = "运行中"
	return status, nil
}

// ==================== 调度配置相关 ====================

// GetSchedulingConfig 获取调度配置
func (j *jobOperator) GetSchedulingConfig(namespace, name string) (*types.SchedulingConfig, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return convertPodSpecToSchedulingConfig(&job.Spec.Template.Spec), nil
}

// UpdateSchedulingConfig 更新调度配置
func (j *jobOperator) UpdateSchedulingConfig(namespace, name string, config *types.UpdateSchedulingConfigRequest) error {
	if config == nil {
		return fmt.Errorf("调度配置不能为空")
	}

	job, err := j.Get(namespace, name)
	if err != nil {
		return err
	}

	// 更新 NodeSelector
	if config.NodeSelector != nil {
		job.Spec.Template.Spec.NodeSelector = config.NodeSelector
	}

	// 更新 NodeName
	if config.NodeName != "" {
		job.Spec.Template.Spec.NodeName = config.NodeName
	}

	// 更新 Affinity
	if config.Affinity != nil {
		job.Spec.Template.Spec.Affinity = convertAffinityConfigToK8s(config.Affinity)
	}

	// 更新 Tolerations
	if config.Tolerations != nil {
		job.Spec.Template.Spec.Tolerations = convertTolerationsConfigToK8s(config.Tolerations)
	}

	// 更新 TopologySpreadConstraints
	if config.TopologySpreadConstraints != nil {
		job.Spec.Template.Spec.TopologySpreadConstraints = convertTopologySpreadConstraintsToK8s(config.TopologySpreadConstraints)
	}

	// 更新 SchedulerName
	if config.SchedulerName != "" {
		job.Spec.Template.Spec.SchedulerName = config.SchedulerName
	}

	// 更新 PriorityClassName
	if config.PriorityClassName != "" {
		job.Spec.Template.Spec.PriorityClassName = config.PriorityClassName
	}

	// 更新 Priority
	if config.Priority != nil {
		job.Spec.Template.Spec.Priority = config.Priority
	}

	// 更新 RuntimeClassName
	if config.RuntimeClassName != nil {
		job.Spec.Template.Spec.RuntimeClassName = config.RuntimeClassName
	}

	_, err = j.Update(job)
	return err
}

// ==================== 存储配置相关 ====================

// GetStorageConfig 获取存储配置
func (j *jobOperator) GetStorageConfig(namespace, name string) (*types.StorageConfig, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	return convertPodSpecToStorageConfig(&job.Spec.Template.Spec), nil
}

// UpdateStorageConfig 更新存储配置
func (j *jobOperator) UpdateStorageConfig(namespace, name string, config *types.UpdateStorageConfigRequest) error {
	if config == nil {
		return fmt.Errorf("存储配置不能为空")
	}

	job, err := j.Get(namespace, name)
	if err != nil {
		return err
	}

	// 更新 Volumes
	if config.Volumes != nil {
		job.Spec.Template.Spec.Volumes = convertVolumesConfigToK8s(config.Volumes)
	}

	// 更新 VolumeMounts
	if config.VolumeMounts != nil {
		for _, vmConfig := range config.VolumeMounts {
			for i := range job.Spec.Template.Spec.Containers {
				if job.Spec.Template.Spec.Containers[i].Name == vmConfig.ContainerName {
					job.Spec.Template.Spec.Containers[i].VolumeMounts = convertVolumeMountsToK8s(vmConfig.Mounts)
					break
				}
			}
		}
	}

	_, err = j.Update(job)
	return err
}

// ==================== Events 相关 ====================

// GetEvents 获取 Job 的事件（已存在，确保实现正确）
func (j *jobOperator) GetEvents(namespace, name string) ([]types.EventInfo, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	eventList, err := j.client.CoreV1().Events(namespace).List(j.ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Job,involvedObject.uid=%s",
			name, job.UID),
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
func (j *jobOperator) GetDescribe(namespace, name string) (string, error) {
	job, err := j.Get(namespace, name)
	if err != nil {
		return "", err
	}

	var buf strings.Builder

	// ========== 基本信息 ==========
	buf.WriteString(fmt.Sprintf("Name:           %s\n", job.Name))
	buf.WriteString(fmt.Sprintf("Namespace:      %s\n", job.Namespace))

	if job.Spec.Selector != nil {
		buf.WriteString(fmt.Sprintf("Selector:       %s\n", metav1.FormatLabelSelector(job.Spec.Selector)))
	} else {
		buf.WriteString("Selector:       <none>\n")
	}

	// Labels
	buf.WriteString("Labels:         ")
	if len(job.Labels) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range job.Labels {
			if !first {
				buf.WriteString("                ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	}

	// Annotations
	buf.WriteString("Annotations:    ")
	if len(job.Annotations) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range job.Annotations {
			if !first {
				buf.WriteString("                ")
			}
			buf.WriteString(fmt.Sprintf("%s: %s\n", k, v))
			first = false
		}
	}

	// Controlled By
	if len(job.OwnerReferences) > 0 {
		owner := job.OwnerReferences[0]
		buf.WriteString(fmt.Sprintf("Controlled By:  %s/%s\n", owner.Kind, owner.Name))
	}

	parallelism := int32(1)
	if job.Spec.Parallelism != nil {
		parallelism = *job.Spec.Parallelism
	}

	completions := int32(1)
	if job.Spec.Completions != nil {
		completions = *job.Spec.Completions
	}

	buf.WriteString(fmt.Sprintf("Parallelism:    %d\n", parallelism))
	buf.WriteString(fmt.Sprintf("Completions:    %d\n", completions))

	if job.Spec.BackoffLimit != nil {
		buf.WriteString(fmt.Sprintf("Backoff Limit:  %d\n", *job.Spec.BackoffLimit))
	}

	if job.Status.StartTime != nil {
		buf.WriteString(fmt.Sprintf("Start Time:     %s\n", job.Status.StartTime.Format(time.RFC1123)))
	} else {
		buf.WriteString("Start Time:     <unset>\n")
	}

	if job.Status.CompletionTime != nil {
		buf.WriteString(fmt.Sprintf("Completed At:   %s\n", job.Status.CompletionTime.Format(time.RFC1123)))
	}

	if job.Spec.ActiveDeadlineSeconds != nil {
		buf.WriteString(fmt.Sprintf("Active Deadline Seconds:  %d\n", *job.Spec.ActiveDeadlineSeconds))
	}

	buf.WriteString(fmt.Sprintf("Pods Statuses:  %d Active / %d Succeeded / %d Failed\n",
		job.Status.Active, job.Status.Succeeded, job.Status.Failed))

	if job.Spec.CompletionMode != nil {
		buf.WriteString(fmt.Sprintf("Completion Mode:  %s\n", *job.Spec.CompletionMode))
	}

	// ========== Pod Template ==========
	buf.WriteString("Pod Template:\n")
	buf.WriteString("  Labels:  ")
	if len(job.Spec.Template.Labels) == 0 {
		buf.WriteString("<none>\n")
	} else {
		first := true
		for k, v := range job.Spec.Template.Labels {
			if !first {
				buf.WriteString("           ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	}

	if job.Spec.Template.Spec.ServiceAccountName != "" {
		buf.WriteString(fmt.Sprintf("  Service Account:  %s\n", job.Spec.Template.Spec.ServiceAccountName))
	} else {
		buf.WriteString("  Service Account:  default\n")
	}

	// Init Containers
	if len(job.Spec.Template.Spec.InitContainers) > 0 {
		buf.WriteString("  Init Containers:\n")
		for _, container := range job.Spec.Template.Spec.InitContainers {
			buf.WriteString(fmt.Sprintf("   %s:\n", container.Name))
			buf.WriteString(fmt.Sprintf("    Image:      %s\n", container.Image))

			if container.ImagePullPolicy != "" {
				buf.WriteString(fmt.Sprintf("    Image Pull Policy:  %s\n", container.ImagePullPolicy))
			}

			if len(container.Ports) > 0 {
				for _, port := range container.Ports {
					buf.WriteString(fmt.Sprintf("    Port:       %d/%s\n", port.ContainerPort, port.Protocol))
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
					if env.ValueFrom != nil {
						if env.ValueFrom.FieldRef != nil {
							buf.WriteString(fmt.Sprintf("      %s:   (%s)\n", env.Name, env.ValueFrom.FieldRef.FieldPath))
						} else if env.ValueFrom.SecretKeyRef != nil {
							buf.WriteString(fmt.Sprintf("      %s:  <set to the key '%s' in secret '%s'>\n",
								env.Name, env.ValueFrom.SecretKeyRef.Key, env.ValueFrom.SecretKeyRef.Name))
						} else if env.ValueFrom.ConfigMapKeyRef != nil {
							buf.WriteString(fmt.Sprintf("      %s:  <set to the key '%s' in config map '%s'>\n",
								env.Name, env.ValueFrom.ConfigMapKeyRef.Key, env.ValueFrom.ConfigMapKeyRef.Name))
						} else if env.ValueFrom.ResourceFieldRef != nil {
							buf.WriteString(fmt.Sprintf("      %s:  <set from resource>\n", env.Name))
						} else {
							buf.WriteString(fmt.Sprintf("      %s:  <set from source>\n", env.Name))
						}
					} else {
						buf.WriteString(fmt.Sprintf("      %s:  %s\n", env.Name, env.Value))
					}
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
	for _, container := range job.Spec.Template.Spec.Containers {
		buf.WriteString(fmt.Sprintf("   %s:\n", container.Name))
		buf.WriteString(fmt.Sprintf("    Image:      %s\n", container.Image))

		if container.ImagePullPolicy != "" {
			buf.WriteString(fmt.Sprintf("    Image Pull Policy:  %s\n", container.ImagePullPolicy))
		}

		if len(container.Ports) > 0 {
			for _, port := range container.Ports {
				buf.WriteString(fmt.Sprintf("    Port:       %d/%s\n", port.ContainerPort, port.Protocol))
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
			buf.WriteString(fmt.Sprintf("    Liveness:   %s\n", j.formatProbeForDescribe(container.LivenessProbe)))
		}
		if container.ReadinessProbe != nil {
			buf.WriteString(fmt.Sprintf("    Readiness:  %s\n", j.formatProbeForDescribe(container.ReadinessProbe)))
		}
		if container.StartupProbe != nil {
			buf.WriteString(fmt.Sprintf("    Startup:    %s\n", j.formatProbeForDescribe(container.StartupProbe)))
		}

		if len(container.Env) > 0 {
			buf.WriteString("    Environment:\n")
			for _, env := range container.Env {
				if env.ValueFrom != nil {
					if env.ValueFrom.FieldRef != nil {
						buf.WriteString(fmt.Sprintf("      %s:   (%s)\n", env.Name, env.ValueFrom.FieldRef.FieldPath))
					} else if env.ValueFrom.SecretKeyRef != nil {
						buf.WriteString(fmt.Sprintf("      %s:  <set to the key '%s' in secret '%s'>\n",
							env.Name, env.ValueFrom.SecretKeyRef.Key, env.ValueFrom.SecretKeyRef.Name))
					} else if env.ValueFrom.ConfigMapKeyRef != nil {
						buf.WriteString(fmt.Sprintf("      %s:  <set to the key '%s' in config map '%s'>\n",
							env.Name, env.ValueFrom.ConfigMapKeyRef.Key, env.ValueFrom.ConfigMapKeyRef.Name))
					} else if env.ValueFrom.ResourceFieldRef != nil {
						buf.WriteString(fmt.Sprintf("      %s:  <set from resource>\n", env.Name))
					} else {
						buf.WriteString(fmt.Sprintf("      %s:  <set from source>\n", env.Name))
					}
				} else {
					buf.WriteString(fmt.Sprintf("      %s:  %s\n", env.Name, env.Value))
				}
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
	if len(job.Spec.Template.Spec.Volumes) > 0 {
		for _, vol := range job.Spec.Template.Spec.Volumes {
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
			} else if vol.EmptyDir != nil {
				buf.WriteString("    Type:       EmptyDir (a temporary directory that shares a pod's lifetime)\n")
				if vol.EmptyDir.Medium != "" {
					buf.WriteString(fmt.Sprintf("    Medium:     %s\n", vol.EmptyDir.Medium))
				}
				if vol.EmptyDir.SizeLimit != nil && !vol.EmptyDir.SizeLimit.IsZero() {
					buf.WriteString(fmt.Sprintf("    SizeLimit:  %s\n", vol.EmptyDir.SizeLimit.String()))
				}
			} else if vol.HostPath != nil {
				buf.WriteString("    Type:       HostPath (bare host directory volume)\n")
				buf.WriteString(fmt.Sprintf("    Path:       %s\n", vol.HostPath.Path))
				if vol.HostPath.Type != nil {
					buf.WriteString(fmt.Sprintf("    HostPathType: %s\n", *vol.HostPath.Type))
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

	if job.Spec.Template.Spec.RestartPolicy != "" {
		buf.WriteString(fmt.Sprintf("  Restart Policy:  %s\n", job.Spec.Template.Spec.RestartPolicy))
	}

	// ========== Events ==========
	buf.WriteString("Events:\n")
	events, err := j.GetEvents(namespace, name)
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
				ageStr = j.formatDurationForDescribe(age)
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

func (j *jobOperator) formatProbeForDescribe(probe *corev1.Probe) string {
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
	}

	parts = append(parts, fmt.Sprintf("delay=%ds", probe.InitialDelaySeconds))
	parts = append(parts, fmt.Sprintf("timeout=%ds", probe.TimeoutSeconds))
	parts = append(parts, fmt.Sprintf("period=%ds", probe.PeriodSeconds))

	return strings.Join(parts, " ")
}

func (j *jobOperator) formatDurationForDescribe(duration time.Duration) string {
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

// GetJobsByCronJob 根据 CronJob 获取其创建的所有 Job
func (j *jobOperator) GetJobsByCronJob(namespace, cronJobName string) ([]types.JobInfo, error) {
	if namespace == "" || cronJobName == "" {
		return nil, fmt.Errorf("命名空间和 CronJob 名称不能为空")
	}

	// 首先获取 CronJob 以获取其 UID
	cronJob, err := j.client.BatchV1().CronJobs(namespace).Get(j.ctx, cronJobName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("CronJob %s/%s 不存在", namespace, cronJobName)
		}
		return nil, fmt.Errorf("获取 CronJob 失败: %v", err)
	}

	// 使用 OwnerReferences 查询关联的 Jobs
	var jobs []*batchv1.Job

	if j.useInformer && j.jobLister != nil {
		allJobs, err := j.jobLister.Jobs(namespace).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("从 Informer 获取 Job 列表失败: %v", err)
		}

		// 过滤出属于该 CronJob 的 Jobs
		for _, job := range allJobs {
			for _, owner := range job.OwnerReferences {
				if owner.Kind == "CronJob" && owner.Name == cronJobName && owner.UID == cronJob.UID {
					jobs = append(jobs, job)
					break
				}
			}
		}
	} else {
		jobList, err := j.client.BatchV1().Jobs(namespace).List(j.ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取 Job 列表失败: %v", err)
		}

		// 过滤出属于该 CronJob 的 Jobs
		for i := range jobList.Items {
			job := &jobList.Items[i]
			for _, owner := range job.OwnerReferences {
				if owner.Kind == "CronJob" && owner.Name == cronJobName && owner.UID == cronJob.UID {
					jobs = append(jobs, job)
					break
				}
			}
		}
	}

	// 按创建时间降序排序（最新的在前面）
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreationTimestamp.After(jobs[j].CreationTimestamp.Time)
	})

	// 转换为 JobInfo
	result := make([]types.JobInfo, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, j.convertToJobInfo(job))
	}

	return result, nil
}

// GetDetail 获取 Job 的详细信息
func (j *jobOperator) GetDetail(namespace, name string) (*types.JobDetailInfo, error) {
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("命名空间和名称不能为空")
	}

	// 获取 Job
	job, err := j.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	detail := &types.JobDetailInfo{
		Name:              job.Name,
		Namespace:         job.Namespace,
		Labels:            job.Labels,
		Annotations:       job.Annotations,
		CreationTimestamp: job.CreationTimestamp.Time,
		UID:               string(job.UID),
	}

	// OwnerReferences
	if len(job.OwnerReferences) > 0 {
		detail.OwnerReferences = make([]types.OwnerReference, 0, len(job.OwnerReferences))
		for _, owner := range job.OwnerReferences {
			detail.OwnerReferences = append(detail.OwnerReferences, types.OwnerReference{
				APIVersion: owner.APIVersion,
				Kind:       owner.Kind,
				Name:       owner.Name,
				UID:        string(owner.UID),
				Controller: owner.Controller != nil && *owner.Controller,
			})
		}
	}

	// Spec 配置
	detail.Parallelism = 1
	if job.Spec.Parallelism != nil {
		detail.Parallelism = *job.Spec.Parallelism
	}

	detail.Completions = 1
	if job.Spec.Completions != nil {
		detail.Completions = *job.Spec.Completions
	}

	detail.BackoffLimit = 6
	if job.Spec.BackoffLimit != nil {
		detail.BackoffLimit = *job.Spec.BackoffLimit
	}

	detail.ActiveDeadlineSeconds = job.Spec.ActiveDeadlineSeconds
	detail.TTLSecondsAfterFinish = job.Spec.TTLSecondsAfterFinished

	detail.Suspend = false
	if job.Spec.Suspend != nil && *job.Spec.Suspend {
		detail.Suspend = true
	}

	// 状态信息
	detail.Active = job.Status.Active
	detail.Succeeded = job.Status.Succeeded
	detail.Failed = job.Status.Failed

	if job.Status.StartTime != nil {
		startTime := job.Status.StartTime.Time
		detail.StartTime = &startTime

		if job.Status.CompletionTime != nil {
			completionTime := job.Status.CompletionTime.Time
			detail.CompletionTime = &completionTime
			detail.Duration = completionTime.Sub(startTime).Round(time.Second).String()
		} else {
			detail.Duration = time.Since(startTime).Round(time.Second).String()
		}
	}

	// 状态判断
	detail.Status = "Running"
	if detail.Suspend {
		detail.Status = "Suspended"
	} else if job.Status.CompletionTime != nil {
		detail.Status = "Completed"
	} else if job.Status.Failed > 0 {
		detail.Status = "Failed"
	}

	// Conditions
	if len(job.Status.Conditions) > 0 {
		detail.Conditions = make([]types.JobCondition, 0, len(job.Status.Conditions))
		for _, cond := range job.Status.Conditions {
			jobCond := types.JobCondition{
				Type:    string(cond.Type),
				Status:  string(cond.Status),
				Reason:  cond.Reason,
				Message: cond.Message,
			}
			// LastProbeTime 在 Job 中通常为空，检查是否有值
			if !cond.LastProbeTime.IsZero() {
				t := cond.LastProbeTime.Time
				jobCond.LastProbeTime = &t
			}
			// LastTransitionTime 是必填字段
			if !cond.LastTransitionTime.IsZero() {
				t := cond.LastTransitionTime.Time
				jobCond.LastTransitionTime = &t
			}
			detail.Conditions = append(detail.Conditions, jobCond)
		}
	}

	// UncountedTerminatedPods
	if job.Status.UncountedTerminatedPods != nil {
		// 转换 []types.UID 为 []string
		succeeded := make([]string, 0, len(job.Status.UncountedTerminatedPods.Succeeded))
		for _, uid := range job.Status.UncountedTerminatedPods.Succeeded {
			succeeded = append(succeeded, string(uid))
		}

		failed := make([]string, 0, len(job.Status.UncountedTerminatedPods.Failed))
		for _, uid := range job.Status.UncountedTerminatedPods.Failed {
			failed = append(failed, string(uid))
		}

		detail.UncountedTerminated = types.UncountedTerminatedPods{
			Succeeded: succeeded,
			Failed:    failed,
		}
	}

	// Pod 模板信息
	detail.PodTemplate = j.convertPodTemplateInfo(&job.Spec.Template)

	// 获取关联的 Pods
	pods, err := j.GetPods(namespace, name)
	if err == nil {
		detail.Pods = pods
	}

	// 获取事件
	events, err := j.GetEvents(namespace, name)
	if err == nil {
		detail.Events = events
	}

	return detail, nil
}

// convertPodTemplateInfo 转换 Pod 模板信息
func (j *jobOperator) convertPodTemplateInfo(template *corev1.PodTemplateSpec) types.PodTemplateInfo {
	info := types.PodTemplateInfo{
		Labels:         template.Labels,
		Annotations:    template.Annotations,
		ServiceAccount: template.Spec.ServiceAccountName,
		NodeSelector:   template.Spec.NodeSelector,
		NodeName:       template.Spec.NodeName,
		RestartPolicy:  string(template.Spec.RestartPolicy),
	}

	// 容器信息
	info.Containers = make([]types.ContainerDetailInfo, 0, len(template.Spec.Containers))
	for _, container := range template.Spec.Containers {
		info.Containers = append(info.Containers, j.convertContainerDetailInfo(&container))
	}

	// 初始化容器
	if len(template.Spec.InitContainers) > 0 {
		info.InitContainers = make([]types.ContainerDetailInfo, 0, len(template.Spec.InitContainers))
		for _, container := range template.Spec.InitContainers {
			info.InitContainers = append(info.InitContainers, j.convertContainerDetailInfo(&container))
		}
	}

	// 卷信息
	if len(template.Spec.Volumes) > 0 {
		info.Volumes = make([]types.VolumeInfo, 0, len(template.Spec.Volumes))
		for _, vol := range template.Spec.Volumes {
			volumeInfo := types.VolumeInfo{
				Name:         vol.Name,
				VolumeSource: make(map[string]interface{}),
			}

			// 简化卷源信息
			if vol.ConfigMap != nil {
				volumeInfo.VolumeSource["type"] = "configMap"
				volumeInfo.VolumeSource["name"] = vol.ConfigMap.Name
			} else if vol.Secret != nil {
				volumeInfo.VolumeSource["type"] = "secret"
				volumeInfo.VolumeSource["secretName"] = vol.Secret.SecretName
			} else if vol.EmptyDir != nil {
				volumeInfo.VolumeSource["type"] = "emptyDir"
			} else if vol.HostPath != nil {
				volumeInfo.VolumeSource["type"] = "hostPath"
				volumeInfo.VolumeSource["path"] = vol.HostPath.Path
			} else if vol.PersistentVolumeClaim != nil {
				volumeInfo.VolumeSource["type"] = "persistentVolumeClaim"
				volumeInfo.VolumeSource["claimName"] = vol.PersistentVolumeClaim.ClaimName
			}

			info.Volumes = append(info.Volumes, volumeInfo)
		}
	}

	// ImagePullSecrets
	if len(template.Spec.ImagePullSecrets) > 0 {
		info.ImagePullSecrets = make([]string, 0, len(template.Spec.ImagePullSecrets))
		for _, secret := range template.Spec.ImagePullSecrets {
			info.ImagePullSecrets = append(info.ImagePullSecrets, secret.Name)
		}
	}

	return info
}

// convertContainerDetailInfo 转换容器详细信息
func (j *jobOperator) convertContainerDetailInfo(container *corev1.Container) types.ContainerDetailInfo {
	info := types.ContainerDetailInfo{
		Name:            container.Name,
		Image:           container.Image,
		ImagePullPolicy: string(container.ImagePullPolicy),
		Command:         container.Command,
		Args:            container.Args,
		WorkingDir:      container.WorkingDir,
	}

	// 端口
	if len(container.Ports) > 0 {
		info.Ports = make([]types.ContainerPort, 0, len(container.Ports))
		for _, port := range container.Ports {
			info.Ports = append(info.Ports, types.ContainerPort{
				Name:          port.Name,
				ContainerPort: port.ContainerPort,
				Protocol:      string(port.Protocol),
				HostPort:      port.HostPort,
			})
		}
	}

	// 环境变量
	if len(container.Env) > 0 {
		info.Env = make([]types.EnvVar, 0, len(container.Env))
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
					if !env.ValueFrom.ResourceFieldRef.Divisor.IsZero() {
						divisor = env.ValueFrom.ResourceFieldRef.Divisor.String()
					}
					envVar.Source.ResourceFieldRef = &types.ResourceFieldSelector{
						ContainerName: env.ValueFrom.ResourceFieldRef.ContainerName,
						Resource:      env.ValueFrom.ResourceFieldRef.Resource,
						Divisor:       divisor,
					}
				}
			}

			info.Env = append(info.Env, envVar)
		}
	}

	// 资源配置
	info.Resources = types.ResourceRequirements{
		Limits: types.ResourceList{
			Cpu:    container.Resources.Limits.Cpu().String(),
			Memory: container.Resources.Limits.Memory().String(),
		},
		Requests: types.ResourceList{
			Cpu:    container.Resources.Requests.Cpu().String(),
			Memory: container.Resources.Requests.Memory().String(),
		},
	}

	// 卷挂载
	if len(container.VolumeMounts) > 0 {
		info.VolumeMounts = make([]types.JobVolumeMount, 0, len(container.VolumeMounts))
		for _, mount := range container.VolumeMounts {
			info.VolumeMounts = append(info.VolumeMounts, types.JobVolumeMount{
				Name:      mount.Name,
				MountPath: mount.MountPath,
				SubPath:   mount.SubPath,
				ReadOnly:  mount.ReadOnly,
			})
		}
	}

	// 健康检查
	if container.LivenessProbe != nil {
		info.LivenessProbe = j.convertProbe(container.LivenessProbe)
	}
	if container.ReadinessProbe != nil {
		info.ReadinessProbe = j.convertProbe(container.ReadinessProbe)
	}
	if container.StartupProbe != nil {
		info.StartupProbe = j.convertProbe(container.StartupProbe)
	}

	return info
}
