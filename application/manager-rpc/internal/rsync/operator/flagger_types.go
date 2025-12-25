package operator

// FlaggerResourceInfo Flagger 资源信息
type FlaggerResourceInfo struct {
	IsFlaggerManaged bool   // 是否由 Flagger 管理
	OriginalAppName  string // 原始应用名（去除 -primary/-canary 后缀）
	VersionRole      string // 版本角色
	ParentName       string // 父名称，用于关联 primary 和 canary
}

const (
	VersionRoleStable  = "stable"
	VersionRolePrimary = "primary"
	VersionRoleCanary  = "canary"
	VersionRoleBlue    = "blue"
	VersionRoleGreen   = "green"

	FlaggerManagedByLabel    = "app.kubernetes.io/managed-by"
	FlaggerAppNameLabel      = "app.kubernetes.io/name"
	FlaggerPrimaryAnnotation = "flagger.app/primary"
	FlaggerCanaryAnnotation  = "flagger.app/canary"
)
