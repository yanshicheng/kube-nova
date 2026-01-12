package types

import (
	"time"
)

// HarborConfig Harbor 连接配置
type HarborConfig struct {
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"` // https://harbor.example.com
	Username string `json:"username"`
	Password string `json:"password"`
	Insecure bool   `json:"insecure"`
	CACert   string `json:"ca_cert"` // PEM 格式证书
}

// ListRequest 通用列表请求
type ListRequest struct {
	Search   string // 模糊搜索关键词
	Labels   string // 标签选择器
	Page     int64  // 页码，从1开始
	PageSize int64  // 每页数量
	SortBy   string // 排序字段
	SortDesc bool   // 是否降序
}

// ListResponse 通用列表响应
type ListResponse struct {
	Total      int   `json:"total"`
	Page       int64 `json:"page"`
	PageSize   int64 `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

// SystemInfo 系统信息
type SystemInfo struct {
	// 存储信息
	StorageTotal int64 `json:"storage_total"` // 存储总量
	StorageUsed  int64 `json:"storage_used"`  // 已使用
	StorageFree  int64 `json:"storage_free"`  // 剩余

	// 项目统计
	TotalProjects   int64 `json:"total_projects"`   // 项目总数
	PublicProjects  int64 `json:"public_projects"`  // 公开项目数
	PrivateProjects int64 `json:"private_projects"` // 私有项目数

	// 仓库统计
	TotalRepositories   int64 `json:"total_repositories"`   // 仓库总数
	PublicRepositories  int64 `json:"public_repositories"`  // 公开仓库数
	PrivateRepositories int64 `json:"private_repositories"` // 私有仓库数
}

// Project 项目
type Project struct {
	ProjectID    int64             `json:"project_id"`
	Name         string            `json:"name"`
	OwnerName    string            `json:"owner_name"`
	Public       bool              `json:"public"`
	RepoCount    int64             `json:"repo_count"`
	CreationTime time.Time         `json:"creation_time"`
	UpdateTime   time.Time         `json:"update_time"`
	Metadata     map[string]string `json:"metadata"`
	// 新增：存储信息
	StorageLimit        int64  `json:"storage_limit"`         // 存储配额限制（字节）
	StorageUsed         int64  `json:"storage_used"`          // 已使用存储（字节）
	StorageLimitDisplay string `json:"storage_limit_display"` // 存储配额显示（如 "10GB"）
	StorageUsedDisplay  string `json:"storage_used_display"`  // 已使用存储显示（如 "5.2GB"）
}

// ProjectReq 项目请求
type ProjectReq struct {
	ProjectName  string            `json:"project_name"`
	Public       bool              `json:"public"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	StorageLimit int64             `json:"storage_limit,omitempty"` // -1 无限制
}

// ListProjectResponse 项目列表响应
type ListProjectResponse struct {
	ListResponse
	Items []Project `json:"items"`
}

// Repository 仓库
type Repository struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	ProjectID     int64     `json:"project_id"`
	Description   string    `json:"description"`
	ArtifactCount int64     `json:"artifact_count"`
	PullCount     int64     `json:"pull_count"`
	CreationTime  time.Time `json:"creation_time"`
	UpdateTime    time.Time `json:"update_time"`
}

// ListRepositoryResponse 仓库列表响应
type ListRepositoryResponse struct {
	ListResponse
	Items []Repository `json:"items"`
}

// Artifact 制品
type Artifact struct {
	ID                int64     `json:"id"`
	Type              string    `json:"type"`
	Digest            string    `json:"digest"`
	Tags              []Tag     `json:"tags"`
	PushTime          time.Time `json:"push_time"`
	PullTime          time.Time `json:"pull_time"`
	Size              int64     `json:"size"`
	ManifestMediaType string    `json:"manifest_media_type"`
}

// ListArtifactResponse 制品列表响应
type ListArtifactResponse struct {
	ListResponse
	Items []Artifact `json:"items"`
}

// Tag 标签
type Tag struct {
	ID           int64     `json:"id"`
	RepositoryID int64     `json:"repository_id"`
	ArtifactID   int64     `json:"artifact_id"`
	Name         string    `json:"name"`
	PushTime     time.Time `json:"push_time"`
	PullTime     time.Time `json:"pull_time"`
	Immutable    bool      `json:"immutable"`
	Signed       bool      `json:"signed"`
}

// Quota 配额
type Quota struct {
	ID        int64        `json:"id"`
	Ref       *QuotaRef    `json:"ref,omitempty"`
	Hard      ResourceList `json:"hard"`
	Used      ResourceList `json:"used"`
	CreatedAt time.Time    `json:"creation_time"`
	UpdatedAt time.Time    `json:"update_time"`
}

// QuotaRef 配额引用信息
type QuotaRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ResourceList struct {
	Storage int64 `json:"storage"`
	Count   int64 `json:"count"`
}

// ProjectMember 项目成员
type ProjectMember struct {
	ID           int64     `json:"id"`
	ProjectID    int64     `json:"project_id"`
	EntityName   string    `json:"entity_name"` // 用户名或用户组名
	EntityType   string    `json:"entity_type"` // u=用户, g=用户组
	RoleID       int64     `json:"role_id"`     // 角色ID: 1=管理员, 2=开发者, 3=访客, 4=维护者
	RoleName     string    `json:"role_name"`   // 角色名称
	CreationTime time.Time `json:"creation_time"`
	UpdateTime   time.Time `json:"update_time"`
}

// ProjectMemberReq 项目成员请求
type ProjectMemberReq struct {
	MemberUser string `json:"member_user"` // 用户名
	RoleID     int64  `json:"role_id"`     // 角色ID
}

// ListProjectMemberResponse 项目成员列表响应
type ListProjectMemberResponse struct {
	ListResponse
	Items []ProjectMember `json:"items"`
}

// ImageSearchResult 镜像搜索结果
type ImageSearchResult struct {
	Name       string                   `json:"name"`       // Harbor 名称
	Endpoint   string                   `json:"endpoint"`   // Harbor 地址
	UUID       string                   `json:"uuid"`       // Harbor UUID
	Repository []RepositorySearchResult `json:"repository"` // 匹配的仓库列表
}

// RepositorySearchResult 仓库搜索结果
type RepositorySearchResult struct {
	ProjectName string   `json:"project_name"` // 项目名称
	RepoName    string   `json:"repo_name"`    // 仓库名称
	Tags        []string `json:"tags"`         // 标签列表
}

// UploadCredential 镜像上传凭证
type UploadCredential struct {
	RegistryUrl        string `json:"registry_url"`         // 镜像仓库地址
	Username           string `json:"username"`             // 用户名
	Password           string `json:"password"`             // 密码
	FullImagePath      string `json:"full_image_path"`      // 完整镜像路径前缀
	DockerLoginCommand string `json:"docker_login_command"` // docker login 命令示例
	DockerPushCommand  string `json:"docker_push_command"`  // docker push 命令示例
}

// PullCommand 镜像拉取命令
type PullCommand struct {
	RegistryUrl        string `json:"registry_url"`         // 镜像仓库地址
	Username           string `json:"username"`             // 用户名
	Password           string `json:"password"`             // 密码
	FullImageUrl       string `json:"full_image_url"`       // 完整镜像 URL
	DockerLoginCommand string `json:"docker_login_command"` // docker login 命令
	DockerPullCommand  string `json:"docker_pull_command"`  // docker pull 命令
}

// ==================== 垃圾回收相关 ====================

// GCHistory GC 历史记录
type GCHistory struct {
	ID             int64     `json:"id"`
	JobName        string    `json:"job_name"`
	JobKind        string    `json:"job_kind"`
	JobStatus      string    `json:"job_status"` // Pending, Running, Success, Error, Stopped
	Schedule       *Schedule `json:"schedule,omitempty"`
	JobParameters  string    `json:"job_parameters"`
	CreationTime   time.Time `json:"creation_time"`
	UpdateTime     time.Time `json:"update_time"`
	DeleteUntagged bool      `json:"delete_untagged"`
}

// Schedule 调度配置
type Schedule struct {
	Type              string    `json:"type"` // Manual, Scheduled, Event
	Cron              string    `json:"cron"` // cron 表达式
	NextScheduledTime time.Time `json:"next_scheduled_time,omitempty"`
}

// GCSchedule GC 调度配置
type GCSchedule struct {
	Schedule   *Schedule         `json:"schedule"`
	Parameters map[string]string `json:"parameters"`
}

// ListGCHistoryResponse GC 历史列表响应
type ListGCHistoryResponse struct {
	ListResponse
	Items []GCHistory `json:"items"`
}

// ==================== 保留策略相关 ====================

// RetentionSelector 保留策略选择器
type RetentionSelector struct {
	Kind       string `json:"kind"`       // doublestar, label
	Decoration string `json:"decoration"` // matches, excludes
	Pattern    string `json:"pattern"`    // 匹配模式
	Extras     string `json:"extras"`     // 额外参数（JSON 字符串）
}

// RetentionRule 保留规则
type RetentionRule struct {
	ID             int64               `json:"id,omitempty"`
	Priority       int                 `json:"priority"`
	Disabled       bool                `json:"disabled"`
	Action         string              `json:"action"`        // retain
	Template       string              `json:"template"`      // latestPushedK, latestPulledN, nDaysSinceLastPull, etc.
	Params         map[string]string   `json:"params"`        // 模板参数
	TagSelectors   []RetentionSelector `json:"tag_selectors"` // 标签选择器
	ScopeSelectors map[string][]struct {
		Repository []RetentionSelector `json:"repository"`
	} `json:"scope_selectors"` // 范围选择器
}

// RetentionPolicy 保留策略
type RetentionPolicy struct {
	ID         int64           `json:"id"`
	Algorithm  string          `json:"algorithm"` // or
	Rules      []RetentionRule `json:"rules"`
	Trigger    *Trigger        `json:"trigger"`
	Scope      *RetentionScope `json:"scope"`
	CreateTime time.Time       `json:"create_time"`
	UpdateTime time.Time       `json:"update_time"`
}

// Trigger 触发器
type Trigger struct {
	Kind       string            `json:"kind"`       // Schedule, Event
	Settings   map[string]string `json:"settings"`   // 触发设置
	References map[string]int64  `json:"references"` // 引用
}

// RetentionScope 保留策略作用域
type RetentionScope struct {
	Level int64 `json:"level"` // 1=项目级别
	Ref   int64 `json:"ref"`   // 项目 ID
}

// RetentionExecution 保留策略执行记录
type RetentionExecution struct {
	ID        int64     `json:"id"`
	PolicyID  int64     `json:"policy_id"`
	Status    string    `json:"status"`  // Pending, Running, Success, Failed, Stopped
	Trigger   string    `json:"trigger"` // manual, scheduled
	DryRun    bool      `json:"dry_run"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// ListRetentionExecutionsResponse 保留策略执行历史响应
type ListRetentionExecutionsResponse struct {
	ListResponse
	Items []RetentionExecution `json:"items"`
}

// ==================== 复制策略相关 ====================

// ReplicationFilter 复制过滤器
type ReplicationFilter struct {
	Type       string `json:"type"`       // resource, name, tag, label
	Value      string `json:"value"`      // 过滤值
	Decoration string `json:"decoration"` // matches, excludes
}

// ReplicationTrigger 复制触发器
type ReplicationTrigger struct {
	Type            string            `json:"type"`             // manual, scheduled, event_based
	TriggerSettings map[string]string `json:"trigger_settings"` // 触发设置
}

// ReplicationPolicy 复制策略
type ReplicationPolicy struct {
	ID            int64               `json:"id"`
	Name          string              `json:"name"`
	Description   string              `json:"description"`
	Creator       string              `json:"creator"`
	SrcRegistry   *Registry           `json:"src_registry"`
	DestRegistry  *Registry           `json:"dest_registry"`
	DestNamespace string              `json:"dest_namespace"`
	Filters       []ReplicationFilter `json:"filters"`
	Trigger       *ReplicationTrigger `json:"trigger"`
	Deletion      bool                `json:"deletion"`
	Override      bool                `json:"override"`
	Enabled       bool                `json:"enabled"`
	CreationTime  time.Time           `json:"creation_time"`
	UpdateTime    time.Time           `json:"update_time"`
}

// Registry 仓库信息（用于复制策略）
type Registry struct {
	ID         int64       `json:"id"`
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	URL        string      `json:"url"`
	Credential *Credential `json:"credential,omitempty"`
	Insecure   bool        `json:"insecure"`
	Status     string      `json:"status"`
}

// Credential 凭证信息
type Credential struct {
	Type         string `json:"type"` // basic, oauth
	AccessKey    string `json:"access_key,omitempty"`
	AccessSecret string `json:"access_secret,omitempty"`
}

// ReplicationExecution 复制执行记录
type ReplicationExecution struct {
	ID         int64     `json:"id"`
	PolicyID   int64     `json:"policy_id"`
	Status     string    `json:"status"` // Pending, Running, Success, Failed, Stopped
	Trigger    string    `json:"trigger"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	StatusText string    `json:"status_text"`
	Total      int64     `json:"total"`
	Failed     int64     `json:"failed"`
	Succeed    int64     `json:"succeed"`
	InProgress int64     `json:"in_progress"`
	Stopped    int64     `json:"stopped"`
}

// ListReplicationPoliciesResponse 复制策略列表响应
type ListReplicationPoliciesResponse struct {
	ListResponse
	Items []ReplicationPolicy `json:"items"`
}

// ListReplicationExecutionsResponse 复制执行历史响应
type ListReplicationExecutionsResponse struct {
	ListResponse
	Items []ReplicationExecution `json:"items"`
}

// ==================== 操作器接口扩展 ====================

// GCOperator 垃圾回收操作器接口
type GCOperator interface {
	// ListHistory 列出 GC 历史
	ListHistory(req ListRequest) (*ListGCHistoryResponse, error)
	// Get 获取 GC 详情
	Get(gcID int64) (*GCHistory, error)
	// GetLog 获取 GC 日志
	GetLog(gcID int64) (string, error)
	// GetSchedule 获取 GC 调度配置
	GetSchedule() (*GCSchedule, error)
	// UpdateSchedule 更新 GC 调度配置
	UpdateSchedule(schedule string, deleteUntagged bool) error
	// Trigger 手动触发 GC
	Trigger(deleteUntagged bool, workers int) (int64, error)
}

// RetentionOperator 保留策略操作器接口
type RetentionOperator interface {
	// GetByProject 通过项目获取保留策略
	GetByProject(projectName string) (*RetentionPolicy, error)
	// Get 获取保留策略
	Get(policyID int64) (*RetentionPolicy, error)
	// Create 创建保留策略
	Create(projectName string, policy *RetentionPolicy) (int64, error)
	// Update 更新保留策略
	Update(policyID int64, policy *RetentionPolicy) error
	// Execute 执行保留策略
	Execute(policyID int64, dryRun bool) (int64, error)
	// ListExecutions 列出执行历史
	ListExecutions(policyID int64, req ListRequest) (*ListRetentionExecutionsResponse, error)
}

// ReplicationOperator 复制策略操作器接口
type ReplicationOperator interface {
	// List 列出复制策略
	List(req ListRequest) (*ListReplicationPoliciesResponse, error)
	// Get 获取复制策略
	Get(policyID int64) (*ReplicationPolicy, error)
	// Create 创建复制策略
	Create(policy *ReplicationPolicy) (int64, error)
	// Update 更新复制策略
	Update(policyID int64, policy *ReplicationPolicy) error
	// Delete 删除复制策略
	Delete(policyID int64) error
	// Execute 执行复制策略
	Execute(policyID int64) (int64, error)
	// ListExecutions 列出执行历史
	ListExecutions(policyID int64, req ListRequest) (*ListReplicationExecutionsResponse, error)
}

// HarborClient Harbor 客户端接口
type HarborClient interface {
	GetUUID() string
	GetName() string
	GetEndpoint() string
	SystemInfo() (*SystemInfo, error)
	Project() ProjectOperator
	Repository() RepositoryOperator
	Artifact() ArtifactOperator
	Quota() QuotaOperator
	User() UserOperator               // 新增
	GC() GCOperator                   // 垃圾回收操作器
	Retention() RetentionOperator     // 保留策略操作器
	Replication() ReplicationOperator // 复制策略操作器
	Member() MemberOperator
	Ping() error
	Close() error
	SearchImages(imageName string) ([]RepositorySearchResult, error)
	GetUploadCredential(projectName string) (*UploadCredential, error)
	GetPullCommand(projectName, repoName, tag string) (*PullCommand, error)
}

// ProjectOperator 项目操作器接口
type ProjectOperator interface {
	List(req ListRequest) (*ListProjectResponse, error)
	Get(projectNameOrID string) (*Project, error)
	Create(req *ProjectReq) error
	Update(projectNameOrID string, req *ProjectReq) error
	Delete(projectNameOrID string) error
	Summary(projectNameOrID string) (*SystemInfo, error)
	ListPublic(req ListRequest) (*ListProjectResponse, error)
}

// RepositoryOperator 仓库操作器接口
type RepositoryOperator interface {
	List(projectName string, req ListRequest) (*ListRepositoryResponse, error)
	Get(projectName, repoName string) (*Repository, error)
	Delete(projectName, repoName string) error
	Update(projectName, repoName, description string) error
}

// ArtifactOperator 制品操作器接口
type ArtifactOperator interface {
	List(projectName, repoName string, req ListRequest) (*ListArtifactResponse, error)
	Get(projectName, repoName, reference string) (*Artifact, error)
	Delete(projectName, repoName, reference string) error
	ListTags(projectName, repoName, reference string) ([]Tag, error)
	CreateTag(projectName, repoName, reference, tagName string) error
	DeleteTag(projectName, repoName, tagName string) error
}

// QuotaOperator 配额操作器接口
type QuotaOperator interface {
	List(refType, refID string) ([]Quota, error)
	Get(id int64) (*Quota, error)
	Update(id int64, hard ResourceList) error
	GetByProject(projectName string) (*Quota, error)
	UpdateByProject(projectName string, hard ResourceList) error
}

// MemberOperator 成员操作器接口
type MemberOperator interface {
	List(projectNameOrID string, req ListRequest) (*ListProjectMemberResponse, error)
	Get(projectNameOrID string, memberID int64) (*ProjectMember, error)
	Add(projectNameOrID string, req *ProjectMemberReq) (int64, error)
	Update(projectNameOrID string, memberID int64, req *ProjectMemberReq) error
	Delete(projectNameOrID string, memberID int64) error
}

// HarborUser Harbor 用户
type HarborUser struct {
	UserID          int64     `json:"user_id"`
	Username        string    `json:"username"`
	Email           string    `json:"email"`
	Realname        string    `json:"realname"`
	Comment         string    `json:"comment"`
	CreationTime    time.Time `json:"creation_time"`
	UpdateTime      time.Time `json:"update_time"`
	SysAdminFlag    bool      `json:"sysadmin_flag"`
	AdminRoleInAuth bool      `json:"admin_role_in_auth"`
}

// UserReq 用户请求
type UserReq struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Realname string `json:"realname"`
	Password string `json:"password"`
	Comment  string `json:"comment"`
}

// UserUpdateReq 用户更新请求
type UserUpdateReq struct {
	Email    string `json:"email,omitempty"`
	Realname string `json:"realname,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

// PasswordReq 密码修改请求
type PasswordReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ListUserResponse 用户列表响应
type ListUserResponse struct {
	ListResponse
	Items []HarborUser `json:"items"`
}

// UserOperator 用户操作器接口
type UserOperator interface {
	List(req ListRequest) (*ListUserResponse, error)
	Get(userID int64) (*HarborUser, error)
	GetByUsername(username string) (*HarborUser, error)
	Create(req *UserReq) (int64, error)
	Update(userID int64, req *UserUpdateReq) error
	Delete(userID int64) error
	ChangePassword(userID int64, req *PasswordReq) error
	SetAdmin(userID int64, isAdmin bool) error
}
