package types

import "context"

const (
	EnableHardDelete = true
)

// SyncService 定义了资源同步服务的接口
// 提供项目和集群两个维度的资源同步功能
type SyncService interface {
	// ========== 集群级别资源同步 ==========
	SyncClusterVersion(ctx context.Context, clusterUuid, operator string, enableAudit bool) error
	SyncClusterResource(ctx context.Context, clusterUuid, operator string, enableAudit bool) error
	SyncClusterNetwork(ctx context.Context, clusterUuid, operator string, enableAudit bool) error

	// ========== 集群节点同步 ==========
	// SyncClusterNodes 同步某个集群的所有节点信息
	SyncClusterNodes(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error

	// ========== 项目资源同步 ==========
	// SyncOneProjectAllResource  同步某一个项目的所有资源信息
	SyncOneProjectAllResource(ctx context.Context, projectId uint64, operator string, enableAudit bool) error
	// SyncAllProjectAllResource 同步所有项目的所有资源信息
	SyncAllProjectAllResource(ctx context.Context, operator string, enableAudit bool) error

	// ========== 集群资源同步 ==========
	// SyncOneClusterAllResource SyncClusterAllResource 同步某一个集群的资源
	SyncOneClusterAllResource(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error
	// SyncAllClusterAllResource 同步所有集群的资源
	SyncAllClusterAllResource(ctx context.Context, operator string, enableAudit bool) error

	// ========== Namespace 资源同步 ==========
	// SyncClusterNamespaces 同步某个集群的所有 Namespace 资源
	SyncClusterNamespaces(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error
	// SyncNamespaceResources 同步某个 Namespace 的资源配额和限制
	SyncNamespaceResources(ctx context.Context, clusterUuid string, namespace string, operator string, enableAudit bool) error
	// SyncAllClusterNamespaces 同步所有集群的 Namespace 资源
	SyncAllClusterNamespaces(ctx context.Context, operator string, enableAudit bool) error
	// SyncAllWorkspaceResources 同步某个集群下所有工作空间的资源配额和限制
	SyncAllWorkspaceResources(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error

	// ========== 应用资源同步 ==========
	// SyncClusterApplications 同步某个集群的所有应用资源
	SyncClusterApplications(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error
	// SyncNamespaceApplications 同步某个 Namespace 的应用资源
	SyncNamespaceApplications(ctx context.Context, clusterUuid string, namespace string, operator string, enableAudit bool) (int, int, int, error)
	// SyncAllClusterApplications 同步所有集群的应用资源
	SyncAllClusterApplications(ctx context.Context, operator string, enableAudit bool) error

	// SyncAll 同步所有资源
	SyncAll(ctx context.Context, operator string, enableAudit bool) error
	// SyncOneCLuster 同步单个集群的所有资源
	SyncOneCLuster(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error
}
