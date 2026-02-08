package types

// ============================================================================
// 通用类型定义
// ============================================================================

// ListRequest 定义列表请求参数
type ListRequest struct {
	Page     int    `json:"page" form:"page" binding:"min=1"`
	PageSize int    `json:"pageSize" form:"pageSize" binding:"min=1"`
	Search   string `json:"search" form:"search"`
	Labels   string `json:"labels" form:"labels"`
	SortBy   string `json:"sortBy" form:"sortBy"`
	SortDesc bool   `json:"sortDesc" form:"sortDesc"`
}

// ListResponse 定义列表响应
type ListResponse struct {
	Total      int `json:"total"`
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalPages int `json:"totalPages"`
}

// HTTPHeader HTTP 请求头
type HTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ============================================================================
// 容器类型枚举
// ============================================================================

// ContainerType 容器类型
type ContainerType string

const (
	ContainerTypeInit      ContainerType = "init"      // 初始化容器
	ContainerTypeMain      ContainerType = "main"      // 主容器
	ContainerTypeEphemeral ContainerType = "ephemeral" // 临时容器
)

// ContainerBasicInfo 容器基本信息（用于获取所有容器列表）
type ContainerBasicInfo struct {
	Name          string        `json:"name"`
	ContainerType ContainerType `json:"containerType"`
	Image         string        `json:"image"`
}

// ============================================================================
// 镜像管理相关
// ============================================================================

// ContainerImageInfo 单个容器的镜像信息
type ContainerImageInfo struct {
	ContainerName string        `json:"containerName"`
	ContainerType ContainerType `json:"containerType"` // init/main/ephemeral
	Image         string        `json:"image"`
}

// CommContainerInfoList 容器镜像列表（用于查询和批量更新）
type CommContainerInfoList struct {
	Containers []ContainerImageInfo `json:"containers"`
}

// UpdateImageRequest 更新单个镜像请求
type UpdateImageRequest struct {
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	ContainerName string `json:"containerName"`
	Image         string `json:"image"`
	Message       string `json:"message"`
	Reason        string `json:"reason,omitempty"`
}

// UpdateImagesRequest 批量更新镜像请求
type UpdateImagesRequest struct {
	Name       string                `json:"name"`
	Namespace  string                `json:"namespace"`
	Containers CommContainerInfoList `json:"containers"`
	Reason     string                `json:"reason,omitempty"`
}

// ============================================================================
// 环境变量相关
// ============================================================================

// EnvVarSource 环境变量值来源
type EnvVarSource struct {
	Type             string                 `json:"type"` // value, configMapKeyRef, secretKeyRef, fieldRef, resourceFieldRef
	Value            string                 `json:"value,omitempty"`
	ConfigMapKeyRef  *ConfigMapKeySelector  `json:"configMapKeyRef,omitempty"`
	SecretKeyRef     *SecretKeySelector     `json:"secretKeyRef,omitempty"`
	FieldRef         *ObjectFieldSelector   `json:"fieldRef,omitempty"`
	ResourceFieldRef *ResourceFieldSelector `json:"resourceFieldRef,omitempty"`
}

type ConfigMapKeySelector struct {
	Name     string `json:"name"`
	Key      string `json:"key"`
	Optional bool   `json:"optional"`
}

type SecretKeySelector struct {
	Name     string `json:"name"`
	Key      string `json:"key"`
	Optional bool   `json:"optional"`
}

type ObjectFieldSelector struct {
	FieldPath string `json:"fieldPath"`
}

type ResourceFieldSelector struct {
	ContainerName string `json:"containerName,omitempty"`
	Resource      string `json:"resource"`
	Divisor       string `json:"divisor,omitempty"`
}

// EnvVar 环境变量定义
type EnvVar struct {
	Name   string       `json:"name"`
	Source EnvVarSource `json:"source"`
}

// ContainerEnvVars 容器环境变量列表
type ContainerEnvVars struct {
	ContainerName string        `json:"containerName"`
	ContainerType ContainerType `json:"containerType"` // init/main/ephemeral
	Env           []EnvVar      `json:"env"`
}

// EnvVarsResponse 查询环境变量响应
type EnvVarsResponse struct {
	Containers []ContainerEnvVars `json:"containers"`
}

// UpdateEnvVarsRequest 修改环境变量请求（全量更新）
type UpdateEnvVarsRequest struct {
	Name       string             `json:"name"`
	Namespace  string             `json:"namespace"`
	Containers []ContainerEnvVars `json:"containers"`
}

// ============================================================================
// 资源配额相关
// ============================================================================

// ResourceList 资源列表
type ResourceList struct {
	Cpu    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// ResourceRequirements 资源需求
type ResourceRequirements struct {
	Limits   ResourceList `json:"limits,omitempty"`
	Requests ResourceList `json:"requests,omitempty"`
}

// ContainerResources 容器资源配额
type ContainerResources struct {
	ContainerName string               `json:"containerName"`
	ContainerType ContainerType        `json:"containerType"` // init/main/ephemeral
	Resources     ResourceRequirements `json:"resources"`
}

// ResourcesResponse 查询资源配额响应
type ResourcesResponse struct {
	Containers []ContainerResources `json:"containers"`
}

// UpdateResourcesRequest 修改资源配额请求（全量更新）
type UpdateResourcesRequest struct {
	Name       string               `json:"name"`
	Namespace  string               `json:"namespace"`
	Containers []ContainerResources `json:"containers"`
}

// ============================================================================
// 健康检查相关
// ============================================================================

// ExecAction 执行命令动作
type ExecAction struct {
	Command []string `json:"command"`
}

// HTTPGetAction HTTP GET 请求动作
type HTTPGetAction struct {
	Path        string       `json:"path"`
	Port        int32        `json:"port"`
	Host        string       `json:"host,omitempty"`
	Scheme      string       `json:"scheme,omitempty"`
	HTTPHeaders []HTTPHeader `json:"httpHeaders,omitempty"`
}

// TCPSocketAction TCP 套接字检查动作
type TCPSocketAction struct {
	Port int32  `json:"port"`
	Host string `json:"host,omitempty"`
}

// GRPCAction gRPC 健康检查
type GRPCAction struct {
	Port    int32  `json:"port"`
	Service string `json:"service,omitempty"`
}

// Probe 健康检查探针
type Probe struct {
	Type                string           `json:"type"` // httpGet, tcpSocket, exec, grpc
	HttpGet             *HTTPGetAction   `json:"httpGet,omitempty"`
	TcpSocket           *TCPSocketAction `json:"tcpSocket,omitempty"`
	Exec                *ExecAction      `json:"exec,omitempty"`
	Grpc                *GRPCAction      `json:"grpc,omitempty"`
	InitialDelaySeconds int32            `json:"initialDelaySeconds,omitempty"`
	TimeoutSeconds      int32            `json:"timeoutSeconds,omitempty"`
	PeriodSeconds       int32            `json:"periodSeconds,omitempty"`
	SuccessThreshold    int32            `json:"successThreshold,omitempty"`
	FailureThreshold    int32            `json:"failureThreshold,omitempty"`
}

// ContainerProbes 容器健康检查
type ContainerProbes struct {
	ContainerName  string        `json:"containerName"`
	ContainerType  ContainerType `json:"containerType"` // init/main/ephemeral
	LivenessProbe  *Probe        `json:"livenessProbe,omitempty"`
	ReadinessProbe *Probe        `json:"readinessProbe,omitempty"`
	StartupProbe   *Probe        `json:"startupProbe,omitempty"`
}

// ProbesResponse 查询健康检查响应
type ProbesResponse struct {
	Containers []ContainerProbes `json:"containers"`
}

// UpdateProbesRequest 修改健康检查请求（全量更新）
type UpdateProbesRequest struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Containers []ContainerProbes `json:"containers"`
}

// ============================================================================
// 生命周期钩子相关
// ============================================================================

// SleepAction 休眠动作（K8s 1.29+ 支持）
type SleepAction struct {
	Seconds int64 `json:"seconds"`
}

// LifecycleHandler 生命周期钩子处理器
// 支持四种类型：exec、httpGet、tcpSocket、sleep
// 注意：每个 handler 只能指定一种类型
type LifecycleHandler struct {
	Type      string           `json:"type"` // exec, httpGet, tcpSocket, sleep
	Exec      *ExecAction      `json:"exec,omitempty"`
	HTTPGet   *HTTPGetAction   `json:"httpGet,omitempty"`
	TCPSocket *TCPSocketAction `json:"tcpSocket,omitempty"`
	Sleep     *SleepAction     `json:"sleep,omitempty"`
}

// Lifecycle 容器生命周期钩子配置
type Lifecycle struct {
	PostStart *LifecycleHandler `json:"postStart,omitempty"` // 启动后钩子
	PreStop   *LifecycleHandler `json:"preStop,omitempty"`   // 停止前钩子
}

// ============================================================================
// 安全上下文相关
// ============================================================================

// Capabilities Linux 能力配置
type Capabilities struct {
	Add  []string `json:"add,omitempty"`
	Drop []string `json:"drop,omitempty"`
}

// SeccompProfile Seccomp 安全计算模式配置
type SeccompProfile struct {
	Type             string `json:"type"` // RuntimeDefault, Unconfined, Localhost
	LocalhostProfile string `json:"localhostProfile,omitempty"`
}

// SELinuxOptions SELinux 安全选项
type SELinuxOptions struct {
	User  string `json:"user,omitempty"`
	Role  string `json:"role,omitempty"`
	Type  string `json:"type,omitempty"`
	Level string `json:"level,omitempty"`
}

// AppArmorProfile AppArmor 配置（K8s 1.30+）
type AppArmorProfile struct {
	Type             string `json:"type"` // RuntimeDefault, Unconfined, Localhost
	LocalhostProfile string `json:"localhostProfile,omitempty"`
}

// ContainerSecurityContext 容器级别安全上下文
type ContainerSecurityContext struct {
	RunAsUser                *int64           `json:"runAsUser,omitempty"`
	RunAsGroup               *int64           `json:"runAsGroup,omitempty"`
	RunAsNonRoot             *bool            `json:"runAsNonRoot,omitempty"`
	ReadOnlyRootFilesystem   *bool            `json:"readOnlyRootFilesystem,omitempty"`
	Privileged               *bool            `json:"privileged,omitempty"`
	AllowPrivilegeEscalation *bool            `json:"allowPrivilegeEscalation,omitempty"`
	ProcMount                string           `json:"procMount,omitempty"`
	Capabilities             *Capabilities    `json:"capabilities,omitempty"`
	SeccompProfile           *SeccompProfile  `json:"seccompProfile,omitempty"`
	SELinuxOptions           *SELinuxOptions  `json:"seLinuxOptions,omitempty"`
	AppArmorProfile          *AppArmorProfile `json:"appArmorProfile,omitempty"`
}

// Sysctl 内核参数配置
type Sysctl struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// WindowsSecurityContextOptions Windows 容器安全选项
type WindowsSecurityContextOptions struct {
	GMSACredentialSpecName string `json:"gmsaCredentialSpecName,omitempty"`
	GMSACredentialSpec     string `json:"gmsaCredentialSpec,omitempty"`
	RunAsUserName          string `json:"runAsUserName,omitempty"`
	HostProcess            *bool  `json:"hostProcess,omitempty"`
}

// PodSecurityContext Pod 级别安全上下文
type PodSecurityContext struct {
	RunAsUser           *int64                         `json:"runAsUser,omitempty"`
	RunAsGroup          *int64                         `json:"runAsGroup,omitempty"`
	RunAsNonRoot        *bool                          `json:"runAsNonRoot,omitempty"`
	SupplementalGroups  []int64                        `json:"supplementalGroups,omitempty"`
	FSGroup             *int64                         `json:"fsGroup,omitempty"`
	FSGroupChangePolicy string                         `json:"fsGroupChangePolicy,omitempty"`
	SeccompProfile      *SeccompProfile                `json:"seccompProfile,omitempty"`
	SELinuxOptions      *SELinuxOptions                `json:"seLinuxOptions,omitempty"`
	AppArmorProfile     *AppArmorProfile               `json:"appArmorProfile,omitempty"`
	Sysctls             []Sysctl                       `json:"sysctls,omitempty"`
	WindowsOptions      *WindowsSecurityContextOptions `json:"windowsOptions,omitempty"`
}

// ============================================================================
// DNS 配置相关
// ============================================================================

// DNSConfigOption DNS 配置选项
type DNSConfigOption struct {
	Name  string  `json:"name"`
	Value *string `json:"value,omitempty"`
}

// DNSConfig 自定义 DNS 配置
type DNSConfig struct {
	Nameservers []string          `json:"nameservers,omitempty"`
	Searches    []string          `json:"searches,omitempty"`
	Options     []DNSConfigOption `json:"options,omitempty"`
}

// ============================================================================
// 调度配置相关
// ============================================================================

// NodeSelectorTerm 节点选择器条件
type NodeSelectorTerm struct {
	MatchExpressions []NodeSelectorRequirement `json:"matchExpressions,omitempty"`
	MatchFields      []NodeSelectorRequirement `json:"matchFields,omitempty"`
}

// NodeSelectorRequirement 节点选择器要求
type NodeSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"` // In, NotIn, Exists, DoesNotExist, Gt, Lt
	Values   []string `json:"values,omitempty"`
}

// PreferredSchedulingTerm 偏好调度条件
type PreferredSchedulingTerm struct {
	Weight     int32            `json:"weight"` // 1-100
	Preference NodeSelectorTerm `json:"preference"`
}

// NodeAffinity 节点亲和性配置
type NodeAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  *NodeSelector             `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// NodeSelector 节点选择器
type NodeSelector struct {
	NodeSelectorTerms []NodeSelectorTerm `json:"nodeSelectorTerms"`
}

// LabelSelectorRequirement 标签选择器要求
type LabelSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"` // In, NotIn, Exists, DoesNotExist
	Values   []string `json:"values,omitempty"`
}

// CommLabelSelectorConfig 标签选择器配置
type CommLabelSelectorConfig struct {
	MatchLabels      map[string]string          `json:"matchLabels,omitempty"`
	MatchExpressions []LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

// PodAffinityTerm Pod 亲和性条件
type PodAffinityTerm struct {
	LabelSelector     *CommLabelSelectorConfig `json:"labelSelector,omitempty"`
	Namespaces        []string                 `json:"namespaces,omitempty"`
	TopologyKey       string                   `json:"topologyKey"`
	NamespaceSelector *CommLabelSelectorConfig `json:"namespaceSelector,omitempty"`
}

// WeightedPodAffinityTerm 带权重的 Pod 亲和性条件
type WeightedPodAffinityTerm struct {
	Weight          int32           `json:"weight"` // 1-100
	PodAffinityTerm PodAffinityTerm `json:"podAffinityTerm"`
}

// PodAffinity Pod 亲和性配置
type PodAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// PodAntiAffinity Pod 反亲和性配置
type PodAntiAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// Toleration 容忍配置
type Toleration struct {
	Key               string `json:"key,omitempty"`
	Operator          string `json:"operator,omitempty"` // Equal, Exists
	Value             string `json:"value,omitempty"`
	Effect            string `json:"effect,omitempty"` // NoSchedule, PreferNoSchedule, NoExecute
	TolerationSeconds *int64 `json:"tolerationSeconds,omitempty"`
}

// TopologySpreadConstraint 拓扑分布约束
type TopologySpreadConstraint struct {
	MaxSkew            int32                    `json:"maxSkew"`
	TopologyKey        string                   `json:"topologyKey"`
	WhenUnsatisfiable  string                   `json:"whenUnsatisfiable"` // DoNotSchedule, ScheduleAnyway
	LabelSelector      *CommLabelSelectorConfig `json:"labelSelector,omitempty"`
	MinDomains         *int32                   `json:"minDomains,omitempty"`
	NodeAffinityPolicy *string                  `json:"nodeAffinityPolicy,omitempty"` // Honor, Ignore
	NodeTaintsPolicy   *string                  `json:"nodeTaintsPolicy,omitempty"`   // Honor, Ignore
	MatchLabelKeys     []string                 `json:"matchLabelKeys,omitempty"`
}

// CommAffinityConfig 亲和性配置
type CommAffinityConfig struct {
	NodeAffinity    *NodeAffinity    `json:"nodeAffinity,omitempty"`
	PodAffinity     *PodAffinity     `json:"podAffinity,omitempty"`
	PodAntiAffinity *PodAntiAffinity `json:"podAntiAffinity,omitempty"`
}

// CommSchedulingConfig 调度配置（Pod 级别）
type CommSchedulingConfig struct {
	NodeSelector              map[string]string          `json:"nodeSelector,omitempty"`
	NodeName                  string                     `json:"nodeName,omitempty"`
	Affinity                  *CommAffinityConfig        `json:"affinity,omitempty"`
	Tolerations               []Toleration               `json:"tolerations,omitempty"`
	TopologySpreadConstraints []TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// CommUpdateSchedulingConfigRequest 更新调度配置请求
type CommUpdateSchedulingConfigRequest struct {
	Name                      string                     `json:"name"`
	Namespace                 string                     `json:"namespace"`
	NodeSelector              map[string]string          `json:"nodeSelector,omitempty"`
	NodeName                  string                     `json:"nodeName,omitempty"`
	Affinity                  *CommAffinityConfig        `json:"affinity,omitempty"`
	Tolerations               []Toleration               `json:"tolerations,omitempty"`
	TopologySpreadConstraints []TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// ============================================================================
// 高级容器配置相关
// ============================================================================

// ContainerAdvancedConfig 单个容器的高级配置
type ContainerAdvancedConfig struct {
	ContainerName string        `json:"containerName"`
	ContainerType ContainerType `json:"containerType,omitempty"` // init/main/ephemeral

	// 生命周期钩子
	Lifecycle *Lifecycle `json:"lifecycle,omitempty"`

	// 镜像与启动配置
	ImagePullPolicy string   `json:"imagePullPolicy,omitempty"` // Always, IfNotPresent, Never
	Command         []string `json:"command,omitempty"`
	Args            []string `json:"args,omitempty"`
	WorkingDir      string   `json:"workingDir,omitempty"`

	// 终端配置
	Stdin     *bool `json:"stdin,omitempty"`
	StdinOnce *bool `json:"stdinOnce,omitempty"`
	TTY       *bool `json:"tty,omitempty"`

	// 安全上下文
	SecurityContext *ContainerSecurityContext `json:"securityContext,omitempty"`
}

// HostAlias 主机别名配置（/etc/hosts 条目）
type HostAlias struct {
	IP        string   `json:"ip"`
	Hostnames []string `json:"hostnames"`
}

// PodOS 指定 Pod 的操作系统
type PodOS struct {
	Name string `json:"name"` // linux 或 windows
}

// PodAdvancedConfig Pod 级别的高级配置
type PodAdvancedConfig struct {
	// 优雅终止
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// 重启策略
	RestartPolicy string `json:"restartPolicy,omitempty"` // Always, OnFailure, Never

	// DNS 配置
	DNSPolicy string     `json:"dnsPolicy,omitempty"`
	DNSConfig *DNSConfig `json:"dnsConfig,omitempty"`

	// 主机命名空间共享
	HostNetwork *bool  `json:"hostNetwork,omitempty"`
	HostPID     *bool  `json:"hostPID,omitempty"`
	HostIPC     *bool  `json:"hostIPC,omitempty"`
	HostUsers   *bool  `json:"hostUsers,omitempty"`
	Hostname    string `json:"hostname,omitempty"`
	Subdomain   string `json:"subdomain,omitempty"`

	// /etc/hosts 配置
	HostAliases []HostAlias `json:"hostAliases,omitempty"`

	// 服务账户
	ServiceAccountName           string `json:"serviceAccountName,omitempty"`
	AutomountServiceAccountToken *bool  `json:"automountServiceAccountToken,omitempty"`

	// 镜像拉取
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`

	// 调度优先级
	PriorityClassName string `json:"priorityClassName,omitempty"`
	Priority          *int32 `json:"priority,omitempty"`
	PreemptionPolicy  string `json:"preemptionPolicy,omitempty"`

	// 容器运行时
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`

	// 安全上下文
	SecurityContext *PodSecurityContext `json:"securityContext,omitempty"`

	// 共享进程命名空间
	ShareProcessNamespace *bool `json:"shareProcessNamespace,omitempty"`

	// 操作系统
	OS *PodOS `json:"os,omitempty"`

	// 调度器
	SchedulerName string `json:"schedulerName,omitempty"`

	// 激活截止时间
	ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"`
}

// AdvancedConfigResponse 高级配置查询响应
type AdvancedConfigResponse struct {
	PodConfig  PodAdvancedConfig         `json:"podConfig"`
	Containers []ContainerAdvancedConfig `json:"containers"` // init + main + ephemeral
}

// UpdateAdvancedConfigRequest 高级配置更新请求
type UpdateAdvancedConfigRequest struct {
	Name       string                    `json:"name"`
	Namespace  string                    `json:"namespace"`
	PodConfig  *PodAdvancedConfig        `json:"podConfig,omitempty"`
	Containers []ContainerAdvancedConfig `json:"containers,omitempty"`
}

// ============================================================================
// 存储配置相关
// ============================================================================

// VolumeConfig 卷配置
type VolumeConfig struct {
	Name                  string                   `json:"name"`
	Type                  string                   `json:"type"`
	EmptyDir              *EmptyDirVolumeConfig    `json:"emptyDir,omitempty"`
	HostPath              *HostPathVolumeConfig    `json:"hostPath,omitempty"`
	ConfigMap             *ConfigMapVolumeConfig   `json:"configMap,omitempty"`
	Secret                *SecretVolumeConfig      `json:"secret,omitempty"`
	PersistentVolumeClaim *PVCVolumeConfig         `json:"persistentVolumeClaim,omitempty"`
	NFS                   *NFSVolumeConfig         `json:"nfs,omitempty"`
	DownwardAPI           *DownwardAPIVolumeConfig `json:"downwardAPI,omitempty"`
	Projected             *ProjectedVolumeConfig   `json:"projected,omitempty"`
	CSI                   *CSIVolumeConfig         `json:"csi,omitempty"`
}

type EmptyDirVolumeConfig struct {
	Medium    string `json:"medium,omitempty"`
	SizeLimit string `json:"sizeLimit,omitempty"`
}

type HostPathVolumeConfig struct {
	Path string  `json:"path"`
	Type *string `json:"type,omitempty"`
}

type ConfigMapVolumeConfig struct {
	Name        string            `json:"name"`
	Items       []KeyToPathConfig `json:"items,omitempty"`
	DefaultMode *int32            `json:"defaultMode,omitempty"`
	Optional    *bool             `json:"optional,omitempty"`
}

type SecretVolumeConfig struct {
	SecretName  string            `json:"secretName"`
	Items       []KeyToPathConfig `json:"items,omitempty"`
	DefaultMode *int32            `json:"defaultMode,omitempty"`
	Optional    *bool             `json:"optional,omitempty"`
}

type KeyToPathConfig struct {
	Key  string `json:"key"`
	Path string `json:"path"`
	Mode *int32 `json:"mode,omitempty"`
}

type PVCVolumeConfig struct {
	ClaimName string `json:"claimName"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

type NFSVolumeConfig struct {
	Server   string `json:"server"`
	Path     string `json:"path"`
	ReadOnly bool   `json:"readOnly,omitempty"`
}

type DownwardAPIVolumeConfig struct {
	Items       []DownwardAPIVolumeFileConfig `json:"items,omitempty"`
	DefaultMode *int32                        `json:"defaultMode,omitempty"`
}

type DownwardAPIVolumeFileConfig struct {
	Path             string                 `json:"path"`
	FieldRef         *ObjectFieldSelector   `json:"fieldRef,omitempty"`
	ResourceFieldRef *ResourceFieldSelector `json:"resourceFieldRef,omitempty"`
	Mode             *int32                 `json:"mode,omitempty"`
}

type ProjectedVolumeConfig struct {
	Sources     []VolumeProjectionConfig `json:"sources"`
	DefaultMode *int32                   `json:"defaultMode,omitempty"`
}

type VolumeProjectionConfig struct {
	Secret              *SecretProjectionConfig              `json:"secret,omitempty"`
	ConfigMap           *ConfigMapProjectionConfig           `json:"configMap,omitempty"`
	DownwardAPI         *DownwardAPIProjectionConfig         `json:"downwardAPI,omitempty"`
	ServiceAccountToken *ServiceAccountTokenProjectionConfig `json:"serviceAccountToken,omitempty"`
}

type SecretProjectionConfig struct {
	Name     string            `json:"name"`
	Items    []KeyToPathConfig `json:"items,omitempty"`
	Optional *bool             `json:"optional,omitempty"`
}

type ConfigMapProjectionConfig struct {
	Name     string            `json:"name"`
	Items    []KeyToPathConfig `json:"items,omitempty"`
	Optional *bool             `json:"optional,omitempty"`
}

type DownwardAPIProjectionConfig struct {
	Items []DownwardAPIVolumeFileConfig `json:"items"`
}

type ServiceAccountTokenProjectionConfig struct {
	Audience          string `json:"audience"`
	ExpirationSeconds *int64 `json:"expirationSeconds,omitempty"`
	Path              string `json:"path"`
}

type CSIVolumeConfig struct {
	Driver               string            `json:"driver"`
	ReadOnly             *bool             `json:"readOnly,omitempty"`
	FSType               *string           `json:"fsType,omitempty"`
	VolumeAttributes     map[string]string `json:"volumeAttributes,omitempty"`
	NodePublishSecretRef *SecretReference  `json:"nodePublishSecretRef,omitempty"`
}

type SecretReference struct {
	Name string `json:"name"`
}

// VolumeMountConfig 容器卷挂载配置
type VolumeMountConfig struct {
	ContainerName string        `json:"containerName"`
	ContainerType ContainerType `json:"containerType"` // init/main/ephemeral
	Mounts        []VolumeMount `json:"mounts"`
}

// VolumeMount 卷挂载
type VolumeMount struct {
	Name             string `json:"name"`
	MountPath        string `json:"mountPath"`
	SubPath          string `json:"subPath,omitempty"`
	SubPathExpr      string `json:"subPathExpr,omitempty"`
	ReadOnly         bool   `json:"readOnly,omitempty"`
	MountPropagation string `json:"mountPropagation,omitempty"`
}

// PersistentVolumeClaimConfig PVC 配置（StatefulSet 用）
type PersistentVolumeClaimConfig struct {
	Name             string                         `json:"name"`
	StorageClassName *string                        `json:"storageClassName,omitempty"`
	AccessModes      []string                       `json:"accessModes"`
	Resources        PersistentVolumeClaimResources `json:"resources"`
	VolumeMode       *string                        `json:"volumeMode,omitempty"`
	Selector         *CommLabelSelectorConfig       `json:"selector,omitempty"`
}

type PersistentVolumeClaimResources struct {
	Requests StorageResourceList `json:"requests"`
	Limits   StorageResourceList `json:"limits,omitempty"`
}

type StorageResourceList struct {
	Storage string `json:"storage"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Volumes              []VolumeConfig                `json:"volumes"`
	VolumeMounts         []VolumeMountConfig           `json:"volumeMounts"`
	VolumeClaimTemplates []PersistentVolumeClaimConfig `json:"volumeClaimTemplates"`
}

// UpdateStorageConfigRequest 更新存储配置请求
type UpdateStorageConfigRequest struct {
	Name                 string                        `json:"name"`
	Namespace            string                        `json:"namespace"`
	Volumes              []VolumeConfig                `json:"volumes"`
	VolumeMounts         []VolumeMountConfig           `json:"volumeMounts"`
	VolumeClaimTemplates []PersistentVolumeClaimConfig `json:"volumeClaimTemplates,omitempty"`
}

// ============================================================================
// 更新策略相关
// ============================================================================

// RollingUpdateConfig 滚动更新配置
type RollingUpdateConfig struct {
	MaxUnavailable string `json:"maxUnavailable,omitempty"`
	MaxSurge       string `json:"maxSurge,omitempty"`
	Partition      int32  `json:"partition,omitempty"`
}

// UpdateStrategyResponse 更新策略响应
type UpdateStrategyResponse struct {
	Type          string               `json:"type"`
	RollingUpdate *RollingUpdateConfig `json:"rollingUpdate,omitempty"`
}

// UpdateStrategyRequest 修改更新策略请求
type UpdateStrategyRequest struct {
	Name          string               `json:"name"`
	Namespace     string               `json:"namespace"`
	Type          string               `json:"type"`
	RollingUpdate *RollingUpdateConfig `json:"rollingUpdate,omitempty"`
}

// ============================================================================
// 暂停/恢复相关
// ============================================================================

// PauseStatusResponse 暂停状态响应
type PauseStatusResponse struct {
	Paused      bool   `json:"paused"`
	SupportType string `json:"supportType"`
}

// ============================================================================
// 版本历史相关
// ============================================================================

// RevisionInfo K8s 原生版本历史
type RevisionInfo struct {
	Revision          int64    `json:"revision"`
	CreationTimestamp int64    `json:"creationTimestamp"`
	Images            []string `json:"images"`
	Replicas          int32    `json:"replicas"`
	Reason            string   `json:"reason"`
}

// ConfigHistoryInfo 配置历史记录（业务层）
type ConfigHistoryInfo struct {
	Id          uint64   `json:"id"`
	Revision    int32    `json:"revision"`
	Images      []string `json:"images"`
	CreatedAt   int64    `json:"createdAt"`
	UpdatedBy   string   `json:"updatedBy"`
	Reason      string   `json:"reason"`
	SpecPreview string   `json:"specPreview"`
	IsCurrent   bool     `json:"isCurrent"`
}

// RollbackToRevisionRequest 回滚到指定版本请求
type RollbackToRevisionRequest struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Revision  int64  `json:"revision"`
}

// RollbackToConfigRequest 回滚到历史配置请求
type RollbackToConfigRequest struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	ConfigHistoryId uint64 `json:"configHistoryId"`
}

// ============================================================================
// 事件相关
// ============================================================================

// CommEventInfo 事件信息
type CommEventInfo struct {
	Type           string `json:"type"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	Count          int32  `json:"count"`
	FirstTimestamp int64  `json:"firstTimestamp"`
	LastTimestamp  int64  `json:"lastTimestamp"`
	Source         string `json:"source"`
}

// ListEventResponse 事件列表响应
type ListEventResponse struct {
	Items []CommEventInfo `json:"items"`
}

// ============================================================================
// Pod 相关
// ============================================================================

// PodInfo Pod 基本信息
type PodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Ready     string `json:"ready"`
	Restarts  int32  `json:"restarts"`
	Age       string `json:"age"`
	Node      string `json:"node"`
	PodIP     string `json:"podIP"`
}

// ContainerInfoList 容器信息列表
type ContainerInfoList struct {
	InitContainers      []ContainerInfo `json:"initContainers"`
	Containers          []ContainerInfo `json:"containers"`
	EphemeralContainers []ContainerInfo `json:"ephemeralContainers,omitempty"`
}

// ContainerInfo 容器信息
type ContainerInfo struct {
	Name         string                  `json:"name"`
	Image        string                  `json:"image"`
	Ready        bool                    `json:"ready"`
	RestartCount int32                   `json:"restartCount"`
	State        string                  `json:"state"` // Running, Waiting, Terminated
	Status       *ContainerStatusDetails `json:"status,omitempty"`
}

// ContainerStatusDetails 容器状态详情（简化版本）- 现在可以安全使用 ContainerStateInfo
type ContainerStatusDetails struct {
	State        ContainerStateInfo `json:"state"`
	LastState    ContainerStateInfo `json:"lastState,omitempty"`
	Ready        bool               `json:"ready"`
	RestartCount int32              `json:"restartCount"`
	Image        string             `json:"image"`
	ImageID      string             `json:"imageID"`
	ContainerID  string             `json:"containerID,omitempty"`
	Started      *bool              `json:"started,omitempty"`
}
