package operator

// FlaggerResourceInfo Flagger 资源信息
type FlaggerResourceInfo struct {
	IsFlaggerManaged bool   // 是否由 Flagger 管理
	OriginalAppName  string // 原始应用名
	VersionRole      string // 版本角色/策略类型
	CanaryName       string // 关联的 Canary CRD 名称
}

// CanaryStrategyInfo Canary 策略信息
type CanaryStrategyInfo struct {
	Name         string // Canary 名称
	TargetName   string // 目标资源名称
	TargetKind   string // 目标资源类型 (Deployment, DaemonSet)
	StrategyType string // 策略类型
}

// VersionRole 常量定义
const (
	// VersionRoleStable 普通资源，或被 Flagger 管理的原始资源（targetRef 指向的资源）
	VersionRoleStable = "stable"
	// VersionRoleCanary 金丝雀发布策略 - 使用 stepWeight/maxWeight 渐进式流量切换
	VersionRoleCanary = "canary"
	// VersionRoleBlueGreen 蓝绿发布策略 - 使用 iterations，一次性切换
	VersionRoleBlueGreen = "bluegreen"
	// VersionRoleABTest A/B 测试策略 - 使用 HTTP 头匹配规则
	VersionRoleABTest = "ab"
	// VersionRoleMirroring 镜像流量策略 - 复制流量到金丝雀版本
	VersionRoleMirroring = "mirroring"
)

// Flagger 相关标签和注解
const (
	FlaggerManagedByLabel = "app.kubernetes.io/managed-by"
	FlaggerAppNameLabel   = "app.kubernetes.io/name"
)

// Flagger CRD 信息
const (
	FlaggerCRDGroup   = "flagger.app"
	FlaggerCRDVersion = "v1beta1"
	FlaggerCRDKind    = "Canary"
	FlaggerCRDPlural  = "canaries"
)
