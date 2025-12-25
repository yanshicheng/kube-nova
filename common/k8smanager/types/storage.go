package types

// ==================== 存储配置相关 ====================

// StorageConfig 存储配置
type StorageConfig struct {
	Volumes              []VolumeConfig                `json:"volumes,omitempty"`              // 卷列表
	VolumeMounts         []VolumeMountConfig           `json:"volumeMounts,omitempty"`         // 卷挂载列表（按容器）
	VolumeClaimTemplates []PersistentVolumeClaimConfig `json:"volumeClaimTemplates,omitempty"` // PVC 模板（StatefulSet）
}

// VolumeConfig 卷配置
type VolumeConfig struct {
	Name                  string                   `json:"name"`
	Type                  string                   `json:"type"` // emptyDir, hostPath, configMap, secret, persistentVolumeClaim, nfs, etc.
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

// EmptyDirVolumeConfig EmptyDir 卷配置
type EmptyDirVolumeConfig struct {
	Medium    string `json:"medium,omitempty"`    // "", "Memory"
	SizeLimit string `json:"sizeLimit,omitempty"` // 如: "1Gi"
}

// HostPathVolumeConfig HostPath 卷配置
type HostPathVolumeConfig struct {
	Path string  `json:"path"`
	Type *string `json:"type,omitempty"` // DirectoryOrCreate, Directory, FileOrCreate, File, Socket, CharDevice, BlockDevice
}

// ConfigMapVolumeConfig ConfigMap 卷配置
type ConfigMapVolumeConfig struct {
	Name        string            `json:"name"`
	Items       []KeyToPathConfig `json:"items,omitempty"`
	DefaultMode *int32            `json:"defaultMode,omitempty"`
	Optional    *bool             `json:"optional,omitempty"`
}

// SecretVolumeConfig Secret 卷配置
type SecretVolumeConfig struct {
	SecretName  string            `json:"secretName"`
	Items       []KeyToPathConfig `json:"items,omitempty"`
	DefaultMode *int32            `json:"defaultMode,omitempty"`
	Optional    *bool             `json:"optional,omitempty"`
}

// KeyToPathConfig 键到路径映射
type KeyToPathConfig struct {
	Key  string `json:"key"`
	Path string `json:"path"`
	Mode *int32 `json:"mode,omitempty"`
}

// PVCVolumeConfig PVC 卷配置
type PVCVolumeConfig struct {
	ClaimName string `json:"claimName"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

// NFSVolumeConfig NFS 卷配置
type NFSVolumeConfig struct {
	Server   string `json:"server"`
	Path     string `json:"path"`
	ReadOnly bool   `json:"readOnly,omitempty"`
}

// DownwardAPIVolumeConfig DownwardAPI 卷配置
type DownwardAPIVolumeConfig struct {
	Items       []DownwardAPIVolumeFileConfig `json:"items,omitempty"`
	DefaultMode *int32                        `json:"defaultMode,omitempty"`
}

// DownwardAPIVolumeFileConfig DownwardAPI 文件配置
type DownwardAPIVolumeFileConfig struct {
	Path             string                 `json:"path"`
	FieldRef         *ObjectFieldSelector   `json:"fieldRef,omitempty"`
	ResourceFieldRef *ResourceFieldSelector `json:"resourceFieldRef,omitempty"`
	Mode             *int32                 `json:"mode,omitempty"`
}

// ProjectedVolumeConfig Projected 卷配置
type ProjectedVolumeConfig struct {
	Sources     []VolumeProjectionConfig `json:"sources"`
	DefaultMode *int32                   `json:"defaultMode,omitempty"`
}

// VolumeProjectionConfig 卷投射配置
type VolumeProjectionConfig struct {
	Secret              *SecretProjectionConfig              `json:"secret,omitempty"`
	ConfigMap           *ConfigMapProjectionConfig           `json:"configMap,omitempty"`
	DownwardAPI         *DownwardAPIProjectionConfig         `json:"downwardAPI,omitempty"`
	ServiceAccountToken *ServiceAccountTokenProjectionConfig `json:"serviceAccountToken,omitempty"`
}

// SecretProjectionConfig Secret 投射配置
type SecretProjectionConfig struct {
	Name     string            `json:"name"`
	Items    []KeyToPathConfig `json:"items,omitempty"`
	Optional *bool             `json:"optional,omitempty"`
}

// ConfigMapProjectionConfig ConfigMap 投射配置
type ConfigMapProjectionConfig struct {
	Name     string            `json:"name"`
	Items    []KeyToPathConfig `json:"items,omitempty"`
	Optional *bool             `json:"optional,omitempty"`
}

// DownwardAPIProjectionConfig DownwardAPI 投射配置
type DownwardAPIProjectionConfig struct {
	Items []DownwardAPIVolumeFileConfig `json:"items"`
}

// ServiceAccountTokenProjectionConfig ServiceAccountToken 投射配置
type ServiceAccountTokenProjectionConfig struct {
	Audience          string `json:"audience"`
	ExpirationSeconds *int64 `json:"expirationSeconds,omitempty"`
	Path              string `json:"path"`
}

// CSIVolumeConfig CSI 卷配置
type CSIVolumeConfig struct {
	Driver               string            `json:"driver"`
	ReadOnly             *bool             `json:"readOnly,omitempty"`
	FSType               *string           `json:"fsType,omitempty"`
	VolumeAttributes     map[string]string `json:"volumeAttributes,omitempty"`
	NodePublishSecretRef *SecretReference  `json:"nodePublishSecretRef,omitempty"`
}

// SecretReference Secret 引用
type SecretReference struct {
	Name string `json:"name"`
}

// VolumeMountConfig 卷挂载配置
type VolumeMountConfig struct {
	ContainerName string        `json:"containerName"` // 容器名称
	Mounts        []VolumeMount `json:"mounts"`        // 挂载列表
}

// VolumeMount 卷挂载
type VolumeMount struct {
	Name             string `json:"name"`
	MountPath        string `json:"mountPath"`
	SubPath          string `json:"subPath,omitempty"`
	SubPathExpr      string `json:"subPathExpr,omitempty"`
	ReadOnly         bool   `json:"readOnly,omitempty"`
	MountPropagation string `json:"mountPropagation,omitempty"` // None, HostToContainer, Bidirectional
}

// PersistentVolumeClaimConfig PVC 配置（StatefulSet 用）
type PersistentVolumeClaimConfig struct {
	Name             string                         `json:"name"`
	StorageClassName *string                        `json:"storageClassName,omitempty"`
	AccessModes      []string                       `json:"accessModes"` // ReadWriteOnce, ReadOnlyMany, ReadWriteMany
	Resources        PersistentVolumeClaimResources `json:"resources"`
	VolumeMode       *string                        `json:"volumeMode,omitempty"` // Filesystem, Block
	Selector         *LabelSelectorConfig           `json:"selector,omitempty"`
}

// PersistentVolumeClaimResources PVC 资源需求
type PersistentVolumeClaimResources struct {
	Requests StorageResourceList `json:"requests"`
	Limits   StorageResourceList `json:"limits,omitempty"`
}

// StorageResourceList 存储资源列表
type StorageResourceList struct {
	Storage string `json:"storage"` // 如: "10Gi"
}

// UpdateStorageConfigRequest 更新存储配置请求
type UpdateStorageConfigRequest struct {
	Volumes              []VolumeConfig                `json:"volumes,omitempty"`
	VolumeMounts         []VolumeMountConfig           `json:"volumeMounts,omitempty"`
	VolumeClaimTemplates []PersistentVolumeClaimConfig `json:"volumeClaimTemplates,omitempty"` // 仅 StatefulSet
}
