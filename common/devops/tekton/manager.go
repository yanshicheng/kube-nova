package tekton

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

const (
	AnnotationStepHash   = "kube-nova.io/step-hash"
	AnnotationSecretHash = "kube-nova.io/secret-hash"

	tektonV1      = "tekton.dev/v1"
	tektonV1Beta1 = "tekton.dev/v1beta1"
)

var taskGVRs = []schema.GroupVersionResource{
	{Group: "tekton.dev", Version: "v1", Resource: "tasks"},
	{Group: "tekton.dev", Version: "v1beta1", Resource: "tasks"},
}

var tektonDurationPattern = regexp.MustCompile(`^([0-9]+(ms|h|m|s))+$`)
var tektonDNSLabelPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

var tektonTaskVolumeSourceKeys = map[string]struct{}{
	"hostPath":              {},
	"emptyDir":              {},
	"gcePersistentDisk":     {},
	"awsElasticBlockStore":  {},
	"gitRepo":               {},
	"secret":                {},
	"nfs":                   {},
	"iscsi":                 {},
	"glusterfs":             {},
	"persistentVolumeClaim": {},
	"rbd":                   {},
	"flexVolume":            {},
	"cinder":                {},
	"cephfs":                {},
	"flocker":               {},
	"downwardAPI":           {},
	"fc":                    {},
	"azureFile":             {},
	"configMap":             {},
	"vsphereVolume":         {},
	"quobyte":               {},
	"azureDisk":             {},
	"photonPersistentDisk":  {},
	"projected":             {},
	"portworxVolume":        {},
	"scaleIO":               {},
	"storageos":             {},
	"csi":                   {},
	"ephemeral":             {},
	"image":                 {},
}

var pipelineGVRs = []schema.GroupVersionResource{
	{Group: "tekton.dev", Version: "v1", Resource: "pipelines"},
	{Group: "tekton.dev", Version: "v1beta1", Resource: "pipelines"},
}

var pipelineRunGVRs = []schema.GroupVersionResource{
	{Group: "tekton.dev", Version: "v1", Resource: "pipelineruns"},
	{Group: "tekton.dev", Version: "v1beta1", Resource: "pipelineruns"},
}

var taskRunGVRs = []schema.GroupVersionResource{
	{Group: "tekton.dev", Version: "v1", Resource: "taskruns"},
	{Group: "tekton.dev", Version: "v1beta1", Resource: "taskruns"},
}

var clusterTaskGVRs = []schema.GroupVersionResource{
	{Group: "tekton.dev", Version: "v1", Resource: "clustertasks"},
	{Group: "tekton.dev", Version: "v1beta1", Resource: "clustertasks"},
}

type Manager struct{}

type Client struct {
	dynamicClient dynamic.Interface
	kubeClient    kubernetes.Interface
	discovery     discovery.DiscoveryInterface
}

type ChannelConfig struct {
	TaskNamespace string `json:"taskNamespace"`
}

type StepResource struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
	Hash       string
	Object     *unstructured.Unstructured
}

type SecretInfo struct {
	Name              string
	Type              string
	Namespace         string
	Keys              []string
	ManagedByKubeNova bool
	CreatedAt         int64
}

type PVCInfo struct {
	Name             string
	Namespace        string
	StorageClassName string
	Storage          string
	AccessModes      []string
	VolumeMode       string
	Status           string
	CreatedAt        int64
}

type ConfigMapInfo struct {
	Name      string
	Namespace string
	Keys      []string
	CreatedAt int64
}

type PodInfo struct {
	Name           string
	Namespace      string
	Phase          string
	Reason         string
	Message        string
	NodeName       string
	ContainerNames []string
	CreatedAt      int64
	Yaml           string
}

type PipelineInfo struct {
	Name        string
	Namespace   string
	Description string
	Params      []string
	Workspaces  []string
	Results     []string
	CreatedAt   int64
	Yaml        string
}

type PipelineRunInfo struct {
	Name      string
	Namespace string
	Status    string
	Reason    string
	Message   string
	CreatedAt int64
	Yaml      string
}

type RunStatus struct {
	Status       string
	Reason       string
	Message      string
	StartedAt    time.Time
	FinishedAt   time.Time
	RawSucceeded string
	SkippedTasks []SkippedTaskInfo
}

type SkippedTaskInfo struct {
	Name    string
	Reason  string
	Message string
}

type TaskRunInfo struct {
	Name             string
	PipelineTaskName string
	PodName          string
	ContainerName    string
	ContainerNames   []string
	Status           RunStatus
}

type TaskInfo struct {
	Name        string
	Namespace   string
	Kind        string
	APIVersion  string
	Description string
	Params      []string
	Workspaces  []string
	Results     []string
	CreatedAt   int64
	Yaml        string
}

type ServiceAccountInfo struct {
	Name             string
	Namespace        string
	Secrets          []string
	ImagePullSecrets []string
	CreatedAt        int64
}

type StorageClassInfo struct {
	Name                 string
	Provisioner          string
	ReclaimPolicy        string
	VolumeBindingMode    string
	AllowVolumeExpansion bool
	CreatedAt            int64
}

func New() devopstypes.Provider {
	return Manager{}
}

func (Manager) TestConnection(ctx context.Context, req devopstypes.Request) devopstypes.Result {
	metadata := devopstypes.Metadata{
		ProductName:  "Tekton",
		APIVersion:   tektonV1,
		Capabilities: map[string]any{},
	}
	client, err := NewClient(req)
	if err != nil {
		logx.Errorf("Tekton 连通性检测失败: %v", err)
		return devopstypes.Unhealthy(devopstypes.TrimMessage(err.Error()), metadata)
	}
	if err := client.CheckTektonAPI(ctx); err != nil {
		logx.Errorf("Tekton API 检测失败: %v", err)
		return devopstypes.Unhealthy(devopstypes.TrimMessage(err.Error()), metadata)
	}
	if err := client.CheckClusterResolver(ctx); err != nil {
		logx.Errorf("Tekton cluster resolver 检测失败: %v", err)
		return devopstypes.Unhealthy(devopstypes.TrimMessage(err.Error()), metadata)
	}
	cfg, err := ParseChannelConfig(req.Channel.Config)
	if err != nil {
		logx.Errorf("Tekton 渠道配置检测失败: %v", err)
		return devopstypes.Unhealthy(devopstypes.TrimMessage(err.Error()), metadata)
	}
	if err := client.NamespaceExists(ctx, cfg.TaskNamespace); err != nil {
		logx.Errorf("Tekton Task 命名空间检测失败: %v", err)
		return devopstypes.Unhealthy(devopstypes.TrimMessage(err.Error()), metadata)
	}
	metadata.Capabilities["task"] = true
	metadata.Capabilities["clusterResolver"] = true
	metadata.Capabilities["taskNamespace"] = cfg.TaskNamespace
	return devopstypes.Healthy("Tekton API 可用", metadata)
}

func ParseChannelConfig(content string) (ChannelConfig, error) {
	var cfg ChannelConfig
	content = strings.TrimSpace(content)
	if content == "" {
		return cfg, fmt.Errorf("Tekton taskNamespace 不能为空")
	}
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		return cfg, fmt.Errorf("Tekton 渠道配置必须是结构化 JSON")
	}
	cfg.TaskNamespace = strings.TrimSpace(cfg.TaskNamespace)
	if cfg.TaskNamespace == "" {
		return cfg, fmt.Errorf("Tekton taskNamespace 不能为空")
	}
	return cfg, nil
}

func NewClient(req devopstypes.Request) (*Client, error) {
	config, err := restConfigFromRequest(req)
	if err != nil {
		return nil, err
	}
	config.Timeout = 10 * time.Second
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Client{dynamicClient: dynamicClient, kubeClient: kubeClient, discovery: discoveryClient}, nil
}

func (c *Client) CheckTektonAPI(ctx context.Context) error {
	for _, groupVersion := range []string{tektonV1, tektonV1Beta1} {
		list, err := c.discovery.ServerResourcesForGroupVersion(groupVersion)
		if err != nil {
			if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
				continue
			}
			return err
		}
		for _, item := range list.APIResources {
			if item.Kind == "Task" || item.Name == "tasks" {
				return nil
			}
		}
	}
	return fmt.Errorf("集群未安装 Tekton Pipelines 或缺少 Task API")
}

func (c *Client) CheckClusterResolver(ctx context.Context) error {
	hasResolutionRequest := false
	for _, groupVersion := range []string{"resolution.tekton.dev/v1beta1", "resolution.tekton.dev/v1alpha1"} {
		list, err := c.discovery.ServerResourcesForGroupVersion(groupVersion)
		if err != nil {
			if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
				continue
			}
			return err
		}
		for _, item := range list.APIResources {
			if item.Kind == "ResolutionRequest" || item.Name == "resolutionrequests" {
				hasResolutionRequest = true
				break
			}
		}
	}
	if !hasResolutionRequest {
		return fmt.Errorf("集群未安装 Tekton resolvers 或缺少 ResolutionRequest API")
	}
	return c.checkClusterResolverFeatureFlag(ctx)
}

func (c *Client) checkClusterResolverFeatureFlag(ctx context.Context) error {
	for _, namespace := range []string{"tekton-pipelines-resolvers", "tekton-pipelines"} {
		cm, err := c.kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, "resolvers-feature-flags", metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			logx.Errorf("读取 Tekton resolver 特性配置失败: namespace=%s err=%v", namespace, err)
			return err
		}
		value := strings.ToLower(strings.TrimSpace(cm.Data["enable-cluster-resolver"]))
		if value == "false" {
			return fmt.Errorf("Tekton cluster resolver 未启用")
		}
		return nil
	}
	return nil
}

func (c *Client) ApplyStepResource(ctx context.Context, namespace, content string) (StepResource, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return StepResource{}, fmt.Errorf("Tekton taskNamespace 不能为空")
	}
	resource, err := ParseStepResource(content)
	if err != nil {
		return StepResource{}, err
	}
	resource.Namespace = namespace
	resource.Object.SetNamespace(namespace)
	annotations := resource.Object.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[AnnotationStepHash] = resource.Hash
	resource.Object.SetAnnotations(annotations)

	gvrs := gvrCandidates(resource.APIVersion)
	var lastErr error
	for _, gvr := range gvrs {
		existing, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, resource.Name, metav1.GetOptions{})
		if err == nil {
			resource.Object.SetResourceVersion(existing.GetResourceVersion())
			updated, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Update(ctx, resource.Object, metav1.UpdateOptions{})
			if err == nil {
				resource.Object = updated
				return resource, nil
			}
			lastErr = err
			continue
		}
		if !apierrors.IsNotFound(err) {
			lastErr = err
			continue
		}
		created, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, resource.Object, metav1.CreateOptions{})
		if err == nil {
			resource.Object = created
			return resource, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		logx.Errorf("同步 Tekton Task 失败: namespace=%s name=%s err=%v", namespace, resource.Name, lastErr)
		return StepResource{}, lastErr
	}
	return StepResource{}, fmt.Errorf("未找到可用的 Tekton Task API")
}

func (c *Client) ApplyPipeline(ctx context.Context, namespace, content string) (StepResource, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return StepResource{}, fmt.Errorf("Tekton namespace 不能为空")
	}
	resource, err := parseResource(content)
	if err != nil {
		return StepResource{}, err
	}
	if resource.Kind != "Pipeline" {
		return StepResource{}, fmt.Errorf("Tekton 流水线资源必须是 Pipeline")
	}
	resource.Namespace = namespace
	resource.Object.SetNamespace(namespace)
	return c.applyNamespacedResource(ctx, namespace, resource, gvrCandidatesByKind(resource.Kind, resource.APIVersion))
}

func (c *Client) DeletePipeline(ctx context.Context, namespace, name string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return fmt.Errorf("Tekton Pipeline 参数不完整")
	}
	var lastErr error
	for _, gvr := range pipelineGVRs {
		err := c.dynamicClient.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err == nil || apierrors.IsNotFound(err) {
			return nil
		}
		if !meta.IsNoMatchError(err) {
			lastErr = err
		}
	}
	if lastErr != nil {
		logx.Errorf("删除 Tekton Pipeline 失败: namespace=%s name=%s err=%v", namespace, name, lastErr)
		return lastErr
	}
	return fmt.Errorf("未找到可用的 Tekton Pipeline API")
}

func (c *Client) CreatePipelineRun(ctx context.Context, namespace, content string) (StepResource, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return StepResource{}, fmt.Errorf("Tekton namespace 不能为空")
	}
	resource, err := parseResource(content)
	if err != nil {
		return StepResource{}, err
	}
	if resource.Kind != "PipelineRun" {
		return StepResource{}, fmt.Errorf("Tekton 运行资源必须是 PipelineRun")
	}
	resource.Namespace = namespace
	resource.Object.SetNamespace(namespace)
	var lastErr error
	for _, gvr := range gvrCandidatesByKind(resource.Kind, resource.APIVersion) {
		created, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, resource.Object, metav1.CreateOptions{})
		if err == nil {
			resource.Object = created
			return resource, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		logx.Errorf("创建 Tekton PipelineRun 失败: namespace=%s name=%s err=%v", namespace, resource.Name, lastErr)
		return StepResource{}, lastErr
	}
	return StepResource{}, fmt.Errorf("未找到可用的 Tekton PipelineRun API")
}

func (c *Client) GetPipelineRunStatus(ctx context.Context, namespace, name string) (RunStatus, error) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return RunStatus{}, fmt.Errorf("Tekton PipelineRun 参数不完整")
	}
	var lastErr error
	for _, gvr := range pipelineRunGVRs {
		obj, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			return statusFromObject(obj), nil
		}
		if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			lastErr = err
		}
	}
	if lastErr != nil {
		logx.Errorf("读取 Tekton PipelineRun 状态失败: namespace=%s name=%s err=%v", namespace, name, lastErr)
		return RunStatus{}, lastErr
	}
	return RunStatus{}, fmt.Errorf("Tekton PipelineRun 不存在")
}

func (c *Client) ListTaskRuns(ctx context.Context, namespace, pipelineRunName string) ([]TaskRunInfo, error) {
	namespace = strings.TrimSpace(namespace)
	pipelineRunName = strings.TrimSpace(pipelineRunName)
	if namespace == "" || pipelineRunName == "" {
		return nil, fmt.Errorf("Tekton TaskRun 查询参数不完整")
	}
	var lastErr error
	for _, gvr := range taskRunGVRs {
		list, err := c.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "tekton.dev/pipelineRun=" + pipelineRunName,
		})
		if err != nil {
			if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
				continue
			}
			lastErr = err
			continue
		}
		items := make([]TaskRunInfo, 0, len(list.Items))
		for index := range list.Items {
			obj := &list.Items[index]
			labels := obj.GetLabels()
			podName, _, _ := unstructured.NestedString(obj.Object, "status", "podName")
			containerNames := c.taskRunContainerNames(ctx, namespace, strings.TrimSpace(podName), obj)
			items = append(items, TaskRunInfo{
				Name:             obj.GetName(),
				PipelineTaskName: strings.TrimSpace(labels["tekton.dev/pipelineTask"]),
				PodName:          strings.TrimSpace(podName),
				ContainerName:    taskRunContainerName(obj),
				ContainerNames:   containerNames,
				Status:           statusFromObject(obj),
			})
		}
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].Name < items[j].Name
		})
		return items, nil
	}
	if lastErr != nil {
		logx.Errorf("读取 Tekton TaskRun 列表失败: namespace=%s pipelineRun=%s err=%v", namespace, pipelineRunName, lastErr)
		return nil, lastErr
	}
	return nil, fmt.Errorf("未找到可用的 Tekton TaskRun API")
}

func (c *Client) ListTasks(ctx context.Context, namespace, keyword string) ([]TaskInfo, error) {
	namespace = strings.TrimSpace(namespace)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if namespace == "" {
		return nil, fmt.Errorf("Tekton Task namespace 不能为空")
	}
	var lastErr error
	for _, gvr := range taskGVRs {
		list, err := c.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
				continue
			}
			lastErr = err
			continue
		}
		return taskInfoListFromObjects(list.Items, keyword), nil
	}
	if lastErr != nil {
		logx.Errorf("查询 Tekton Task 列表失败: namespace=%s err=%v", namespace, lastErr)
		return nil, lastErr
	}
	return nil, fmt.Errorf("未找到可用的 Tekton Task API")
}

func (c *Client) ListClusterTasks(ctx context.Context, keyword string) ([]TaskInfo, error) {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	var lastErr error
	for _, gvr := range clusterTaskGVRs {
		list, err := c.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
				continue
			}
			lastErr = err
			continue
		}
		return taskInfoListFromObjects(list.Items, keyword), nil
	}
	if lastErr != nil {
		logx.Errorf("查询 Tekton ClusterTask 列表失败: err=%v", lastErr)
		return nil, lastErr
	}
	return nil, fmt.Errorf("未找到可用的 Tekton ClusterTask API")
}

func (c *Client) TaskExists(ctx context.Context, namespace, name string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return fmt.Errorf("Tekton Task 参数不完整")
	}
	return c.namedResourceExists(ctx, taskGVRs, namespace, name, "Task")
}

func (c *Client) GetTask(ctx context.Context, namespace, name string) (TaskInfo, error) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return TaskInfo{}, fmt.Errorf("Tekton Task 参数不完整")
	}
	return c.getTaskInfo(ctx, taskGVRs, namespace, name, "Task")
}

func (c *Client) ClusterTaskExists(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("Tekton ClusterTask 参数不完整")
	}
	return c.namedResourceExists(ctx, clusterTaskGVRs, "", name, "ClusterTask")
}

func (c *Client) GetClusterTask(ctx context.Context, name string) (TaskInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return TaskInfo{}, fmt.Errorf("Tekton ClusterTask 参数不完整")
	}
	return c.getTaskInfo(ctx, clusterTaskGVRs, "", name, "ClusterTask")
}

func (c *Client) ListPods(ctx context.Context, namespace, keyword string) ([]PodInfo, error) {
	namespace = strings.TrimSpace(namespace)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if namespace == "" {
		return nil, fmt.Errorf("Kubernetes Pod namespace 不能为空")
	}
	list, err := c.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		logx.Errorf("查询 Kubernetes Pod 列表失败: namespace=%s err=%v", namespace, err)
		return nil, err
	}
	result := make([]PodInfo, 0, len(list.Items))
	for _, item := range list.Items {
		if keyword != "" && !strings.Contains(strings.ToLower(item.Name), keyword) {
			continue
		}
		containerNames := make([]string, 0, len(item.Spec.InitContainers)+len(item.Spec.Containers))
		for _, container := range item.Spec.InitContainers {
			containerNames = appendUniqueString(containerNames, container.Name)
		}
		for _, container := range item.Spec.Containers {
			containerNames = appendUniqueString(containerNames, container.Name)
		}
		phase := string(item.Status.Phase)
		reason := strings.TrimSpace(item.Status.Reason)
		message := strings.TrimSpace(item.Status.Message)
		nodeName := strings.TrimSpace(item.Spec.NodeName)
		result = append(result, PodInfo{
			Name:           item.Name,
			Namespace:      item.Namespace,
			Phase:          phase,
			Reason:         reason,
			Message:        message,
			NodeName:       nodeName,
			ContainerNames: containerNames,
			CreatedAt:      item.CreationTimestamp.Unix(),
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (c *Client) GetPodYaml(ctx context.Context, namespace, name string) (string, error) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return "", fmt.Errorf("Kubernetes Pod 参数不完整")
	}
	pod, err := c.kubeClient.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("Kubernetes Pod 不存在: %s", name)
		}
		logx.Errorf("读取 Kubernetes Pod 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return "", err
	}
	pod = pod.DeepCopy()
	pod.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}
	pod.ManagedFields = nil
	data, err := yaml.Marshal(pod)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *Client) GetPodLog(ctx context.Context, namespace, name, container string, tailLines int64) (string, []string, error) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	container = strings.TrimSpace(container)
	if namespace == "" || name == "" {
		return "", nil, fmt.Errorf("Kubernetes Pod 日志参数不完整")
	}
	pod, err := c.kubeClient.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil, fmt.Errorf("Kubernetes Pod 不存在: %s", name)
		}
		logx.Errorf("读取 Kubernetes Pod 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return "", nil, err
	}
	containers := podContainerNames(pod)
	if container == "" {
		container = defaultPodLogContainer(pod)
	}
	if container == "" {
		return "", containers, fmt.Errorf("Kubernetes Pod 容器不能为空")
	}
	options := &corev1.PodLogOptions{Container: container}
	if tailLines > 0 {
		options.TailLines = &tailLines
	}
	data, err := c.kubeClient.CoreV1().Pods(namespace).GetLogs(name, options).Do(ctx).Raw()
	if err != nil {
		logx.Errorf("读取 Kubernetes Pod 日志失败: namespace=%s name=%s container=%s err=%v", namespace, name, container, err)
		return "", containers, err
	}
	if len(data) == 0 {
		return "当前容器暂无日志\n", containers, nil
	}
	return string(data), containers, nil
}

func (c *Client) DescribePod(ctx context.Context, namespace, name string) (string, error) {
	pod, err := c.kubeClient.CoreV1().Pods(strings.TrimSpace(namespace)).Get(ctx, strings.TrimSpace(name), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("Kubernetes Pod 不存在: %s", name)
		}
		logx.Errorf("读取 Kubernetes Pod 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return "", err
	}
	lines := []string{
		fmt.Sprintf("Phase: %s", pod.Status.Phase),
		fmt.Sprintf("Node: %s", pod.Spec.NodeName),
		fmt.Sprintf("Pod IP: %s", pod.Status.PodIP),
		fmt.Sprintf("Containers: %s", strings.Join(podContainerNames(pod), ", ")),
	}
	return c.describeObject(ctx, pod, "Pod", "v1", lines), nil
}

func (c *Client) DeletePod(ctx context.Context, namespace, name string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return fmt.Errorf("Kubernetes Pod 参数不完整")
	}
	if err := c.kubeClient.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		logx.Errorf("删除 Kubernetes Pod 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	return nil
}

func (c *Client) ListPipelines(ctx context.Context, namespace, keyword string) ([]PipelineInfo, error) {
	namespace = strings.TrimSpace(namespace)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if namespace == "" {
		return nil, fmt.Errorf("Tekton Pipeline namespace 不能为空")
	}
	var lastErr error
	for _, gvr := range pipelineGVRs {
		list, err := c.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
				continue
			}
			lastErr = err
			continue
		}
		result := make([]PipelineInfo, 0, len(list.Items))
		for i := range list.Items {
			obj := &list.Items[i]
			name := strings.TrimSpace(obj.GetName())
			if keyword != "" && !strings.Contains(strings.ToLower(name), keyword) {
				continue
			}
			params, workspaces, results := tektonTaskSpecCollections(obj)
			result = append(result, PipelineInfo{
				Name:        name,
				Namespace:   strings.TrimSpace(obj.GetNamespace()),
				Description: tektonTaskDescription(obj),
				Params:      params,
				Workspaces:  workspaces,
				Results:     results,
				CreatedAt:   obj.GetCreationTimestamp().Unix(),
				Yaml:        tektonTaskYaml(obj),
			})
		}
		sort.SliceStable(result, func(i, j int) bool {
			return result[i].Name < result[j].Name
		})
		return result, nil
	}
	if lastErr != nil {
		logx.Errorf("查询 Tekton Pipeline 列表失败: namespace=%s err=%v", namespace, lastErr)
		return nil, lastErr
	}
	return nil, fmt.Errorf("未找到可用的 Tekton Pipeline API")
}

func (c *Client) GetPipelineYaml(ctx context.Context, namespace, name string) (string, error) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return "", fmt.Errorf("Tekton Pipeline 参数不完整")
	}
	obj, err := c.getDynamicResource(ctx, pipelineGVRs, namespace, name, "Tekton Pipeline")
	if err != nil {
		return "", err
	}
	return tektonTaskYaml(obj), nil
}

func (c *Client) DescribePipeline(ctx context.Context, namespace, name string) (string, error) {
	obj, err := c.getDynamicResource(ctx, pipelineGVRs, namespace, name, "Tekton Pipeline")
	if err != nil {
		return "", err
	}
	params, workspaces, results := tektonTaskSpecCollections(obj)
	lines := []string{
		fmt.Sprintf("Description: %s", tektonTaskDescription(obj)),
		fmt.Sprintf("Params: %s", strings.Join(params, ", ")),
		fmt.Sprintf("Workspaces: %s", strings.Join(workspaces, ", ")),
		fmt.Sprintf("Results: %s", strings.Join(results, ", ")),
	}
	return c.describeObject(ctx, obj, obj.GetKind(), obj.GetAPIVersion(), lines), nil
}

func (c *Client) ListPipelineRuns(ctx context.Context, namespace, keyword string) ([]PipelineRunInfo, error) {
	namespace = strings.TrimSpace(namespace)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if namespace == "" {
		return nil, fmt.Errorf("Tekton PipelineRun namespace 不能为空")
	}
	var lastErr error
	for _, gvr := range pipelineRunGVRs {
		list, err := c.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
				continue
			}
			lastErr = err
			continue
		}
		result := make([]PipelineRunInfo, 0, len(list.Items))
		for i := range list.Items {
			obj := &list.Items[i]
			name := strings.TrimSpace(obj.GetName())
			if keyword != "" && !strings.Contains(strings.ToLower(name), keyword) {
				continue
			}
			status := statusFromObject(obj)
			result = append(result, PipelineRunInfo{
				Name:      name,
				Namespace: strings.TrimSpace(obj.GetNamespace()),
				Status:    status.Status,
				Reason:    status.Reason,
				Message:   status.Message,
				CreatedAt: obj.GetCreationTimestamp().Unix(),
				Yaml:      tektonTaskYaml(obj),
			})
		}
		sort.SliceStable(result, func(i, j int) bool {
			return result[i].Name < result[j].Name
		})
		return result, nil
	}
	if lastErr != nil {
		logx.Errorf("查询 Tekton PipelineRun 列表失败: namespace=%s err=%v", namespace, lastErr)
		return nil, lastErr
	}
	return nil, fmt.Errorf("未找到可用的 Tekton PipelineRun API")
}

func (c *Client) GetPipelineRunYaml(ctx context.Context, namespace, name string) (string, error) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return "", fmt.Errorf("Tekton PipelineRun 参数不完整")
	}
	obj, err := c.getDynamicResource(ctx, pipelineRunGVRs, namespace, name, "Tekton PipelineRun")
	if err != nil {
		return "", err
	}
	return tektonTaskYaml(obj), nil
}

func (c *Client) DescribePipelineRun(ctx context.Context, namespace, name string) (string, error) {
	obj, err := c.getDynamicResource(ctx, pipelineRunGVRs, namespace, name, "Tekton PipelineRun")
	if err != nil {
		return "", err
	}
	status := statusFromObject(obj)
	taskRuns, _ := c.ListTaskRuns(ctx, namespace, name)
	taskRunLines := make([]string, 0, len(taskRuns))
	for _, item := range taskRuns {
		taskRunLines = append(taskRunLines, fmt.Sprintf("%s(%s/%s)", item.Name, item.PipelineTaskName, item.Status.Status))
	}
	lines := []string{
		fmt.Sprintf("Status: %s", status.Status),
		fmt.Sprintf("Reason: %s", status.Reason),
		fmt.Sprintf("Message: %s", status.Message),
		fmt.Sprintf("StartedAt: %s", status.StartedAt.Format(time.RFC3339)),
		fmt.Sprintf("FinishedAt: %s", status.FinishedAt.Format(time.RFC3339)),
		fmt.Sprintf("TaskRuns: %s", strings.Join(taskRunLines, ", ")),
	}
	return c.describeObject(ctx, obj, obj.GetKind(), obj.GetAPIVersion(), lines), nil
}

func (c *Client) GetPipelineRunLog(ctx context.Context, namespace, name, container string, tailLines int64) (string, []string, error) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	container = strings.TrimSpace(container)
	if namespace == "" || name == "" {
		return "", nil, fmt.Errorf("Tekton PipelineRun 日志参数不完整")
	}
	taskRuns, err := c.ListTaskRuns(ctx, namespace, name)
	if err != nil {
		return "", nil, err
	}
	var builder strings.Builder
	containers := make([]string, 0)
	for _, taskRun := range taskRuns {
		if taskRun.PodName == "" {
			continue
		}
		containers = appendUniqueStrings(containers, taskRun.ContainerNames...)
		if container != "" && !stringSliceContains(taskRun.ContainerNames, container) {
			continue
		}
		podContainer := container
		if podContainer == "" {
			podContainer = taskRun.ContainerName
		}
		content, _, err := c.GetPodLog(ctx, namespace, taskRun.PodName, podContainer, tailLines)
		builder.WriteString(fmt.Sprintf("===== TaskRun: %s Pod: %s Container: %s =====\n", taskRun.Name, taskRun.PodName, podContainer))
		if err != nil {
			builder.WriteString("读取日志失败: " + err.Error() + "\n")
			continue
		}
		builder.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			builder.WriteString("\n")
		}
	}
	if strings.TrimSpace(builder.String()) == "" {
		builder.WriteString("未找到可用的 TaskRun Pod 日志\n")
	}
	return builder.String(), containers, nil
}

func (c *Client) ListServiceAccounts(ctx context.Context, namespace, keyword string) ([]ServiceAccountInfo, error) {
	namespace = strings.TrimSpace(namespace)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if namespace == "" {
		return nil, fmt.Errorf("Kubernetes ServiceAccount namespace 不能为空")
	}
	list, err := c.kubeClient.CoreV1().ServiceAccounts(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		logx.Errorf("查询 Kubernetes ServiceAccount 列表失败: namespace=%s err=%v", namespace, err)
		return nil, err
	}
	result := make([]ServiceAccountInfo, 0, len(list.Items))
	for _, item := range list.Items {
		if keyword != "" && !strings.Contains(strings.ToLower(item.Name), keyword) {
			continue
		}
		secrets := make([]string, 0, len(item.Secrets))
		for _, secret := range item.Secrets {
			if strings.TrimSpace(secret.Name) != "" {
				secrets = append(secrets, secret.Name)
			}
		}
		imagePullSecrets := make([]string, 0, len(item.ImagePullSecrets))
		for _, secret := range item.ImagePullSecrets {
			if strings.TrimSpace(secret.Name) != "" {
				imagePullSecrets = append(imagePullSecrets, secret.Name)
			}
		}
		sort.Strings(secrets)
		sort.Strings(imagePullSecrets)
		result = append(result, ServiceAccountInfo{
			Name:             item.Name,
			Namespace:        item.Namespace,
			Secrets:          secrets,
			ImagePullSecrets: imagePullSecrets,
			CreatedAt:        item.CreationTimestamp.Unix(),
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (c *Client) ListStorageClasses(ctx context.Context, keyword string) ([]StorageClassInfo, error) {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	list, err := c.kubeClient.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		logx.Errorf("查询 Kubernetes StorageClass 列表失败: %v", err)
		return nil, err
	}
	result := make([]StorageClassInfo, 0, len(list.Items))
	for _, item := range list.Items {
		if keyword != "" && !strings.Contains(strings.ToLower(item.Name), keyword) {
			continue
		}
		allowVolumeExpansion := false
		if item.AllowVolumeExpansion != nil {
			allowVolumeExpansion = *item.AllowVolumeExpansion
		}
		reclaimPolicy := ""
		if item.ReclaimPolicy != nil {
			reclaimPolicy = string(*item.ReclaimPolicy)
		}
		volumeBindingMode := ""
		if item.VolumeBindingMode != nil {
			volumeBindingMode = string(*item.VolumeBindingMode)
		}
		result = append(result, StorageClassInfo{
			Name:                 item.Name,
			Provisioner:          item.Provisioner,
			ReclaimPolicy:        reclaimPolicy,
			VolumeBindingMode:    volumeBindingMode,
			AllowVolumeExpansion: allowVolumeExpansion,
			CreatedAt:            item.CreationTimestamp.Unix(),
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (c *Client) GetPersistentVolumeClaim(ctx context.Context, namespace, name string) (*corev1.PersistentVolumeClaim, error) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("Kubernetes PVC 参数不完整")
	}
	pvc, err := c.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("Kubernetes PVC 不存在: %s", name)
		}
		logx.Errorf("读取 Kubernetes PVC 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return nil, err
	}
	return pvc, nil
}

func (c *Client) DescribePersistentVolumeClaim(ctx context.Context, namespace, name string) (string, error) {
	pvc, err := c.GetPersistentVolumeClaim(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	storage := ""
	if pvc.Spec.Resources.Requests != nil {
		storage = pvc.Spec.Resources.Requests.Storage().String()
	}
	lines := []string{
		fmt.Sprintf("Status: %s", pvc.Status.Phase),
		fmt.Sprintf("StorageClass: %s", stringValue(pvc.Spec.StorageClassName)),
		fmt.Sprintf("Capacity: %s", pvc.Status.Capacity.Storage().String()),
		fmt.Sprintf("Request: %s", storage),
		fmt.Sprintf("AccessModes: %s", accessModesToString(pvc.Spec.AccessModes)),
	}
	return c.describeObject(ctx, pvc, "PersistentVolumeClaim", "v1", lines), nil
}

func (c *Client) ApplyPersistentVolumeClaim(ctx context.Context, namespace string, pvc *corev1.PersistentVolumeClaim) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" || pvc == nil || strings.TrimSpace(pvc.Name) == "" {
		return fmt.Errorf("Kubernetes PVC 参数不完整")
	}
	pvc.Namespace = namespace
	existing, err := c.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvc.Name, metav1.GetOptions{})
	if err == nil {
		pvc.ResourceVersion = existing.ResourceVersion
		pvc.Labels = mergeStringMap(existing.Labels, pvc.Labels)
		pvc.Annotations = mergeStringMap(existing.Annotations, pvc.Annotations)
		_, err = c.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Update(ctx, pvc, metav1.UpdateOptions{})
		if err != nil {
			logx.Errorf("更新 Kubernetes PVC 失败: namespace=%s name=%s err=%v", namespace, pvc.Name, err)
			return err
		}
		return nil
	}
	if !apierrors.IsNotFound(err) {
		logx.Errorf("查询 Kubernetes PVC 失败: namespace=%s name=%s err=%v", namespace, pvc.Name, err)
		return err
	}
	_, err = c.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		logx.Errorf("创建 Kubernetes PVC 失败: namespace=%s name=%s err=%v", namespace, pvc.Name, err)
		return err
	}
	return nil
}

func (c *Client) DeletePersistentVolumeClaim(ctx context.Context, namespace, name string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return fmt.Errorf("Kubernetes PVC 参数不完整")
	}
	if err := c.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		logx.Errorf("删除 Kubernetes PVC 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	return nil
}

func (c *Client) GetServiceAccount(ctx context.Context, namespace, name string) (*corev1.ServiceAccount, error) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("Kubernetes ServiceAccount 参数不完整")
	}
	sa, err := c.kubeClient.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("Kubernetes ServiceAccount 不存在: %s", name)
		}
		logx.Errorf("读取 Kubernetes ServiceAccount 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return nil, err
	}
	return sa, nil
}

func (c *Client) DescribeServiceAccount(ctx context.Context, namespace, name string) (string, error) {
	sa, err := c.GetServiceAccount(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	secrets := make([]string, 0, len(sa.Secrets))
	for _, item := range sa.Secrets {
		secrets = append(secrets, item.Name)
	}
	imagePullSecrets := make([]string, 0, len(sa.ImagePullSecrets))
	for _, item := range sa.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, item.Name)
	}
	lines := []string{
		fmt.Sprintf("Secrets: %s", strings.Join(secrets, ", ")),
		fmt.Sprintf("ImagePullSecrets: %s", strings.Join(imagePullSecrets, ", ")),
	}
	return c.describeObject(ctx, sa, "ServiceAccount", "v1", lines), nil
}

func (c *Client) ApplyServiceAccount(ctx context.Context, namespace string, sa *corev1.ServiceAccount) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" || sa == nil || strings.TrimSpace(sa.Name) == "" {
		return fmt.Errorf("Kubernetes ServiceAccount 参数不完整")
	}
	sa.Namespace = namespace
	existing, err := c.kubeClient.CoreV1().ServiceAccounts(namespace).Get(ctx, sa.Name, metav1.GetOptions{})
	if err == nil {
		sa.ResourceVersion = existing.ResourceVersion
		sa.Labels = mergeStringMap(existing.Labels, sa.Labels)
		sa.Annotations = mergeStringMap(existing.Annotations, sa.Annotations)
		_, err = c.kubeClient.CoreV1().ServiceAccounts(namespace).Update(ctx, sa, metav1.UpdateOptions{})
		if err != nil {
			logx.Errorf("更新 Kubernetes ServiceAccount 失败: namespace=%s name=%s err=%v", namespace, sa.Name, err)
			return err
		}
		return nil
	}
	if !apierrors.IsNotFound(err) {
		logx.Errorf("查询 Kubernetes ServiceAccount 失败: namespace=%s name=%s err=%v", namespace, sa.Name, err)
		return err
	}
	_, err = c.kubeClient.CoreV1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil {
		logx.Errorf("创建 Kubernetes ServiceAccount 失败: namespace=%s name=%s err=%v", namespace, sa.Name, err)
		return err
	}
	return nil
}

func (c *Client) DeleteServiceAccount(ctx context.Context, namespace, name string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return fmt.Errorf("Kubernetes ServiceAccount 参数不完整")
	}
	if err := c.kubeClient.CoreV1().ServiceAccounts(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		logx.Errorf("删除 Kubernetes ServiceAccount 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	return nil
}

func (c *Client) CancelPipelineRun(ctx context.Context, namespace, name string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return fmt.Errorf("Tekton PipelineRun 参数不完整")
	}
	patch := []byte(`{"spec":{"status":"CancelledRunFinally"}}`)
	var lastErr error
	for _, gvr := range pipelineRunGVRs {
		_, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
		if err == nil {
			return nil
		}
		if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			lastErr = err
		}
	}
	if lastErr != nil {
		logx.Errorf("取消 Tekton PipelineRun 失败: namespace=%s name=%s err=%v", namespace, name, lastErr)
		return lastErr
	}
	return fmt.Errorf("Tekton PipelineRun 不存在")
}

func (c *Client) DeletePipelineRun(ctx context.Context, namespace, name string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return fmt.Errorf("Tekton PipelineRun 参数不完整")
	}
	var lastErr error
	for _, gvr := range pipelineRunGVRs {
		err := c.dynamicClient.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err == nil || apierrors.IsNotFound(err) {
			return nil
		}
		if meta.IsNoMatchError(err) {
			continue
		}
		lastErr = err
	}
	if lastErr != nil {
		logx.Errorf("删除 Tekton PipelineRun 失败: namespace=%s name=%s err=%v", namespace, name, lastErr)
		return lastErr
	}
	return fmt.Errorf("Tekton PipelineRun 不存在")
}

func (c *Client) DeleteStepResource(ctx context.Context, namespace, content string) (StepResource, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return StepResource{}, fmt.Errorf("Tekton taskNamespace 不能为空")
	}
	resource, err := ParseStepResource(content)
	if err != nil {
		return StepResource{}, err
	}
	var lastErr error
	for _, gvr := range gvrCandidates(resource.APIVersion) {
		err := c.dynamicClient.Resource(gvr).Namespace(namespace).Delete(ctx, resource.Name, metav1.DeleteOptions{})
		if err == nil || apierrors.IsNotFound(err) {
			resource.Namespace = namespace
			return resource, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		logx.Errorf("删除 Tekton Task 失败: namespace=%s name=%s err=%v", namespace, resource.Name, lastErr)
		return StepResource{}, lastErr
	}
	return StepResource{}, fmt.Errorf("未找到可用的 Tekton Task API")
}

func (c *Client) ValidateStepSynced(ctx context.Context, namespace string, resource StepResource) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" || strings.TrimSpace(resource.Name) == "" {
		return fmt.Errorf("Tekton Task 参数不完整")
	}
	var lastErr error
	for _, gvr := range gvrCandidates(resource.APIVersion) {
		obj, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, resource.Name, metav1.GetOptions{})
		if err == nil {
			hash := strings.TrimSpace(obj.GetAnnotations()[AnnotationStepHash])
			if hash != resource.Hash {
				return fmt.Errorf("Tekton Task %s/%s 未同步或内容不一致", namespace, resource.Name)
			}
			return nil
		}
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			continue
		}
		lastErr = err
	}
	if lastErr != nil {
		logx.Errorf("校验 Tekton Task 同步状态失败: namespace=%s name=%s err=%v", namespace, resource.Name, lastErr)
		return lastErr
	}
	return fmt.Errorf("Tekton Task %s/%s 未同步", namespace, resource.Name)
}

func (c *Client) NamespaceExists(ctx context.Context, namespace string) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return fmt.Errorf("Tekton namespace 不能为空")
	}
	if _, err := c.kubeClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("Tekton namespace 不存在")
		}
		logx.Errorf("校验 Tekton namespace 失败: namespace=%s err=%v", namespace, err)
		return err
	}
	return nil
}

func (c *Client) EnsureNamespace(ctx context.Context, namespace string) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return fmt.Errorf("Tekton namespace 不能为空")
	}
	if _, err := c.kubeClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{}); err == nil {
		return nil
	} else if !apierrors.IsNotFound(err) {
		logx.Errorf("校验 Tekton namespace 失败: namespace=%s err=%v", namespace, err)
		return err
	}
	if _, err := c.kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	}, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		logx.Errorf("创建 Tekton namespace 失败: namespace=%s err=%v", namespace, err)
		return err
	}
	return nil
}

func (c *Client) ServiceAccountExists(ctx context.Context, namespace, name string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if _, err := c.kubeClient.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("Tekton serviceAccountName 不存在")
		}
		logx.Errorf("校验 Tekton serviceAccount 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	return nil
}

func (c *Client) SecretExists(ctx context.Context, namespace, name string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if _, err := c.kubeClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("Kubernetes Secret 不存在: %s", name)
		}
		logx.Errorf("校验 Kubernetes Secret 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	return nil
}

func (c *Client) PersistentVolumeClaimExists(ctx context.Context, namespace, name string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if _, err := c.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("Kubernetes PVC 不存在: %s", name)
		}
		logx.Errorf("校验 Kubernetes PVC 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	return nil
}

func (c *Client) ConfigMapExists(ctx context.Context, namespace, name string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if _, err := c.kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("Kubernetes ConfigMap 不存在: %s", name)
		}
		logx.Errorf("校验 Kubernetes ConfigMap 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	return nil
}

func (c *Client) StorageClassExists(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if _, err := c.kubeClient.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("Kubernetes StorageClass 不存在: %s", name)
		}
		logx.Errorf("校验 Kubernetes StorageClass 失败: name=%s err=%v", name, err)
		return err
	}
	return nil
}

func (c *Client) ListPersistentVolumeClaims(ctx context.Context, namespace, keyword string) ([]PVCInfo, error) {
	namespace = strings.TrimSpace(namespace)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if namespace == "" {
		return nil, fmt.Errorf("Kubernetes PVC namespace 不能为空")
	}
	list, err := c.kubeClient.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		logx.Errorf("查询 Kubernetes PVC 列表失败: namespace=%s err=%v", namespace, err)
		return nil, err
	}
	result := make([]PVCInfo, 0, len(list.Items))
	for _, item := range list.Items {
		if keyword != "" && !strings.Contains(strings.ToLower(item.Name), keyword) {
			continue
		}
		accessModes := make([]string, 0, len(item.Spec.AccessModes))
		for _, mode := range item.Spec.AccessModes {
			accessModes = append(accessModes, string(mode))
		}
		storageClassName := ""
		if item.Spec.StorageClassName != nil {
			storageClassName = *item.Spec.StorageClassName
		}
		volumeMode := ""
		if item.Spec.VolumeMode != nil {
			volumeMode = string(*item.Spec.VolumeMode)
		}
		storage := ""
		if quantity, ok := item.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			storage = quantity.String()
		}
		result = append(result, PVCInfo{
			Name:             item.Name,
			Namespace:        item.Namespace,
			StorageClassName: storageClassName,
			Storage:          storage,
			AccessModes:      accessModes,
			VolumeMode:       volumeMode,
			Status:           string(item.Status.Phase),
			CreatedAt:        item.CreationTimestamp.Unix(),
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (c *Client) ListSecrets(ctx context.Context, namespace, keyword string) ([]SecretInfo, error) {
	namespace = strings.TrimSpace(namespace)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if namespace == "" {
		return nil, fmt.Errorf("Kubernetes Secret namespace 不能为空")
	}
	list, err := c.kubeClient.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		logx.Errorf("查询 Kubernetes Secret 列表失败: namespace=%s err=%v", namespace, err)
		return nil, err
	}
	result := make([]SecretInfo, 0, len(list.Items))
	for _, item := range list.Items {
		if keyword != "" && !strings.Contains(strings.ToLower(item.Name), keyword) {
			continue
		}
		keys := make([]string, 0, len(item.Data))
		for key := range item.Data {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		result = append(result, SecretInfo{
			Name:              item.Name,
			Type:              string(item.Type),
			Namespace:         item.Namespace,
			Keys:              keys,
			ManagedByKubeNova: item.Labels["kube-nova.io/managed-by"] == "kube-nova",
			CreatedAt:         item.CreationTimestamp.Unix(),
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (c *Client) ListConfigMaps(ctx context.Context, namespace, keyword string) ([]ConfigMapInfo, error) {
	namespace = strings.TrimSpace(namespace)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if namespace == "" {
		return nil, fmt.Errorf("Kubernetes ConfigMap namespace 不能为空")
	}
	list, err := c.kubeClient.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		logx.Errorf("查询 Kubernetes ConfigMap 列表失败: namespace=%s err=%v", namespace, err)
		return nil, err
	}
	result := make([]ConfigMapInfo, 0, len(list.Items))
	for _, item := range list.Items {
		if keyword != "" && !strings.Contains(strings.ToLower(item.Name), keyword) {
			continue
		}
		keys := make([]string, 0, len(item.Data)+len(item.BinaryData))
		for key := range item.Data {
			keys = append(keys, key)
		}
		for key := range item.BinaryData {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		result = append(result, ConfigMapInfo{
			Name:      item.Name,
			Namespace: item.Namespace,
			Keys:      keys,
			CreatedAt: item.CreationTimestamp.Unix(),
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (c *Client) GetConfigMap(ctx context.Context, namespace, name string) (*corev1.ConfigMap, error) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("Kubernetes ConfigMap 参数不完整")
	}
	cm, err := c.kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("Kubernetes ConfigMap 不存在: %s", name)
		}
		logx.Errorf("读取 Kubernetes ConfigMap 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return nil, err
	}
	return cm, nil
}

func (c *Client) DescribeConfigMap(ctx context.Context, namespace, name string) (string, error) {
	cm, err := c.GetConfigMap(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	keys := make([]string, 0, len(cm.Data)+len(cm.BinaryData))
	for key := range cm.Data {
		keys = append(keys, key)
	}
	for key := range cm.BinaryData {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := []string{
		fmt.Sprintf("Data Keys: %s", strings.Join(keys, ", ")),
	}
	return c.describeObject(ctx, cm, "ConfigMap", "v1", lines), nil
}

func (c *Client) ApplyConfigMap(ctx context.Context, namespace string, cm *corev1.ConfigMap) error {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" || cm == nil || strings.TrimSpace(cm.Name) == "" {
		return fmt.Errorf("Kubernetes ConfigMap 参数不完整")
	}
	cm.Namespace = namespace
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	if cm.BinaryData == nil {
		cm.BinaryData = map[string][]byte{}
	}
	existing, err := c.kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, cm.Name, metav1.GetOptions{})
	if err == nil {
		cm.ResourceVersion = existing.ResourceVersion
		cm.Labels = mergeStringMap(existing.Labels, cm.Labels)
		cm.Annotations = mergeStringMap(existing.Annotations, cm.Annotations)
		_, err = c.kubeClient.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
		if err != nil {
			logx.Errorf("更新 Kubernetes ConfigMap 失败: namespace=%s name=%s err=%v", namespace, cm.Name, err)
			return err
		}
		return nil
	}
	if !apierrors.IsNotFound(err) {
		logx.Errorf("查询 Kubernetes ConfigMap 失败: namespace=%s name=%s err=%v", namespace, cm.Name, err)
		return err
	}
	_, err = c.kubeClient.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		logx.Errorf("创建 Kubernetes ConfigMap 失败: namespace=%s name=%s err=%v", namespace, cm.Name, err)
		return err
	}
	return nil
}

func (c *Client) DeleteConfigMap(ctx context.Context, namespace, name string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return fmt.Errorf("Kubernetes ConfigMap 参数不完整")
	}
	if err := c.kubeClient.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		logx.Errorf("删除 Kubernetes ConfigMap 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	return nil
}

func (c *Client) ListProjectSecrets(ctx context.Context, namespace, keyword, projectID, bindingID string) ([]SecretInfo, error) {
	namespace = strings.TrimSpace(namespace)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	projectID = strings.TrimSpace(projectID)
	bindingID = strings.TrimSpace(bindingID)
	if namespace == "" || projectID == "" || bindingID == "" {
		return nil, fmt.Errorf("Kubernetes Secret 查询参数不完整")
	}
	list, err := c.kubeClient.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf(
			"kube-nova.io/managed-by=kube-nova,kube-nova.io/devops-project-id=%s,kube-nova.io/devops-binding-id=%s",
			projectID,
			bindingID,
		),
	})
	if err != nil {
		logx.Errorf("查询项目 Kubernetes Secret 列表失败: namespace=%s err=%v", namespace, err)
		return nil, err
	}
	result := make([]SecretInfo, 0, len(list.Items))
	for _, item := range list.Items {
		if keyword != "" && !strings.Contains(strings.ToLower(item.Name), keyword) {
			continue
		}
		keys := make([]string, 0, len(item.Data))
		for key := range item.Data {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		result = append(result, SecretInfo{
			Name:              item.Name,
			Type:              string(item.Type),
			Namespace:         item.Namespace,
			Keys:              keys,
			ManagedByKubeNova: true,
			CreatedAt:         item.CreationTimestamp.Unix(),
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (c *Client) GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("Kubernetes Secret 参数不完整")
	}
	secret, err := c.kubeClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("Kubernetes Secret 不存在: %s", name)
		}
		logx.Errorf("读取 Kubernetes Secret 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return nil, err
	}
	return secret, nil
}

func (c *Client) GetProjectSecret(ctx context.Context, namespace, name, projectID, bindingID string) (*corev1.Secret, error) {
	secret, err := c.GetSecret(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	if !isProjectSecret(secret, projectID, bindingID) {
		return nil, fmt.Errorf("Kubernetes Secret 不属于当前项目 Tekton 渠道")
	}
	return secret, nil
}

func (c *Client) DescribeProjectSecret(ctx context.Context, namespace, name, projectID, bindingID string) (string, error) {
	secret, err := c.GetProjectSecret(ctx, namespace, name, projectID, bindingID)
	if err != nil {
		return "", err
	}
	keys := make([]string, 0, len(secret.Data)+len(secret.StringData))
	for key := range secret.Data {
		keys = append(keys, key)
	}
	for key := range secret.StringData {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := []string{
		fmt.Sprintf("Type: %s", secret.Type),
		fmt.Sprintf("Data Keys: %s", strings.Join(keys, ", ")),
	}
	return c.describeObject(ctx, secret, "Secret", "v1", lines), nil
}

func (c *Client) CreateSecret(ctx context.Context, namespace string, secret *corev1.Secret) error {
	if err := prepareSecret(namespace, secret); err != nil {
		return err
	}
	if _, err := c.kubeClient.CoreV1().Secrets(namespace).Get(ctx, secret.Name, metav1.GetOptions{}); err == nil {
		return fmt.Errorf("Kubernetes Secret 已存在: %s", secret.Name)
	} else if !apierrors.IsNotFound(err) {
		logx.Errorf("查询 Kubernetes Secret 失败: namespace=%s name=%s err=%v", namespace, secret.Name, err)
		return err
	}
	if _, err := c.kubeClient.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		logx.Errorf("创建 Kubernetes Secret 失败: namespace=%s name=%s err=%v", namespace, secret.Name, err)
		return err
	}
	return nil
}

func (c *Client) UpdateSecret(ctx context.Context, namespace string, secret *corev1.Secret) error {
	if err := prepareSecret(namespace, secret); err != nil {
		return err
	}
	existing, err := c.kubeClient.CoreV1().Secrets(namespace).Get(ctx, secret.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("Kubernetes Secret 不存在: %s", secret.Name)
		}
		logx.Errorf("查询 Kubernetes Secret 失败: namespace=%s name=%s err=%v", namespace, secret.Name, err)
		return err
	}
	if !isProjectSecret(existing, secret.Labels["kube-nova.io/devops-project-id"], secret.Labels["kube-nova.io/devops-binding-id"]) {
		return fmt.Errorf("Kubernetes Secret 不属于当前项目 Tekton 渠道")
	}
	secret.ResourceVersion = existing.ResourceVersion
	secret.Labels = mergeStringMap(existing.Labels, secret.Labels)
	secret.Annotations = mergeStringMap(existing.Annotations, secret.Annotations)
	if _, err := c.kubeClient.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
		logx.Errorf("更新 Kubernetes Secret 失败: namespace=%s name=%s err=%v", namespace, secret.Name, err)
		return err
	}
	return nil
}

func (c *Client) ApplyProjectSecret(ctx context.Context, namespace string, secret *corev1.Secret) error {
	if err := prepareSecret(namespace, secret); err != nil {
		return err
	}
	namespace = strings.TrimSpace(namespace)
	existing, err := c.kubeClient.CoreV1().Secrets(namespace).Get(ctx, secret.Name, metav1.GetOptions{})
	if err == nil {
		if !isProjectSecret(existing, secret.Labels["kube-nova.io/devops-project-id"], secret.Labels["kube-nova.io/devops-binding-id"]) {
			return fmt.Errorf("Kubernetes Secret 不属于当前项目 Tekton 渠道")
		}
		secret.ResourceVersion = existing.ResourceVersion
		secret.Labels = mergeStringMap(existing.Labels, secret.Labels)
		secret.Annotations = mergeStringMap(existing.Annotations, secret.Annotations)
		if _, err := c.kubeClient.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
			logx.Errorf("更新 Kubernetes Secret 失败: namespace=%s name=%s err=%v", namespace, secret.Name, err)
			return err
		}
		return nil
	}
	if !apierrors.IsNotFound(err) {
		logx.Errorf("查询 Kubernetes Secret 失败: namespace=%s name=%s err=%v", namespace, secret.Name, err)
		return err
	}
	if _, err := c.kubeClient.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		logx.Errorf("创建 Kubernetes Secret 失败: namespace=%s name=%s err=%v", namespace, secret.Name, err)
		return err
	}
	return nil
}

func (c *Client) ApplySecret(ctx context.Context, namespace string, secret *corev1.Secret) error {
	if err := prepareSecret(namespace, secret); err != nil {
		return err
	}
	namespace = strings.TrimSpace(namespace)

	existing, err := c.kubeClient.CoreV1().Secrets(namespace).Get(ctx, secret.Name, metav1.GetOptions{})
	if err == nil {
		secret.ResourceVersion = existing.ResourceVersion
		secret.Labels = mergeStringMap(existing.Labels, secret.Labels)
		secret.Annotations = mergeStringMap(existing.Annotations, secret.Annotations)
		if _, err := c.kubeClient.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
			logx.Errorf("更新 Kubernetes Secret 失败: namespace=%s name=%s err=%v", namespace, secret.Name, err)
			return err
		}
		return nil
	}
	if !apierrors.IsNotFound(err) {
		logx.Errorf("查询 Kubernetes Secret 失败: namespace=%s name=%s err=%v", namespace, secret.Name, err)
		return err
	}
	if _, err := c.kubeClient.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		logx.Errorf("创建 Kubernetes Secret 失败: namespace=%s name=%s err=%v", namespace, secret.Name, err)
		return err
	}
	return nil
}

func (c *Client) DeleteProjectSecret(ctx context.Context, namespace, name, projectID, bindingID string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return fmt.Errorf("Kubernetes Secret 参数不完整")
	}
	secret, err := c.kubeClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		logx.Errorf("读取 Kubernetes Secret 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	if !isProjectSecret(secret, projectID, bindingID) {
		return fmt.Errorf("Kubernetes Secret 不属于当前项目 Tekton 渠道")
	}
	return c.DeleteSecret(ctx, namespace, name)
}

func (c *Client) DeleteSecret(ctx context.Context, namespace, name string) error {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return fmt.Errorf("Kubernetes Secret 参数不完整")
	}
	if err := c.kubeClient.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		logx.Errorf("删除 Kubernetes Secret 失败: namespace=%s name=%s err=%v", namespace, name, err)
		return err
	}
	return nil
}

func (c *Client) PodLog(ctx context.Context, namespace, podName, containerName string) (string, error) {
	namespace = strings.TrimSpace(namespace)
	podName = strings.TrimSpace(podName)
	containerName = strings.TrimSpace(containerName)
	if namespace == "" || podName == "" {
		return "", fmt.Errorf("Pod 日志参数不完整")
	}
	req := c.kubeClient.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{Container: containerName})
	data, err := req.DoRaw(ctx)
	if err != nil {
		logx.Errorf("读取 Tekton Pod 日志失败: namespace=%s pod=%s container=%s err=%v", namespace, podName, containerName, err)
		return "", err
	}
	return string(data), nil
}

func (c *Client) PodLogs(ctx context.Context, namespace, podName, containerName string) (string, error) {
	namespace = strings.TrimSpace(namespace)
	podName = strings.TrimSpace(podName)
	containerName = strings.TrimSpace(containerName)
	if namespace == "" || podName == "" {
		return "", fmt.Errorf("Pod 日志参数不完整")
	}
	if containerName != "" {
		return c.PodLog(ctx, namespace, podName, containerName)
	}
	pod, err := c.kubeClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		logx.Errorf("读取 Tekton Pod 失败: namespace=%s pod=%s err=%v", namespace, podName, err)
		return "", err
	}
	var builder strings.Builder
	for _, container := range pod.Spec.InitContainers {
		appendPodContainerLog(ctx, c, namespace, podName, container.Name, &builder)
	}
	for _, container := range pod.Spec.Containers {
		appendPodContainerLog(ctx, c, namespace, podName, container.Name, &builder)
	}
	content := strings.TrimSpace(builder.String())
	if content == "" {
		return "", fmt.Errorf("Pod 日志暂未就绪")
	}
	return content, nil
}

func ParseStepResource(content string) (StepResource, error) {
	resource, err := parseResource(content)
	if err != nil {
		return StepResource{}, err
	}
	if resource.Kind != "Task" {
		return StepResource{}, fmt.Errorf("Tekton 步骤库只维护 Task")
	}
	if !validateTektonDNSLabel(resource.Name) {
		return StepResource{}, fmt.Errorf("Tekton Task metadata.name 不符合 DNS_LABEL 规范：%s", resource.Name)
	}
	steps, ok, err := unstructured.NestedSlice(resource.Object.Object, "spec", "steps")
	if err != nil || !ok || len(steps) == 0 {
		return StepResource{}, fmt.Errorf("Tekton Task spec.steps 不能为空")
	}
	for _, item := range steps {
		step, ok := item.(map[string]any)
		if !ok || strings.TrimSpace(fmt.Sprint(step["name"])) == "" || !hasTektonStepImageOrRef(step) {
			return StepResource{}, fmt.Errorf("Tekton Task spec.steps 的 name 和 image/ref 不能为空")
		}
		if name := tektonStepString(step["name"]); !validateTektonDNSLabel(name) {
			return StepResource{}, fmt.Errorf("Tekton Task spec.steps.name 不符合 DNS_LABEL 规范：%s", name)
		}
		if err := validateTektonTaskStepExecution(step); err != nil {
			return StepResource{}, err
		}
		if hasTektonStepRef(step) {
			if conflicts := tektonStepRefConflictFields(step); len(conflicts) > 0 {
				return StepResource{}, fmt.Errorf("Tekton Task spec.steps 使用 ref 时不能同时配置 %s", strings.Join(conflicts, "、"))
			}
		}
	}
	if err := validateTektonTaskSidecars(resource.Object.Object); err != nil {
		return StepResource{}, err
	}
	if err := validateTektonTaskVolumes(resource.Object.Object); err != nil {
		return StepResource{}, err
	}
	if err := validateTektonTaskObjectProperties(resource.Object.Object, "params"); err != nil {
		return StepResource{}, err
	}
	if err := validateTektonTaskObjectProperties(resource.Object.Object, "results"); err != nil {
		return StepResource{}, err
	}
	if err := validateTektonTaskTypedValues(resource.Object.Object); err != nil {
		return StepResource{}, err
	}
	resource.Object.SetNamespace("")
	resource.Namespace = ""
	return resource, nil
}

func validateTektonTaskTypedValues(obj map[string]any) error {
	if err := validateTektonTaskTypedValueField(obj, "params", "default", "默认值"); err != nil {
		return err
	}
	if err := validateTektonTaskTypedValueField(obj, "results", "value", "value"); err != nil {
		return err
	}
	return nil
}

func validateTektonTaskTypedValueField(obj map[string]any, field, valueField, label string) error {
	items, ok, err := unstructured.NestedSlice(obj, "spec", field)
	if err != nil || !ok {
		return nil
	}
	for _, item := range items {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		raw, exists := data[valueField]
		if !exists || raw == nil {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(data["name"]))
		if name == "" {
			name = "未命名"
		}
		itemType := strings.TrimSpace(fmt.Sprint(data["type"]))
		if itemType == "" {
			itemType = "string"
		}
		if err := validateTektonTaskParamValueShape(itemType, raw); err != nil {
			return fmt.Errorf("Tekton Task spec.%s %s %s%s", field, name, label, err.Error())
		}
	}
	return nil
}

func validateTektonTaskParamValueShape(itemType string, raw any) error {
	switch itemType {
	case "array":
		items, ok := raw.([]any)
		if !ok {
			return fmt.Errorf("必须是数组")
		}
		for _, item := range items {
			if _, ok := item.(string); !ok {
				return fmt.Errorf("必须是字符串数组")
			}
		}
		return nil
	case "object":
		if _, ok := raw.(map[string]any); !ok {
			return fmt.Errorf("必须是对象")
		}
		return nil
	default:
		if _, ok := raw.(string); !ok {
			return fmt.Errorf("必须是字符串")
		}
		return nil
	}
}

func validateTektonTaskObjectProperties(obj map[string]any, field string) error {
	items, ok, err := unstructured.NestedSlice(obj, "spec", field)
	if err != nil || !ok {
		return nil
	}
	for _, item := range items {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(data["name"]))
		itemType := tektonStepString(data["type"])
		switch itemType {
		case "", "string", "array", "object":
		default:
			return fmt.Errorf("Tekton Task spec.%s %s type 只支持 string、array、object", field, name)
		}
		_, hasProperties := data["properties"]
		if hasProperties && itemType != "object" {
			return fmt.Errorf("Tekton Task spec.%s %s 配置 properties 时类型必须是 object", field, name)
		}
		if itemType != "object" {
			continue
		}
		if field == "params" && strings.Contains(name, ".") {
			return fmt.Errorf("Tekton Task spec.params %s 类型为 object 时名称不能包含点号", name)
		}
		properties, ok := data["properties"].(map[string]any)
		if !ok || len(properties) == 0 {
			if name == "" {
				name = "未命名"
			}
			return fmt.Errorf("Tekton Task spec.%s %s 类型为 object 时必须配置 properties", field, name)
		}
		for propertyName, property := range properties {
			propertySpec, ok := property.(map[string]any)
			if !ok {
				return fmt.Errorf("Tekton Task spec.%s %s properties %s 必须是对象", field, name, propertyName)
			}
			switch propertyType := strings.TrimSpace(fmt.Sprint(propertySpec["type"])); propertyType {
			case "", "string", "array", "object":
			default:
				return fmt.Errorf("Tekton Task spec.%s %s properties %s 类型不支持", field, name, propertyName)
			}
		}
	}
	return nil
}

func validateTektonTaskSidecars(obj map[string]any) error {
	items, ok, err := unstructured.NestedSlice(obj, "spec", "sidecars")
	if err != nil || !ok {
		return nil
	}
	names := map[string]struct{}{}
	for _, item := range items {
		sidecar, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("Tekton Task spec.sidecars 数组项必须是对象")
		}
		name := tektonStepString(sidecar["name"])
		if name == "" {
			return fmt.Errorf("Tekton Task spec.sidecars.name 不能为空")
		}
		if !validateTektonDNSLabel(name) {
			return fmt.Errorf("Tekton Task spec.sidecars.name 不符合 DNS_LABEL 规范：%s", name)
		}
		if _, exists := names[name]; exists {
			return fmt.Errorf("Tekton Task spec.sidecars.name 不能重复：%s", name)
		}
		names[name] = struct{}{}
		if tektonStepString(sidecar["image"]) == "" {
			return fmt.Errorf("Tekton Task spec.sidecars %s image 不能为空", name)
		}
	}
	return nil
}

func validateTektonTaskVolumes(obj map[string]any) error {
	items, ok, err := unstructured.NestedSlice(obj, "spec", "volumes")
	if err != nil || !ok {
		return nil
	}
	names := map[string]struct{}{}
	for _, item := range items {
		volume, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("Tekton Task spec.volumes 数组项必须是对象")
		}
		name := tektonStepString(volume["name"])
		if name == "" {
			return fmt.Errorf("Tekton Task spec.volumes.name 不能为空")
		}
		if !validateTektonDNSLabel(name) {
			return fmt.Errorf("Tekton Task spec.volumes.name 不符合 DNS_LABEL 规范：%s", name)
		}
		if _, exists := names[name]; exists {
			return fmt.Errorf("Tekton Task spec.volumes.name 不能重复：%s", name)
		}
		names[name] = struct{}{}
		hasSource := false
		for key := range volume {
			if key == "name" {
				continue
			}
			if _, ok := tektonTaskVolumeSourceKeys[key]; ok {
				hasSource = true
				break
			}
		}
		if !hasSource {
			return fmt.Errorf("Tekton Task spec.volumes %s 必须配置支持的卷来源", name)
		}
	}
	return nil
}

func validateTektonTaskStepExecution(step map[string]any) error {
	name := tektonStepString(step["name"])
	if name == "" {
		name = "未命名"
	}
	if onError := tektonStepString(step["onError"]); onError != "" && onError != "continue" && onError != "stopAndFail" {
		return fmt.Errorf("Tekton Task spec.steps %s onError 只支持 continue 或 stopAndFail", name)
	}
	if timeout := tektonStepString(step["timeout"]); timeout != "" && !tektonDurationPattern.MatchString(timeout) {
		return fmt.Errorf("Tekton Task spec.steps %s timeout 格式不合法", name)
	}
	return nil
}

func hasTektonStepImageOrRef(step map[string]any) bool {
	if tektonStepString(step["image"]) != "" {
		return true
	}
	return hasTektonStepRef(step)
}

func hasTektonStepRef(step map[string]any) bool {
	ref, ok := step["ref"]
	if !ok || ref == nil {
		return false
	}
	if text, ok := ref.(string); ok {
		return strings.TrimSpace(text) != ""
	}
	if data, ok := ref.(map[string]any); ok {
		return tektonStepString(data["name"]) != "" || tektonStepString(data["resolver"]) != ""
	}
	return false
}

func tektonStepRefConflictFields(step map[string]any) []string {
	fields := []string{
		"image",
		"command",
		"args",
		"script",
		"workingDir",
		"env",
		"envFrom",
		"volumeMounts",
		"volumeDevices",
		"computeResources",
		"resources",
		"securityContext",
		"timeout",
		"onError",
		"stdoutConfig",
		"stderrConfig",
		"results",
		"imagePullPolicy",
		"ports",
		"livenessProbe",
		"readinessProbe",
		"startupProbe",
	}
	conflicts := make([]string, 0)
	for _, field := range fields {
		if hasTektonStepValue(step[field]) {
			conflicts = append(conflicts, field)
		}
	}
	return conflicts
}

func hasTektonStepValue(value any) bool {
	switch data := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(data) != ""
	case []any:
		return len(data) > 0
	case map[string]any:
		return len(data) > 0
	default:
		return true
	}
}

func tektonStepString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func validateTektonDNSLabel(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) <= 63 && tektonDNSLabelPattern.MatchString(value)
}

func appendPodContainerLog(ctx context.Context, c *Client, namespace, podName, containerName string, builder *strings.Builder) {
	if containerName == "" || builder == nil {
		return
	}
	content, err := c.PodLog(ctx, namespace, podName, containerName)
	if err != nil || strings.TrimSpace(content) == "" {
		return
	}
	if builder.Len() > 0 {
		builder.WriteString("\n")
	}
	builder.WriteString("===== ")
	builder.WriteString(containerName)
	builder.WriteString(" =====\n")
	builder.WriteString(content)
}

func statusFromObject(obj *unstructured.Unstructured) RunStatus {
	status := RunStatus{
		Status:       "running",
		StartedAt:    tektonTime(obj, "startTime"),
		FinishedAt:   tektonTime(obj, "completionTime"),
		SkippedTasks: skippedTasksFromObject(obj),
	}
	conditions, ok, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !ok || len(conditions) == 0 {
		return status
	}
	for _, item := range conditions {
		condition, ok := item.(map[string]any)
		if !ok || strings.TrimSpace(fmt.Sprint(condition["type"])) != "Succeeded" {
			continue
		}
		raw := cleanAnyString(condition["status"])
		reason := cleanAnyString(condition["reason"])
		message := cleanAnyString(condition["message"])
		status.RawSucceeded = raw
		status.Reason = reason
		status.Message = message
		switch strings.ToLower(raw) {
		case "true":
			status.Status = "success"
		case "false":
			if strings.Contains(strings.ToLower(reason), "cancel") || strings.Contains(strings.ToLower(reason), "stop") {
				status.Status = "aborted"
			} else {
				status.Status = "failed"
			}
		default:
			status.Status = "running"
		}
		return status
	}
	return status
}

func skippedTasksFromObject(obj *unstructured.Unstructured) []SkippedTaskInfo {
	items, ok, err := unstructured.NestedSlice(obj.Object, "status", "skippedTasks")
	if err != nil || !ok || len(items) == 0 {
		return nil
	}
	result := make([]SkippedTaskInfo, 0, len(items))
	for _, item := range items {
		task, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(task["name"]))
		if name == "" {
			continue
		}
		result = append(result, SkippedTaskInfo{
			Name:    name,
			Reason:  cleanAnyString(task["reason"]),
			Message: cleanAnyString(task["message"]),
		})
	}
	return result
}

func tektonTime(obj *unstructured.Unstructured, field string) time.Time {
	value, ok, _ := unstructured.NestedString(obj.Object, "status", field)
	if !ok || strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

func taskRunContainerName(obj *unstructured.Unstructured) string {
	steps, ok, err := unstructured.NestedSlice(obj.Object, "status", "steps")
	if err != nil || !ok {
		return ""
	}
	for _, item := range steps {
		step, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if container := strings.TrimSpace(fmt.Sprint(step["container"])); container != "" {
			return container
		}
		if name := strings.TrimSpace(fmt.Sprint(step["name"])); name != "" {
			return "step-" + name
		}
	}
	return ""
}

func (c *Client) taskRunContainerNames(ctx context.Context, namespace, podName string, obj *unstructured.Unstructured) []string {
	names := taskRunStatusContainerNames(obj)
	if podName == "" || c == nil || c.kubeClient == nil {
		return names
	}
	pod, err := c.kubeClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		logx.Errorf("读取 Tekton Pod 容器列表失败: namespace=%s pod=%s err=%v", namespace, podName, err)
		return names
	}
	for _, container := range pod.Spec.InitContainers {
		names = appendUniqueString(names, container.Name)
	}
	for _, container := range pod.Spec.Containers {
		names = appendUniqueString(names, container.Name)
	}
	for _, container := range pod.Spec.EphemeralContainers {
		names = appendUniqueString(names, container.Name)
	}
	return names
}

func taskRunStatusContainerNames(obj *unstructured.Unstructured) []string {
	steps, ok, err := unstructured.NestedSlice(obj.Object, "status", "steps")
	if err != nil || !ok {
		return nil
	}
	result := make([]string, 0, len(steps))
	seen := make(map[string]struct{}, len(steps))
	for _, item := range steps {
		step, ok := item.(map[string]any)
		if !ok {
			continue
		}
		container := strings.TrimSpace(fmt.Sprint(step["container"]))
		if container == "" {
			container = strings.TrimSpace(fmt.Sprint(step["name"]))
			if container != "" {
				container = "step-" + container
			}
		}
		if container == "" {
			continue
		}
		if _, ok := seen[container]; ok {
			continue
		}
		seen[container] = struct{}{}
		result = append(result, container)
	}
	return result
}

func podContainerNames(pod *corev1.Pod) []string {
	if pod == nil {
		return nil
	}
	names := make([]string, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers)+len(pod.Spec.EphemeralContainers))
	for _, container := range pod.Spec.InitContainers {
		names = appendUniqueString(names, container.Name)
	}
	for _, container := range pod.Spec.Containers {
		names = appendUniqueString(names, container.Name)
	}
	for _, container := range pod.Spec.EphemeralContainers {
		names = appendUniqueString(names, container.Name)
	}
	return names
}

func defaultPodLogContainer(pod *corev1.Pod) string {
	if pod == nil {
		return ""
	}
	if len(pod.Spec.Containers) > 0 {
		return strings.TrimSpace(pod.Spec.Containers[0].Name)
	}
	if len(pod.Spec.InitContainers) > 0 {
		return strings.TrimSpace(pod.Spec.InitContainers[0].Name)
	}
	if len(pod.Spec.EphemeralContainers) > 0 {
		return strings.TrimSpace(pod.Spec.EphemeralContainers[0].Name)
	}
	return ""
}

func appendUniqueStrings(items []string, values ...string) []string {
	for _, value := range values {
		items = appendUniqueString(items, value)
	}
	return items
}

func stringSliceContains(items []string, value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range items {
		if strings.TrimSpace(item) == value {
			return true
		}
	}
	return false
}

func appendUniqueString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func (c *Client) getDynamicResource(ctx context.Context, gvrs []schema.GroupVersionResource, namespace, name, displayName string) (*unstructured.Unstructured, error) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return nil, fmt.Errorf("%s 参数不完整", strings.TrimSpace(displayName))
	}
	var lastErr error
	for _, gvr := range gvrs {
		obj, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			obj.SetManagedFields(nil)
			return obj, nil
		}
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			continue
		}
		lastErr = err
	}
	if lastErr != nil {
		logx.Errorf("读取 %s 失败: namespace=%s name=%s err=%v", displayName, namespace, name, lastErr)
		return nil, lastErr
	}
	return nil, fmt.Errorf("%s 不存在: %s", displayName, name)
}

func (c *Client) describeObject(ctx context.Context, obj metav1.Object, kind, apiVersion string, lines []string) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Name: %s\n", obj.GetName()))
	builder.WriteString(fmt.Sprintf("Namespace: %s\n", obj.GetNamespace()))
	builder.WriteString(fmt.Sprintf("Kind: %s\n", strings.TrimSpace(kind)))
	builder.WriteString(fmt.Sprintf("API Version: %s\n", strings.TrimSpace(apiVersion)))
	builder.WriteString(fmt.Sprintf("Created At: %s\n", obj.GetCreationTimestamp().Format(time.RFC3339)))
	writeStringMap(&builder, "Labels", obj.GetLabels())
	writeStringMap(&builder, "Annotations", obj.GetAnnotations())
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		builder.WriteString(line + "\n")
	}
	events := c.resourceEvents(ctx, obj.GetNamespace(), obj.GetName(), kind)
	builder.WriteString("\nEvents:\n")
	if len(events) == 0 {
		builder.WriteString("  <none>\n")
		return builder.String()
	}
	for _, event := range events {
		builder.WriteString(fmt.Sprintf(
			"  %s  %s  %s  %s\n",
			event.Type,
			event.Reason,
			event.LastTimestamp.Format(time.RFC3339),
			event.Message,
		))
	}
	return builder.String()
}

func (c *Client) resourceEvents(ctx context.Context, namespace, name, kind string) []corev1.Event {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	kind = strings.TrimSpace(kind)
	if namespace == "" || name == "" || kind == "" || c == nil || c.kubeClient == nil {
		return nil
	}
	selector := fields.AndSelectors(
		fields.OneTermEqualSelector("involvedObject.name", name),
		fields.OneTermEqualSelector("involvedObject.kind", kind),
	)
	list, err := c.kubeClient.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{FieldSelector: selector.String()})
	if err != nil {
		logx.Errorf("查询 Kubernetes 事件失败: namespace=%s kind=%s name=%s err=%v", namespace, kind, name, err)
		return nil
	}
	sort.SliceStable(list.Items, func(i, j int) bool {
		return list.Items[i].LastTimestamp.Before(&list.Items[j].LastTimestamp)
	})
	return list.Items
}

func writeStringMap(builder *strings.Builder, title string, data map[string]string) {
	builder.WriteString(title + ":\n")
	if len(data) == 0 {
		builder.WriteString("  <none>\n")
		return
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		builder.WriteString(fmt.Sprintf("  %s: %s\n", key, data[key]))
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func accessModesToString(items []corev1.PersistentVolumeAccessMode) string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, string(item))
	}
	return strings.Join(values, ", ")
}

func cleanAnyString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func taskInfoListFromObjects(items []unstructured.Unstructured, keyword string) []TaskInfo {
	result := make([]TaskInfo, 0, len(items))
	for index := range items {
		obj := &items[index]
		name := strings.TrimSpace(obj.GetName())
		if keyword != "" && !strings.Contains(strings.ToLower(name), keyword) {
			continue
		}
		params, workspaces, results := tektonTaskSpecCollections(obj)
		result = append(result, TaskInfo{
			Name:        name,
			Namespace:   strings.TrimSpace(obj.GetNamespace()),
			Kind:        strings.TrimSpace(obj.GetKind()),
			APIVersion:  strings.TrimSpace(obj.GetAPIVersion()),
			Description: tektonTaskDescription(obj),
			Params:      params,
			Workspaces:  workspaces,
			Results:     results,
			CreatedAt:   obj.GetCreationTimestamp().Unix(),
			Yaml:        tektonTaskYaml(obj),
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func (c *Client) namedResourceExists(ctx context.Context, gvrs []schema.GroupVersionResource, namespace, name, kind string) error {
	var lastErr error
	for _, gvr := range gvrs {
		var err error
		if strings.TrimSpace(namespace) != "" {
			_, err = c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		} else {
			_, err = c.dynamicClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		}
		if err == nil {
			return nil
		}
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			continue
		}
		lastErr = err
	}
	if lastErr != nil {
		logx.Errorf("校验 Tekton %s 存在失败: namespace=%s name=%s err=%v", kind, namespace, name, lastErr)
		return lastErr
	}
	if strings.TrimSpace(namespace) != "" {
		return fmt.Errorf("Tekton %s %s/%s 不存在", kind, namespace, name)
	}
	return fmt.Errorf("Tekton %s %s 不存在", kind, name)
}

func (c *Client) getTaskInfo(ctx context.Context, gvrs []schema.GroupVersionResource, namespace, name, kind string) (TaskInfo, error) {
	var lastErr error
	for _, gvr := range gvrs {
		var obj *unstructured.Unstructured
		var err error
		if strings.TrimSpace(namespace) != "" {
			obj, err = c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		} else {
			obj, err = c.dynamicClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		}
		if err == nil {
			items := taskInfoListFromObjects([]unstructured.Unstructured{*obj}, "")
			if len(items) == 0 {
				return TaskInfo{}, fmt.Errorf("Tekton %s %s spec 为空", kind, name)
			}
			return items[0], nil
		}
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			continue
		}
		lastErr = err
	}
	if lastErr != nil {
		logx.Errorf("读取 Tekton %s 失败: namespace=%s name=%s err=%v", kind, namespace, name, lastErr)
		return TaskInfo{}, lastErr
	}
	if strings.TrimSpace(namespace) != "" {
		return TaskInfo{}, fmt.Errorf("Tekton %s %s/%s 不存在", kind, namespace, name)
	}
	return TaskInfo{}, fmt.Errorf("Tekton %s %s 不存在", kind, name)
}

func tektonTaskSpecCollections(obj *unstructured.Unstructured) ([]string, []string, []string) {
	params := tektonNameListFromNestedSlice(obj, "spec", "params")
	workspaces := tektonNameListFromNestedSlice(obj, "spec", "workspaces")
	results := tektonNameListFromNestedSlice(obj, "spec", "results")
	return params, workspaces, results
}

func tektonNameListFromNestedSlice(obj *unstructured.Unstructured, fields ...string) []string {
	items, ok, err := unstructured.NestedSlice(obj.Object, fields...)
	if err != nil || !ok || len(items) == 0 {
		return nil
	}
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(data["name"]))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}

func tektonTaskDescription(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	if desc := strings.TrimSpace(obj.GetAnnotations()["tekton.dev/displayName"]); desc != "" {
		return desc
	}
	if desc, ok, _ := unstructured.NestedString(obj.Object, "spec", "displayName"); ok {
		return strings.TrimSpace(desc)
	}
	if desc, ok, _ := unstructured.NestedString(obj.Object, "spec", "description"); ok {
		return strings.TrimSpace(desc)
	}
	return ""
}

func tektonTaskYaml(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	data, err := yaml.Marshal(obj.Object)
	if err != nil {
		return ""
	}
	return string(data)
}

func parseResource(content string) (StepResource, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return StepResource{}, fmt.Errorf("Tekton YAML 不能为空")
	}
	jsonData, err := yaml.YAMLToJSON([]byte(content))
	if err != nil {
		return StepResource{}, fmt.Errorf("Tekton YAML 解析失败: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		return StepResource{}, fmt.Errorf("Tekton YAML 解析失败: %w", err)
	}
	obj := &unstructured.Unstructured{Object: raw}
	apiVersion := strings.TrimSpace(obj.GetAPIVersion())
	kind := strings.TrimSpace(obj.GetKind())
	name := strings.TrimSpace(obj.GetName())
	if apiVersion != tektonV1 && apiVersion != tektonV1Beta1 {
		return StepResource{}, fmt.Errorf("Tekton apiVersion 只支持 tekton.dev/v1 或 tekton.dev/v1beta1")
	}
	if name == "" {
		return StepResource{}, fmt.Errorf("Tekton metadata.name 不能为空")
	}
	return StepResource{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		Namespace:  obj.GetNamespace(),
		Hash:       contentHash(content),
		Object:     obj,
	}, nil
}

func gvrCandidates(apiVersion string) []schema.GroupVersionResource {
	if apiVersion == tektonV1Beta1 {
		return []schema.GroupVersionResource{taskGVRs[1], taskGVRs[0]}
	}
	return taskGVRs
}

func gvrCandidatesByKind(kind, apiVersion string) []schema.GroupVersionResource {
	switch kind {
	case "Pipeline":
		if apiVersion == tektonV1Beta1 {
			return []schema.GroupVersionResource{pipelineGVRs[1], pipelineGVRs[0]}
		}
		return pipelineGVRs
	case "PipelineRun":
		if apiVersion == tektonV1Beta1 {
			return []schema.GroupVersionResource{pipelineRunGVRs[1], pipelineRunGVRs[0]}
		}
		return pipelineRunGVRs
	default:
		return gvrCandidates(apiVersion)
	}
}

func (c *Client) applyNamespacedResource(ctx context.Context, namespace string, resource StepResource, gvrs []schema.GroupVersionResource) (StepResource, error) {
	var lastErr error
	for _, gvr := range gvrs {
		existing, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, resource.Name, metav1.GetOptions{})
		if err == nil {
			resource.Object.SetResourceVersion(existing.GetResourceVersion())
			updated, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Update(ctx, resource.Object, metav1.UpdateOptions{})
			if err == nil {
				resource.Object = updated
				return resource, nil
			}
			lastErr = err
			continue
		}
		if !apierrors.IsNotFound(err) {
			lastErr = err
			continue
		}
		created, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, resource.Object, metav1.CreateOptions{})
		if err == nil {
			resource.Object = created
			return resource, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		logx.Errorf("同步 Tekton 资源失败: kind=%s name=%s namespace=%s err=%v", resource.Kind, resource.Name, namespace, lastErr)
		return StepResource{}, lastErr
	}
	return StepResource{}, fmt.Errorf("未找到可用的 Tekton %s API", resource.Kind)
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return hex.EncodeToString(sum[:])
}

func secretHash(secret *corev1.Secret) string {
	hash := sha256.New()
	hash.Write([]byte(string(secret.Type)))
	keys := make([]string, 0, len(secret.Data))
	for key := range secret.Data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		hash.Write([]byte("\n" + key + "="))
		hash.Write(secret.Data[key])
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func prepareSecret(namespace string, secret *corev1.Secret) error {
	namespace = strings.TrimSpace(namespace)
	if secret == nil {
		return fmt.Errorf("Kubernetes Secret 不能为空")
	}
	secret.Name = strings.TrimSpace(secret.Name)
	if namespace == "" || secret.Name == "" {
		return fmt.Errorf("Kubernetes Secret 参数不完整")
	}
	secret.Namespace = namespace
	if secret.Type == "" {
		secret.Type = corev1.SecretTypeOpaque
	}
	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	secret.Labels["kube-nova.io/managed-by"] = "kube-nova"
	secret.Labels["kube-nova.io/resource"] = "tekton-workspace-secret"
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	secret.Annotations[AnnotationSecretHash] = secretHash(secret)
	return nil
}

func isProjectSecret(secret *corev1.Secret, projectID, bindingID string) bool {
	if secret == nil {
		return false
	}
	return secret.Labels["kube-nova.io/managed-by"] == "kube-nova" &&
		secret.Labels["kube-nova.io/devops-project-id"] == strings.TrimSpace(projectID) &&
		secret.Labels["kube-nova.io/devops-binding-id"] == strings.TrimSpace(bindingID)
}

func mergeStringMap(base, override map[string]string) map[string]string {
	result := make(map[string]string, len(base)+len(override))
	for key, value := range base {
		result[key] = value
	}
	for key, value := range override {
		result[key] = value
	}
	return result
}

func restConfigFromRequest(req devopstypes.Request) (*rest.Config, error) {
	if req.Credential != nil {
		switch strings.TrimSpace(req.Credential.Type) {
		case "kubeconfig":
			return restConfigFromKubeconfig(req.Credential.Kubeconfig, req.Channel.InsecureSkipTLS)
		case "secret_text":
			content := strings.TrimSpace(req.Credential.SecretText)
			if looksLikeKubeconfig(content) {
				return restConfigFromKubeconfig(content, req.Channel.InsecureSkipTLS)
			}
		}
	}
	endpoint, token := resolveEndpointAndToken(req)
	if endpoint == "" || strings.HasPrefix(endpoint, "system://") {
		return nil, fmt.Errorf("Kubernetes API 地址为空")
	}
	if token == "" {
		return nil, fmt.Errorf("Kubernetes 凭据未包含可用 token")
	}
	return &rest.Config{
		Host:        endpoint,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: req.Channel.InsecureSkipTLS,
		},
	}, nil
}

func restConfigFromKubeconfig(content string, insecureSkipTLS bool) (*rest.Config, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("Kubeconfig 凭据为空")
	}
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("解析 kubeconfig 失败: %w", err)
	}
	if insecureSkipTLS {
		config.TLSClientConfig.Insecure = true
	}
	return config, nil
}

func resolveEndpointAndToken(req devopstypes.Request) (string, string) {
	endpoint := strings.TrimSpace(req.Channel.Endpoint)
	token := strings.TrimSpace(req.Channel.Token)
	if req.Credential == nil {
		return endpoint, token
	}
	switch req.Credential.Type {
	case "token":
		token = strings.TrimSpace(req.Credential.Token)
	case "secret_text":
		content := strings.TrimSpace(req.Credential.SecretText)
		if looksLikeKubeconfig(content) {
			if parsed := kubeconfigValue(content, "server"); parsed != "" && (endpoint == "" || strings.HasPrefix(endpoint, "system://")) {
				endpoint = parsed
			}
			if token == "" {
				token = kubeconfigValue(content, "token")
			}
		} else if content != "" {
			token = content
		}
	case "kubeconfig":
		if parsed := kubeconfigValue(req.Credential.Kubeconfig, "server"); parsed != "" && (endpoint == "" || strings.HasPrefix(endpoint, "system://")) {
			endpoint = parsed
		}
		if token == "" {
			token = kubeconfigValue(req.Credential.Kubeconfig, "token")
		}
	}
	return endpoint, token
}

func looksLikeKubeconfig(content string) bool {
	content = strings.TrimSpace(content)
	return strings.Contains(content, "apiVersion:") && strings.Contains(content, "clusters:")
}

func kubeconfigValue(content, key string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, key+":") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, key+":"))
		return strings.Trim(value, `"'`)
	}
	return ""
}
