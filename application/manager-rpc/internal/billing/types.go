package billing

import (
	"database/sql"
	"time"
)

// 账单类型常量
const (
	StatementTypeInitial      = "initial"       // 首次生成账单
	StatementTypeDaily        = "daily"         // 日常账单
	StatementTypeConfigChange = "config_change" // 配置变更账单
)

// 绑定类型常量
const (
	BindingTypeCluster        = "cluster"         // 集群默认配置
	BindingTypeProjectCluster = "project_cluster" // 项目集群配置
)

// GenerateOption 账单生成选项
type GenerateOption struct {
	StatementType string // 账单类型：initial/daily/config_change
	CreatedBy     string // 创建人
}

// GenerateResult 账单生成结果
type GenerateResult struct {
	TotalCount   int      // 生成账单总数
	SuccessCount int      // 成功数量
	FailedCount  int      // 失败数量
	FailedItems  []string // 失败项详情
}

// ProjectClusterInfo 项目集群信息，用于账单生成
type ProjectClusterInfo struct {
	ProjectClusterId uint64  // 项目集群ID
	ProjectId        uint64  // 项目ID
	ProjectName      string  // 项目名称
	ProjectUuid      string  // 项目UUID
	ClusterUuid      string  // 集群UUID
	ClusterName      string  // 集群名称
	CpuCapacity      float64 // CPU超分后容量（核）
	MemCapacity      float64 // 内存超分后容量（GiB）
	StorageLimit     float64 // 存储配额（GiB）
	GpuCapacity      float64 // GPU超分后容量
	PodsLimit        int64   // Pod配额
	WorkspaceCount   int64   // 工作空间数量
	ApplicationCount int64   // 应用数量
}

// PriceConfig 价格配置信息
type PriceConfig struct {
	ConfigId      uint64  // 配置ID
	CpuPrice      float64 // CPU单价（元/核/小时）
	MemoryPrice   float64 // 内存单价（元/GiB/小时）
	StoragePrice  float64 // 存储单价（元/GiB/小时）
	GpuPrice      float64 // GPU单价（元/卡/小时）
	PodPrice      float64 // Pod单价（元/个/小时）
	ManagementFee float64 // 管理费（元/小时）
}

// BindingInfo 计费绑定信息
type BindingInfo struct {
	BindingId        uint64       // 绑定ID
	BindingType      string       // 绑定类型
	ClusterUuid      string       // 集群UUID
	ProjectId        uint64       // 项目ID
	PriceConfigId    uint64       // 价格配置ID
	BillingStartTime sql.NullTime // 计费开始时间
	LastBillingTime  sql.NullTime // 上次计费时间
}

// BillingPeriod 计费周期
type BillingPeriod struct {
	StartTime    time.Time // 开始时间
	EndTime      time.Time // 结束时间
	BillingHours float64   // 计费时长（小时）
}

// StatementDetail 账单明细
type StatementDetail struct {
	// 基本信息
	StatementNo   string // 账单编号
	StatementType string // 账单类型

	// 计费周期
	BillingStartTime time.Time // 计费开始时间
	BillingEndTime   time.Time // 计费结束时间
	BillingHours     float64   // 计费时长（小时）

	// 关联信息
	ClusterUuid      string // 集群UUID
	ClusterName      string // 集群名称
	ProjectId        uint64 // 项目ID
	ProjectName      string // 项目名称
	ProjectUuid      string // 项目UUID
	ProjectClusterId uint64 // 项目集群ID
	BindingId        uint64 // 绑定ID

	// 资源配额快照
	CpuCapacity      string // CPU容量
	MemCapacity      string // 内存容量
	StorageLimit     string // 存储配额
	GpuCapacity      string // GPU容量
	PodsLimit        int64  // Pod配额
	WorkspaceCount   int64  // 工作空间数量
	ApplicationCount int64  // 应用数量

	// 单价快照
	PriceCpu           float64 // CPU单价
	PriceMemory        float64 // 内存单价
	PriceStorage       float64 // 存储单价
	PriceGpu           float64 // GPU单价
	PricePod           float64 // Pod单价
	PriceManagementFee float64 // 管理费单价

	// 费用明细
	CpuCost           float64 // CPU费用
	MemoryCost        float64 // 内存费用
	StorageCost       float64 // 存储费用
	GpuCost           float64 // GPU费用
	PodCost           float64 // Pod费用
	ManagementFee     float64 // 管理费
	ResourceCostTotal float64 // 资源费用合计
	TotalAmount       float64 // 总费用

	// 其他
	Remark    string // 备注
	CreatedBy string // 创建人
}
