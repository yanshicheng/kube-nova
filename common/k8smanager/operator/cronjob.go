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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
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
	injectCommonAnnotations(cronJob)
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

	var selector k8slabels.Selector = k8slabels.Everything()
	if req.Labels != "" {
		parsedSelector, err := k8slabels.Parse(req.Labels)
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

func (c *cronJobOperator) UpdateLabels(namespace, name string, labelMap map[string]string) error {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	if cronJob.Labels == nil {
		cronJob.Labels = make(map[string]string)
	}
	for k, v := range labelMap {
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

// ==================== 镜像管理 ====================

// GetContainerImages 获取所有容器的镜像信息
// 使用公共转换函数 ConvertPodSpecToContainerImages
func (c *cronJobOperator) GetContainerImages(namespace, name string) (*types.ContainerInfoList, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	podSpec := &cronJob.Spec.JobTemplate.Spec.Template.Spec
	return ConvertPodSpecToContainerImages(podSpec), nil
}

func (c *cronJobOperator) UpdateImage(req *types.UpdateImageRequest) error {
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}
	if req.Namespace == "" {
		return fmt.Errorf("命名空间不能为空")
	}
	if req.Name == "" {
		return fmt.Errorf("CronJob名称不能为空")
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

	cronJob, err := c.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	var oldImage string
	found := false

	for i := range cronJob.Spec.JobTemplate.Spec.Template.Spec.InitContainers {
		container := &cronJob.Spec.JobTemplate.Spec.Template.Spec.InitContainers[i]
		if container.Name == req.ContainerName {
			oldImage = container.Image
			container.Image = req.Image
			found = true
			break
		}
	}

	if !found {
		for i := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
			container := &cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i]
			if container.Name == req.ContainerName {
				oldImage = container.Image
				container.Image = req.Image
				found = true
				break
			}
		}
	}

	if !found {
		for i := range cronJob.Spec.JobTemplate.Spec.Template.Spec.EphemeralContainers {
			container := &cronJob.Spec.JobTemplate.Spec.Template.Spec.EphemeralContainers[i]
			if container.Name == req.ContainerName {
				oldImage = container.Image
				container.Image = req.Image
				found = true
				break
			}
		}
	}

	if !found {
		availableContainers := c.getAvailableContainerNames(cronJob)
		return fmt.Errorf("未找到容器 '%s'，可用容器: %v", req.ContainerName, availableContainers)
	}

	if oldImage == req.Image {
		return nil
	}

	if cronJob.Annotations == nil {
		cronJob.Annotations = make(map[string]string)
	}

	changeCause := req.Reason
	if changeCause == "" {
		changeCause = fmt.Sprintf("image updated: %s %s -> %s",
			req.ContainerName, extractImageTag(oldImage), extractImageTag(req.Image))
	}
	cronJob.Annotations["kubernetes.io/change-cause"] = changeCause

	_, err = c.Update(cronJob)
	if err != nil {
		return fmt.Errorf("更新CronJob失败: %v", err)
	}

	return nil
}

// UpdateImages 批量更新容器镜像
// 使用公共转换函数 ApplyImagesToPodSpec
// 注意：req.Containers 是 CommContainerInfoList 类型，只有一个 Containers 字段
func (c *cronJobOperator) UpdateImages(req *types.UpdateImagesRequest) error {
	if req == nil {
		return fmt.Errorf("请求不能为空")
	}
	if req.Namespace == "" {
		return fmt.Errorf("命名空间不能为空")
	}
	if req.Name == "" {
		return fmt.Errorf("CronJob名称不能为空")
	}

	// CommContainerInfoList 只有 Containers 字段
	if len(req.Containers.Containers) == 0 {
		return fmt.Errorf("未指定要更新的容器")
	}

	// 验证所有镜像格式
	for _, cont := range req.Containers.Containers {
		if err := validateImageFormat(cont.Image); err != nil {
			return fmt.Errorf("容器 '%s' 镜像格式无效: %v", cont.ContainerName, err)
		}
	}

	cronJob, err := c.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	podSpec := &cronJob.Spec.JobTemplate.Spec.Template.Spec

	// 使用公共转换函数应用镜像更新
	// req.Containers.Containers 已经是 []ContainerImageInfo 类型
	changes, err := ApplyImagesToPodSpec(podSpec, req.Containers.Containers)
	if err != nil {
		return fmt.Errorf("应用镜像更新失败: %v", err)
	}

	if len(changes) == 0 {
		return nil
	}

	if cronJob.Annotations == nil {
		cronJob.Annotations = make(map[string]string)
	}

	changeCause := req.Reason
	if changeCause == "" {
		changeCause = fmt.Sprintf("images updated: %s", strings.Join(changes, ", "))
	}
	cronJob.Annotations["kubernetes.io/change-cause"] = changeCause

	_, err = c.Update(cronJob)
	if err != nil {
		return fmt.Errorf("更新CronJob失败: %v", err)
	}

	return nil
}

func (c *cronJobOperator) getAvailableContainerNames(cronJob *batchv1.CronJob) []string {
	names := make([]string, 0)
	for _, cont := range cronJob.Spec.JobTemplate.Spec.Template.Spec.InitContainers {
		names = append(names, cont.Name+" (init)")
	}
	for _, cont := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
		names = append(names, cont.Name)
	}
	for _, cont := range cronJob.Spec.JobTemplate.Spec.Template.Spec.EphemeralContainers {
		names = append(names, cont.Name+" (ephemeral)")
	}
	return names
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

// ==================== 环境变量管理 ====================

// GetEnvVars 获取所有容器的环境变量
// 使用公共转换函数 ConvertPodSpecToEnvVars
func (c *cronJobOperator) GetEnvVars(namespace, name string) (*types.EnvVarsResponse, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	podSpec := &cronJob.Spec.JobTemplate.Spec.Template.Spec
	return ConvertPodSpecToEnvVars(podSpec), nil
}

// UpdateEnvVars 全量更新所有容器的环境变量
// 使用公共转换函数 ApplyEnvVarsToPodSpec
func (c *cronJobOperator) UpdateEnvVars(req *types.UpdateEnvVarsRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	if len(req.Containers) == 0 {
		return fmt.Errorf("未指定要更新的容器")
	}

	cronJob, err := c.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	podSpec := &cronJob.Spec.JobTemplate.Spec.Template.Spec

	if err := ApplyEnvVarsToPodSpec(podSpec, req.Containers); err != nil {
		return fmt.Errorf("应用环境变量更新失败: %v", err)
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

// ==================== 资源配额管理 ====================

// GetResources 获取所有容器的资源配额
// 使用公共转换函数 ConvertPodSpecToResources
func (c *cronJobOperator) GetResources(namespace, name string) (*types.ResourcesResponse, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	podSpec := &cronJob.Spec.JobTemplate.Spec.Template.Spec
	return ConvertPodSpecToResources(podSpec), nil
}

// UpdateResources 全量更新所有容器的资源配额
// 使用公共转换函数 ApplyResourcesToPodSpec
func (c *cronJobOperator) UpdateResources(req *types.UpdateResourcesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	if len(req.Containers) == 0 {
		return fmt.Errorf("未指定要更新的容器")
	}

	cronJob, err := c.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	podSpec := &cronJob.Spec.JobTemplate.Spec.Template.Spec

	if err := ApplyResourcesToPodSpec(podSpec, req.Containers); err != nil {
		return fmt.Errorf("应用资源配额更新失败: %v", err)
	}

	_, err = c.Update(cronJob)
	return err
}

// ==================== 健康检查管理 ====================

// GetProbes 获取所有容器的健康检查配置
// 使用公共转换函数 ConvertPodSpecToProbes
func (c *cronJobOperator) GetProbes(namespace, name string) (*types.ProbesResponse, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	podSpec := &cronJob.Spec.JobTemplate.Spec.Template.Spec
	return ConvertPodSpecToProbes(podSpec), nil
}

// UpdateProbes 全量更新所有容器的健康检查配置
// 使用公共转换函数 ApplyProbesToPodSpec
func (c *cronJobOperator) UpdateProbes(req *types.UpdateProbesRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	if len(req.Containers) == 0 {
		return fmt.Errorf("未指定要更新的容器")
	}

	cronJob, err := c.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	podSpec := &cronJob.Spec.JobTemplate.Spec.Template.Spec

	if err := ApplyProbesToPodSpec(podSpec, req.Containers); err != nil {
		return fmt.Errorf("应用健康检查更新失败: %v", err)
	}

	_, err = c.Update(cronJob)
	return err
}

// ==================== 高级配置管理 ====================

// GetAdvancedConfig 获取高级容器配置
// 使用公共转换函数 ConvertPodSpecToAdvancedConfig
func (c *cronJobOperator) GetAdvancedConfig(namespace, name string) (*types.AdvancedConfigResponse, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	podSpec := &cronJob.Spec.JobTemplate.Spec.Template.Spec
	return ConvertPodSpecToAdvancedConfig(podSpec), nil
}

// UpdateAdvancedConfig 更新高级容器配置
// 使用公共转换函数 ApplyAdvancedConfigToPodSpec
func (c *cronJobOperator) UpdateAdvancedConfig(req *types.UpdateAdvancedConfigRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	cronJob, err := c.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

	podSpec := &cronJob.Spec.JobTemplate.Spec.Template.Spec

	if err := ApplyAdvancedConfigToPodSpec(podSpec, req); err != nil {
		return fmt.Errorf("应用高级配置更新失败: %v", err)
	}

	// 验证 CronJob 的 RestartPolicy 限制
	if podSpec.RestartPolicy != "" &&
		podSpec.RestartPolicy != corev1.RestartPolicyOnFailure &&
		podSpec.RestartPolicy != corev1.RestartPolicyNever {
		return fmt.Errorf("CronJob 的 RestartPolicy 只能是 'OnFailure' 或 'Never'，当前值: %s", podSpec.RestartPolicy)
	}

	_, err = c.Update(cronJob)
	return err
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

func (c *cronJobOperator) GetJobHistory(namespace, name string) (*types.CronJobHistoryResponse, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	jobList, err := c.client.BatchV1().Jobs(namespace).List(c.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取Job列表失败: %v", err)
	}

	history := make([]types.CronJobHistoryItem, 0)

	for i := range jobList.Items {
		job := &jobList.Items[i]

		belongsToCronJob := false

		for _, owner := range job.OwnerReferences {
			if owner.Kind == "CronJob" && owner.Name == cronJob.Name && owner.UID == cronJob.UID {
				belongsToCronJob = true
				break
			}
		}

		if !belongsToCronJob {
			if strings.HasPrefix(job.Name, cronJob.Name+"-") || strings.HasPrefix(job.Name, cronJob.Name) {
				belongsToCronJob = true
			}
		}

		if !belongsToCronJob {
			continue
		}

		status := "Running"
		if job.Spec.Suspend != nil && *job.Spec.Suspend {
			status = "Suspended"
		} else if job.Status.CompletionTime != nil {
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
			status = "Failed"
		}

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
				d := time.Since(job.Status.StartTime.Time)
				duration = formatDuration(d)
			}
		}

		completions := int32(1)
		if job.Spec.Completions != nil {
			completions = *job.Spec.Completions
		}

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

	sort.Slice(history, func(i, j int) bool {
		return history[i].CreationTimestamp > history[j].CreationTimestamp
	})

	return &types.CronJobHistoryResponse{
		Jobs: history,
	}, nil
}

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

func (c *cronJobOperator) TriggerOnce(namespace, name string) error {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	jobName := fmt.Sprintf("%s-manual-%d", cronJob.Name, time.Now().Unix())

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels:    cronJob.Spec.JobTemplate.Labels,
			Annotations: map[string]string{
				"cronjob.kubernetes.io/instantiate": "manual",
				"manual-trigger-time":               time.Now().Format(time.RFC3339),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "batch/v1",
					Kind:       "CronJob",
					Name:       cronJob.Name,
					UID:        cronJob.UID,
					Controller: func() *bool { b := false; return &b }(),
				},
			},
		},
		Spec: cronJob.Spec.JobTemplate.Spec,
	}

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

	podLabels := make(map[string]string)
	for k, v := range cronJob.Spec.JobTemplate.Spec.Template.Labels {
		podLabels[k] = v
	}
	return podLabels, nil
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

	selectorLabels := make(map[string]string)
	for k, v := range cronJob.Spec.JobTemplate.Spec.Selector.MatchLabels {
		selectorLabels[k] = v
	}
	return selectorLabels, nil
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

	if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
		status.Status = types.StatusStopped
		status.Message = "CronJob 已挂起"
		status.Ready = true
		return status, nil
	}

	if len(cronJob.Status.Active) > 0 {
		status.Status = types.StatusRunning
		status.Message = fmt.Sprintf("有 %d 个活跃 Job 正在运行", len(cronJob.Status.Active))
		return status, nil
	}

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

// GetSchedulingConfig 获取 CronJob 调度配置
// 使用 types 包中的公共转换函数
func (c *cronJobOperator) GetSchedulingConfig(namespace, name string) (*types.SchedulingConfig, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, fmt.Errorf("获取 CronJob 失败: %w", err)
	}

	return types.ConvertPodSpecToSchedulingConfig(&cronJob.Spec.JobTemplate.Spec.Template.Spec), nil
}

// UpdateSchedulingConfig 更新 CronJob 调度配置
// 使用 types 包中的公共转换函数
func (c *cronJobOperator) UpdateSchedulingConfig(namespace, name string, req *types.UpdateSchedulingConfigRequest) error {
	if req == nil {
		return fmt.Errorf("调度配置请求不能为空")
	}

	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return fmt.Errorf("获取 CronJob 失败: %w", err)
	}

	types.ApplySchedulingConfigToPodSpec(&cronJob.Spec.JobTemplate.Spec.Template.Spec, req)

	_, err = c.Update(cronJob)
	if err != nil {
		return fmt.Errorf("更新 CronJob 调度配置失败: %w", err)
	}

	return nil
}

// ==================== 存储配置管理 ====================

// GetStorageConfig 获取存储配置
// 使用公共转换函数 ConvertPodSpecToStorageConfig
func (c *cronJobOperator) GetStorageConfig(namespace, name string) (*types.StorageConfig, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	podSpec := &cronJob.Spec.JobTemplate.Spec.Template.Spec
	return ConvertPodSpecToStorageConfig(podSpec), nil
}

// UpdateStorageConfig 更新存储配置
// 使用公共转换函数 ApplyStorageConfigToPodSpec
func (c *cronJobOperator) UpdateStorageConfig(namespace, name string, config *types.UpdateStorageConfigRequest) error {
	if config == nil {
		return fmt.Errorf("存储配置不能为空")
	}

	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	podSpec := &cronJob.Spec.JobTemplate.Spec.Template.Spec

	if config.VolumeClaimTemplates != nil && len(config.VolumeClaimTemplates) > 0 {
		return fmt.Errorf("CronJob 不支持 VolumeClaimTemplates")
	}

	if err := ApplyStorageConfigToPodSpec(podSpec, config); err != nil {
		return fmt.Errorf("应用存储配置更新失败: %v", err)
	}

	_, err = c.Update(cronJob)
	return err
}

// ==================== Events 相关 ====================

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

	buf.WriteString(fmt.Sprintf("Name:                          %s\n", cronJob.Name))
	buf.WriteString(fmt.Sprintf("Namespace:                     %s\n", cronJob.Namespace))

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

	if cronJob.Spec.TimeZone != nil && *cronJob.Spec.TimeZone != "" {
		buf.WriteString(fmt.Sprintf("Time Zone:                     %s\n", *cronJob.Spec.TimeZone))
	}

	buf.WriteString(fmt.Sprintf("Concurrency Policy:            %s\n", cronJob.Spec.ConcurrencyPolicy))

	suspend := false
	if cronJob.Spec.Suspend != nil {
		suspend = *cronJob.Spec.Suspend
	}
	buf.WriteString(fmt.Sprintf("Suspend:                       %v\n", suspend))

	if cronJob.Spec.SuccessfulJobsHistoryLimit != nil {
		buf.WriteString(fmt.Sprintf("Successful Job History Limit:  %d\n", *cronJob.Spec.SuccessfulJobsHistoryLimit))
	} else {
		buf.WriteString("Successful Job History Limit:  3 (default)\n")
	}

	if cronJob.Spec.FailedJobsHistoryLimit != nil {
		buf.WriteString(fmt.Sprintf("Failed Job History Limit:      %d\n", *cronJob.Spec.FailedJobsHistoryLimit))
	} else {
		buf.WriteString("Failed Job History Limit:      1 (default)\n")
	}

	if cronJob.Spec.StartingDeadlineSeconds != nil {
		buf.WriteString(fmt.Sprintf("Starting Deadline Seconds:     %d\n", *cronJob.Spec.StartingDeadlineSeconds))
	}

	if cronJob.Status.LastScheduleTime != nil {
		buf.WriteString(fmt.Sprintf("Last Schedule Time:            %s\n", cronJob.Status.LastScheduleTime.Format(time.RFC1123)))
	} else {
		buf.WriteString("Last Schedule Time:            <none>\n")
	}

	if cronJob.Status.LastSuccessfulTime != nil {
		buf.WriteString(fmt.Sprintf("Last Successful Time:          %s\n", cronJob.Status.LastSuccessfulTime.Format(time.RFC1123)))
	} else {
		buf.WriteString("Last Successful Time:          <none>\n")
	}

	buf.WriteString(fmt.Sprintf("Active Jobs:                   %d\n", len(cronJob.Status.Active)))

	if len(cronJob.Status.Active) > 0 {
		buf.WriteString("Active Job References:\n")
		for _, ref := range cronJob.Status.Active {
			buf.WriteString(fmt.Sprintf("  - %s\n", ref.Name))
		}
	}

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

	if jobSpec.BackoffLimit != nil {
		buf.WriteString(fmt.Sprintf("    Backoff Limit:  %d\n", *jobSpec.BackoffLimit))
	} else {
		buf.WriteString("    Backoff Limit:  6 (default)\n")
	}

	if jobSpec.ActiveDeadlineSeconds != nil {
		buf.WriteString(fmt.Sprintf("    Active Deadline Seconds:  %d\n", *jobSpec.ActiveDeadlineSeconds))
	}

	if jobSpec.CompletionMode != nil {
		buf.WriteString(fmt.Sprintf("    Completion Mode:  %s\n", *jobSpec.CompletionMode))
	}

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

	if jobSpec.Template.Spec.ServiceAccountName != "" {
		buf.WriteString(fmt.Sprintf("      Service Account:  %s\n", jobSpec.Template.Spec.ServiceAccountName))
	} else {
		buf.WriteString("      Service Account:  default\n")
	}

	if jobSpec.Template.Spec.RestartPolicy != "" {
		buf.WriteString(fmt.Sprintf("      Restart Policy:   %s\n", jobSpec.Template.Spec.RestartPolicy))
	}

	if len(jobSpec.Template.Spec.InitContainers) > 0 {
		buf.WriteString("      Init Containers:\n")
		for _, container := range jobSpec.Template.Spec.InitContainers {
			c.writeContainerDescribe(&buf, container, "       ")
		}
	}

	buf.WriteString("      Containers:\n")
	for _, container := range jobSpec.Template.Spec.Containers {
		c.writeContainerDescribe(&buf, container, "       ")
	}

	buf.WriteString("      Volumes:\n")
	if len(jobSpec.Template.Spec.Volumes) > 0 {
		for _, vol := range jobSpec.Template.Spec.Volumes {
			c.writeVolumeDescribe(&buf, vol, "       ")
		}
	} else {
		buf.WriteString("       <none>\n")
	}

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

func (c *cronJobOperator) writeContainerDescribe(buf *strings.Builder, container corev1.Container, indent string) {
	buf.WriteString(fmt.Sprintf("%s%s:\n", indent, container.Name))
	buf.WriteString(fmt.Sprintf("%s Image:      %s\n", indent, container.Image))

	if container.ImagePullPolicy != "" {
		buf.WriteString(fmt.Sprintf("%s Image Pull Policy:  %s\n", indent, container.ImagePullPolicy))
	}

	if len(container.Ports) > 0 {
		for _, port := range container.Ports {
			buf.WriteString(fmt.Sprintf("%s Port:       %d/%s\n", indent, port.ContainerPort, port.Protocol))
		}
	}

	if len(container.Command) > 0 {
		buf.WriteString(fmt.Sprintf("%s Command:\n", indent))
		for _, cmd := range container.Command {
			buf.WriteString(fmt.Sprintf("%s   %s\n", indent, cmd))
		}
	}

	if len(container.Args) > 0 {
		buf.WriteString(fmt.Sprintf("%s Args:\n", indent))
		for _, arg := range container.Args {
			buf.WriteString(fmt.Sprintf("%s   %s\n", indent, arg))
		}
	}

	if len(container.Resources.Limits) > 0 {
		buf.WriteString(fmt.Sprintf("%s Limits:\n", indent))
		if cpu := container.Resources.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
			buf.WriteString(fmt.Sprintf("%s   cpu:     %s\n", indent, cpu.String()))
		}
		if mem := container.Resources.Limits.Memory(); mem != nil && !mem.IsZero() {
			buf.WriteString(fmt.Sprintf("%s   memory:  %s\n", indent, mem.String()))
		}
	}

	if len(container.Resources.Requests) > 0 {
		buf.WriteString(fmt.Sprintf("%s Requests:\n", indent))
		if cpu := container.Resources.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
			buf.WriteString(fmt.Sprintf("%s   cpu:     %s\n", indent, cpu.String()))
		}
		if mem := container.Resources.Requests.Memory(); mem != nil && !mem.IsZero() {
			buf.WriteString(fmt.Sprintf("%s   memory:  %s\n", indent, mem.String()))
		}
	}

	if container.LivenessProbe != nil {
		buf.WriteString(fmt.Sprintf("%s Liveness:   %s\n", indent, c.formatProbeForDescribe(container.LivenessProbe)))
	}
	if container.ReadinessProbe != nil {
		buf.WriteString(fmt.Sprintf("%s Readiness:  %s\n", indent, c.formatProbeForDescribe(container.ReadinessProbe)))
	}
	if container.StartupProbe != nil {
		buf.WriteString(fmt.Sprintf("%s Startup:    %s\n", indent, c.formatProbeForDescribe(container.StartupProbe)))
	}

	if len(container.Env) > 0 {
		buf.WriteString(fmt.Sprintf("%s Environment:\n", indent))
		for _, env := range container.Env {
			c.formatEnvironment(buf, env, indent+"   ")
		}
	}

	if len(container.VolumeMounts) > 0 {
		buf.WriteString(fmt.Sprintf("%s Mounts:\n", indent))
		for _, mount := range container.VolumeMounts {
			ro := "rw"
			if mount.ReadOnly {
				ro = "ro"
			}
			buf.WriteString(fmt.Sprintf("%s   %s from %s (%s)\n", indent, mount.MountPath, mount.Name, ro))
		}
	}
}

func (c *cronJobOperator) writeVolumeDescribe(buf *strings.Builder, vol corev1.Volume, indent string) {
	buf.WriteString(fmt.Sprintf("%s%s:\n", indent, vol.Name))
	if vol.ConfigMap != nil {
		buf.WriteString(fmt.Sprintf("%s Type:       ConfigMap (a volume populated by a ConfigMap)\n", indent))
		buf.WriteString(fmt.Sprintf("%s Name:       %s\n", indent, vol.ConfigMap.Name))
		if vol.ConfigMap.Optional != nil {
			buf.WriteString(fmt.Sprintf("%s Optional:   %v\n", indent, *vol.ConfigMap.Optional))
		}
	} else if vol.Secret != nil {
		buf.WriteString(fmt.Sprintf("%s Type:       Secret (a volume populated by a Secret)\n", indent))
		buf.WriteString(fmt.Sprintf("%s SecretName: %s\n", indent, vol.Secret.SecretName))
		if vol.Secret.Optional != nil {
			buf.WriteString(fmt.Sprintf("%s Optional:   %v\n", indent, *vol.Secret.Optional))
		}
	} else if vol.EmptyDir != nil {
		buf.WriteString(fmt.Sprintf("%s Type:       EmptyDir (a temporary directory that shares a pod's lifetime)\n", indent))
		if vol.EmptyDir.Medium != "" {
			buf.WriteString(fmt.Sprintf("%s Medium:     %s\n", indent, vol.EmptyDir.Medium))
		}
		if vol.EmptyDir.SizeLimit != nil && !vol.EmptyDir.SizeLimit.IsZero() {
			buf.WriteString(fmt.Sprintf("%s SizeLimit:  %s\n", indent, vol.EmptyDir.SizeLimit.String()))
		}
	} else if vol.HostPath != nil {
		buf.WriteString(fmt.Sprintf("%s Type:       HostPath (bare host directory volume)\n", indent))
		buf.WriteString(fmt.Sprintf("%s Path:       %s\n", indent, vol.HostPath.Path))
		if vol.HostPath.Type != nil {
			buf.WriteString(fmt.Sprintf("%s HostPathType: %s\n", indent, *vol.HostPath.Type))
		}
	} else if vol.PersistentVolumeClaim != nil {
		buf.WriteString(fmt.Sprintf("%s Type:       PersistentVolumeClaim (a reference to a PersistentVolumeClaim in the same namespace)\n", indent))
		buf.WriteString(fmt.Sprintf("%s ClaimName:  %s\n", indent, vol.PersistentVolumeClaim.ClaimName))
		buf.WriteString(fmt.Sprintf("%s ReadOnly:   %v\n", indent, vol.PersistentVolumeClaim.ReadOnly))
	} else if vol.Projected != nil {
		buf.WriteString(fmt.Sprintf("%s Type:       Projected (a volume that contains injected data from multiple sources)\n", indent))
	}
}

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

func (c *cronJobOperator) GetJobSpec(namespace, name string) (*types.JobSpecConfig, error) {
	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	jobSpec := cronJob.Spec.JobTemplate.Spec

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

	if jobSpec.PodFailurePolicy != nil {
		config.PodFailurePolicy = c.convertPodFailurePolicyFromK8s(jobSpec.PodFailurePolicy)
	}

	return config, nil
}

func (c *cronJobOperator) UpdateJobSpec(req *types.UpdateJobSpecRequest) error {
	if req == nil || req.Namespace == "" || req.Name == "" {
		return fmt.Errorf("请求参数不完整")
	}

	cronJob, err := c.Get(req.Namespace, req.Name)
	if err != nil {
		return err
	}

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

	if req.CompletionMode != nil {
		mode := batchv1.CompletionMode(*req.CompletionMode)
		cronJob.Spec.JobTemplate.Spec.CompletionMode = &mode
	}

	if req.Suspend != nil {
		cronJob.Spec.JobTemplate.Spec.Suspend = req.Suspend
	}

	if req.PodReplacementPolicy != nil {
		policy := batchv1.PodReplacementPolicy(*req.PodReplacementPolicy)
		cronJob.Spec.JobTemplate.Spec.PodReplacementPolicy = &policy
	}

	if req.BackoffLimitPerIndex != nil {
		cronJob.Spec.JobTemplate.Spec.BackoffLimitPerIndex = req.BackoffLimitPerIndex
	}

	if req.MaxFailedIndexes != nil {
		cronJob.Spec.JobTemplate.Spec.MaxFailedIndexes = req.MaxFailedIndexes
	}

	if req.PodFailurePolicy != nil {
		cronJob.Spec.JobTemplate.Spec.PodFailurePolicy = c.convertPodFailurePolicyToK8s(req.PodFailurePolicy)
	}

	_, err = c.Update(cronJob)
	return err
}

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

	if cronJob.Spec.TimeZone != nil {
		response.Timezone = *cronJob.Spec.TimeZone
	}

	if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
		response.IsSuspended = true
		return response, nil
	}

	schedule, err := cron.ParseStandard(cronJob.Spec.Schedule)
	if err != nil {
		return nil, fmt.Errorf("解析Cron表达式失败: %v", err)
	}

	now := time.Now()

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

func (c *cronJobOperator) ListAll(namespace string) ([]batchv1.CronJob, error) {
	var cronJobs []*batchv1.CronJob
	var err error

	if c.useInformer && c.cronJobLister != nil {
		if namespace == "" {
			cronJobs, err = c.cronJobLister.List(k8slabels.Everything())
		} else {
			cronJobs, err = c.cronJobLister.CronJobs(namespace).List(k8slabels.Everything())
		}

		if err != nil {
			return c.listAllFromAPI(namespace)
		}
	} else {
		return c.listAllFromAPI(namespace)
	}

	result := make([]batchv1.CronJob, 0, len(cronJobs))
	for _, cronJob := range cronJobs {
		if cronJob != nil {
			result = append(result, *cronJob)
		}
	}

	return result, nil
}

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

	cronJob, err := c.Get(namespace, name)
	if err != nil {
		return nil, err
	}

	// 使用 Pod 模板标签，而不是 selector 标签
	// Pod 模板标签包含了 Pod 实际拥有的完整标签集合
	podLabels := cronJob.Spec.JobTemplate.Spec.Template.Labels
	if len(podLabels) == 0 {
		return nil, fmt.Errorf("CronJob 没有 Pod 模板标签")
	}

	return getWorkloadResourceSummary(
		namespace,
		podLabels, // 使用 Pod 模板标签
		domainSuffix,
		nodeLb,
		podOp,
		svcOp,
		ingressOp,
	)
}
