package operator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
)

const (
	// ResourceQuotaNamePrefix ResourceQuota 名称前缀
	ResourceQuotaNamePrefix = "ikubeops-"
	// LimitRangeNamePrefix LimitRange 名称前缀
	LimitRangeNamePrefix = "ikubeops-"
)

// ==================== Namespace 资源配额和限制同步 ====================

// SyncNamespaceResources 同步某个 Namespace 的资源配额和限制
// 逻辑：从 K8s 读取 ResourceQuota 和 LimitRange，然后同步到数据库
// 注意：此方法只读取 K8s 数据同步到数据库，不会在 K8s 中创建任何资源
func (s *ClusterResourceSync) SyncNamespaceResources(ctx context.Context, clusterUuid string, namespace string, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步 Namespace 资源配额和限制, clusterUuid: %s, namespace: %s, operator: %s", clusterUuid, namespace, operator)

	// 1. 验证集群是否存在
	onecCluster, err := s.ClusterModel.FindOneByUuid(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询集群失败, clusterUuid: %s, error: %v", clusterUuid, err)
		return fmt.Errorf("查询集群失败: %v", err)
	}

	// 2. 获取 K8s 客户端
	k8sClient, err := s.K8sManager.GetCluster(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("获取K8s客户端失败, clusterUuid: %s, error: %v", clusterUuid, err)
		return fmt.Errorf("获取K8s客户端失败: %v", err)
	}

	// 3. 查询数据库中的 Workspace 记录
	query := "cluster_uuid = ? AND namespace = ?"
	workspaces, err := s.ProjectWorkspaceModel.SearchNoPage(ctx, "", false, query, clusterUuid, namespace)
	if err != nil || len(workspaces) == 0 {
		s.Logger.WithContext(ctx).Errorf("查询 Workspace 失败或不存在, namespace: %s", namespace)
		return fmt.Errorf("查询 Workspace 失败或不存在")
	}

	workspace := workspaces[0]

	// 4. 并发同步 ResourceQuota 和 LimitRange
	var wg sync.WaitGroup
	var errResourceQuota, errLimitRange error
	quotaChanged := false
	limitChanged := false

	// 同步 ResourceQuota
	wg.Add(1)
	go func() {
		defer wg.Done()
		var changed bool
		changed, errResourceQuota = s.syncResourceQuotaFromK8s(ctx, k8sClient, onecCluster, workspace, operator)
		quotaChanged = changed
		if errResourceQuota != nil {
			s.Logger.WithContext(ctx).Errorf("同步 ResourceQuota 失败: %v", errResourceQuota)
		}
	}()

	// 同步 LimitRange
	wg.Add(1)
	go func() {
		defer wg.Done()
		var changed bool
		changed, errLimitRange = s.syncLimitRangeFromK8s(ctx, k8sClient, onecCluster, workspace, operator)
		limitChanged = changed
		if errLimitRange != nil {
			s.Logger.WithContext(ctx).Errorf("同步 LimitRange 失败: %v", errLimitRange)
		}
	}()

	wg.Wait()

	// 5. 记录审计日志
	if enableAudit && (quotaChanged || limitChanged) {
		auditDetail := fmt.Sprintf("NS[%s]配额同步", namespace)
		if quotaChanged {
			auditDetail += ": ResourceQuota已更新"
		}
		if limitChanged {
			if quotaChanged {
				auditDetail += ", "
			} else {
				auditDetail += ": "
			}
			auditDetail += "LimitRange已更新"
		}

		s.writeProjectAuditLog(ctx, workspace.Id, 0, workspace.ProjectClusterId, operator,
			namespace, "Workspace", "QUOTA_SYNC", auditDetail, 1)
	}

	s.Logger.WithContext(ctx).Infof("Namespace 资源配额和限制同步完成, namespace: %s", namespace)

	return nil
}

// syncResourceQuotaFromK8s 从 K8s 读取 ResourceQuota 并同步到数据库
// 注意：此方法只读取 K8s 数据同步到数据库，不会在 K8s 中创建任何资源
// 【修复】完整的变更检测(18字段) + 资源汇总同步 + 缓存清理
func (s *ClusterResourceSync) syncResourceQuotaFromK8s(ctx context.Context, k8sClient cluster.Client, cluster *model.OnecCluster, workspace *model.OnecProjectWorkspace, operator string) (bool, error) {
	namespace := workspace.Namespace
	resourceQuotaName := ResourceQuotaNamePrefix + namespace

	s.Logger.WithContext(ctx).Infof("从 K8s 读取 ResourceQuota: %s/%s", namespace, resourceQuotaName)

	// 获取 ResourceQuota Operator
	resourceQuotaOp := k8sClient.ResourceQuota()

	// 从 K8s 读取 ResourceQuota
	allocated, err := resourceQuotaOp.GetAllocated(namespace, resourceQuotaName)
	if err != nil {
		// K8s 中不存在 ResourceQuota，使用默认值（全量同步不创建 K8s 资源）
		s.Logger.WithContext(ctx).Infof("K8s 中不存在 ResourceQuota，使用默认值: %s", resourceQuotaName)

		allocated = &types.ResourceQuotaAllocated{
			CPUAllocated:              "0",
			MemoryAllocated:           "0Gi",
			StorageAllocated:          "0Gi",
			GPUAllocated:              "0",
			EphemeralStorageAllocated: "0Gi",
			PodsAllocated:             0,
			ConfigMapsAllocated:       0,
			SecretsAllocated:          0,
			PVCsAllocated:             0,
			ServicesAllocated:         0,
			LoadBalancersAllocated:    0,
			NodePortsAllocated:        0,
			DeploymentsAllocated:      0,
			JobsAllocated:             0,
			CronJobsAllocated:         0,
			DaemonSetsAllocated:       0,
			StatefulSetsAllocated:     0,
			IngressesAllocated:        0,
		}
	}

	// 【修复】完整的变更检测，检查所有 18 个字段
	changes := s.checkResourceQuotaChanges(workspace, allocated)

	if len(changes) == 0 {
		s.Logger.WithContext(ctx).Debugf("ResourceQuota 无变更，跳过: %s", resourceQuotaName)
		return false, nil
	}

	// 更新数据库中的 workspace 记录
	workspace.CpuAllocated = allocated.CPUAllocated
	workspace.MemAllocated = allocated.MemoryAllocated
	workspace.StorageAllocated = allocated.StorageAllocated
	workspace.GpuAllocated = allocated.GPUAllocated
	workspace.EphemeralStorageAllocated = allocated.EphemeralStorageAllocated
	workspace.PodsAllocated = allocated.PodsAllocated
	workspace.ConfigmapAllocated = allocated.ConfigMapsAllocated
	workspace.SecretAllocated = allocated.SecretsAllocated
	workspace.PvcAllocated = allocated.PVCsAllocated
	workspace.ServiceAllocated = allocated.ServicesAllocated
	workspace.LoadbalancersAllocated = allocated.LoadBalancersAllocated
	workspace.NodeportsAllocated = allocated.NodePortsAllocated
	workspace.DeploymentsAllocated = allocated.DeploymentsAllocated
	workspace.JobsAllocated = allocated.JobsAllocated
	workspace.CronjobsAllocated = allocated.CronJobsAllocated
	workspace.DaemonsetsAllocated = allocated.DaemonSetsAllocated
	workspace.StatefulsetsAllocated = allocated.StatefulSetsAllocated
	workspace.IngressesAllocated = allocated.IngressesAllocated
	workspace.UpdatedAt = time.Now()
	workspace.UpdatedBy = operator

	// 更新数据库
	if err := s.ProjectWorkspaceModel.Update(ctx, workspace); err != nil {
		s.Logger.WithContext(ctx).Errorf("更新数据库 Workspace 失败: %v", err)
		return false, fmt.Errorf("更新数据库 Workspace 失败: %v", err)
	}

	// 【修复】同步项目集群资源分配并清除缓存
	s.syncProjectClusterResourceAndCache(ctx, workspace.ProjectClusterId)

	s.Logger.WithContext(ctx).Infof("ResourceQuota 同步成功: %s, Changes=%v", resourceQuotaName, changes)
	return true, nil
}

// checkResourceQuotaChanges 检查 ResourceQuota 是否有变更，返回变更列表
// 检查所有 18 个字段，与增量同步保持一致
func (s *ClusterResourceSync) checkResourceQuotaChanges(workspace *model.OnecProjectWorkspace, allocated *types.ResourceQuotaAllocated) []string {
	var changes []string

	// 基础资源（5个）
	if workspace.CpuAllocated != allocated.CPUAllocated {
		changes = append(changes, fmt.Sprintf("CPU: %s -> %s", workspace.CpuAllocated, allocated.CPUAllocated))
	}
	if workspace.MemAllocated != allocated.MemoryAllocated {
		changes = append(changes, fmt.Sprintf("Mem: %s -> %s", workspace.MemAllocated, allocated.MemoryAllocated))
	}
	if workspace.StorageAllocated != allocated.StorageAllocated {
		changes = append(changes, fmt.Sprintf("Storage: %s -> %s", workspace.StorageAllocated, allocated.StorageAllocated))
	}
	if workspace.GpuAllocated != allocated.GPUAllocated {
		changes = append(changes, fmt.Sprintf("GPU: %s -> %s", workspace.GpuAllocated, allocated.GPUAllocated))
	}
	if workspace.EphemeralStorageAllocated != allocated.EphemeralStorageAllocated {
		changes = append(changes, fmt.Sprintf("EphemeralStorage: %s -> %s", workspace.EphemeralStorageAllocated, allocated.EphemeralStorageAllocated))
	}

	// 对象数量（7个）
	if workspace.PodsAllocated != allocated.PodsAllocated {
		changes = append(changes, fmt.Sprintf("Pods: %d -> %d", workspace.PodsAllocated, allocated.PodsAllocated))
	}
	if workspace.ConfigmapAllocated != allocated.ConfigMapsAllocated {
		changes = append(changes, fmt.Sprintf("ConfigMaps: %d -> %d", workspace.ConfigmapAllocated, allocated.ConfigMapsAllocated))
	}
	if workspace.SecretAllocated != allocated.SecretsAllocated {
		changes = append(changes, fmt.Sprintf("Secrets: %d -> %d", workspace.SecretAllocated, allocated.SecretsAllocated))
	}
	if workspace.PvcAllocated != allocated.PVCsAllocated {
		changes = append(changes, fmt.Sprintf("PVCs: %d -> %d", workspace.PvcAllocated, allocated.PVCsAllocated))
	}
	if workspace.ServiceAllocated != allocated.ServicesAllocated {
		changes = append(changes, fmt.Sprintf("Services: %d -> %d", workspace.ServiceAllocated, allocated.ServicesAllocated))
	}
	if workspace.LoadbalancersAllocated != allocated.LoadBalancersAllocated {
		changes = append(changes, fmt.Sprintf("LoadBalancers: %d -> %d", workspace.LoadbalancersAllocated, allocated.LoadBalancersAllocated))
	}
	if workspace.NodeportsAllocated != allocated.NodePortsAllocated {
		changes = append(changes, fmt.Sprintf("NodePorts: %d -> %d", workspace.NodeportsAllocated, allocated.NodePortsAllocated))
	}

	// 工作负载数量（6个）
	if workspace.DeploymentsAllocated != allocated.DeploymentsAllocated {
		changes = append(changes, fmt.Sprintf("Deployments: %d -> %d", workspace.DeploymentsAllocated, allocated.DeploymentsAllocated))
	}
	if workspace.JobsAllocated != allocated.JobsAllocated {
		changes = append(changes, fmt.Sprintf("Jobs: %d -> %d", workspace.JobsAllocated, allocated.JobsAllocated))
	}
	if workspace.CronjobsAllocated != allocated.CronJobsAllocated {
		changes = append(changes, fmt.Sprintf("CronJobs: %d -> %d", workspace.CronjobsAllocated, allocated.CronJobsAllocated))
	}
	if workspace.DaemonsetsAllocated != allocated.DaemonSetsAllocated {
		changes = append(changes, fmt.Sprintf("DaemonSets: %d -> %d", workspace.DaemonsetsAllocated, allocated.DaemonSetsAllocated))
	}
	if workspace.StatefulsetsAllocated != allocated.StatefulSetsAllocated {
		changes = append(changes, fmt.Sprintf("StatefulSets: %d -> %d", workspace.StatefulsetsAllocated, allocated.StatefulSetsAllocated))
	}
	if workspace.IngressesAllocated != allocated.IngressesAllocated {
		changes = append(changes, fmt.Sprintf("Ingresses: %d -> %d", workspace.IngressesAllocated, allocated.IngressesAllocated))
	}

	return changes
}

// syncLimitRangeFromK8s 从 K8s 读取 LimitRange 并同步到数据库
// 注意：此方法只读取 K8s 数据同步到数据库，不会在 K8s 中创建任何资源
// 【修复】完整的变更检测（18个字段）
func (s *ClusterResourceSync) syncLimitRangeFromK8s(ctx context.Context, k8sClient cluster.Client, cluster *model.OnecCluster, workspace *model.OnecProjectWorkspace, operator string) (bool, error) {
	namespace := workspace.Namespace
	limitRangeName := LimitRangeNamePrefix + namespace

	s.Logger.WithContext(ctx).Infof("从 K8s 读取 LimitRange: %s/%s", namespace, limitRangeName)

	// 获取 LimitRange Operator
	limitRangeOp := k8sClient.LimitRange()

	// 从 K8s 读取 LimitRange
	limits, err := limitRangeOp.GetLimits(namespace, limitRangeName)
	if err != nil {
		// K8s 中不存在 LimitRange，使用默认值（全量同步不创建 K8s 资源）
		s.Logger.WithContext(ctx).Infof("K8s 中不存在 LimitRange，使用默认值: %s", limitRangeName)

		// 默认值与增量同步保持一致
		limits = &types.LimitRangeLimits{
			PodMaxCPU:                               "0",
			PodMaxMemory:                            "0Gi",
			PodMaxEphemeralStorage:                  "0Gi",
			PodMinCPU:                               "0",
			PodMinMemory:                            "0Mi",
			PodMinEphemeralStorage:                  "0Mi",
			ContainerMaxCPU:                         "0",
			ContainerMaxMemory:                      "0Gi",
			ContainerMaxEphemeralStorage:            "0Gi",
			ContainerMinCPU:                         "0",
			ContainerMinMemory:                      "0Mi",
			ContainerMinEphemeralStorage:            "0Mi",
			ContainerDefaultCPU:                     "100m",
			ContainerDefaultMemory:                  "128Mi",
			ContainerDefaultEphemeralStorage:        "0Gi",
			ContainerDefaultRequestCPU:              "50m",
			ContainerDefaultRequestMemory:           "64Mi",
			ContainerDefaultRequestEphemeralStorage: "0Mi",
		}
	}

	// 【修复】完整的变更检测，检查所有 18 个字段
	changes := s.checkLimitRangeChanges(workspace, limits)

	if len(changes) == 0 {
		s.Logger.WithContext(ctx).Debugf("LimitRange 无变更，跳过: %s", limitRangeName)
		return false, nil
	}

	// 更新数据库中的 workspace 记录
	workspace.PodMaxCpu = limits.PodMaxCPU
	workspace.PodMaxMemory = limits.PodMaxMemory
	workspace.PodMaxEphemeralStorage = limits.PodMaxEphemeralStorage
	workspace.PodMinCpu = limits.PodMinCPU
	workspace.PodMinMemory = limits.PodMinMemory
	workspace.PodMinEphemeralStorage = limits.PodMinEphemeralStorage
	workspace.ContainerMaxCpu = limits.ContainerMaxCPU
	workspace.ContainerMaxMemory = limits.ContainerMaxMemory
	workspace.ContainerMaxEphemeralStorage = limits.ContainerMaxEphemeralStorage
	workspace.ContainerMinCpu = limits.ContainerMinCPU
	workspace.ContainerMinMemory = limits.ContainerMinMemory
	workspace.ContainerMinEphemeralStorage = limits.ContainerMinEphemeralStorage
	workspace.ContainerDefaultCpu = limits.ContainerDefaultCPU
	workspace.ContainerDefaultMemory = limits.ContainerDefaultMemory
	workspace.ContainerDefaultEphemeralStorage = limits.ContainerDefaultEphemeralStorage
	workspace.ContainerDefaultRequestCpu = limits.ContainerDefaultRequestCPU
	workspace.ContainerDefaultRequestMemory = limits.ContainerDefaultRequestMemory
	workspace.ContainerDefaultRequestEphemeralStorage = limits.ContainerDefaultRequestEphemeralStorage
	workspace.UpdatedAt = time.Now()
	workspace.UpdatedBy = operator

	// 更新数据库
	if err := s.ProjectWorkspaceModel.Update(ctx, workspace); err != nil {
		s.Logger.WithContext(ctx).Errorf("更新数据库 Workspace 失败: %v", err)
		return false, fmt.Errorf("更新数据库 Workspace 失败: %v", err)
	}

	// 【注意】LimitRange 是限制配置，不是资源配额，因此不触发项目集群资源同步
	// 这与增量同步的设计保持一致

	s.Logger.WithContext(ctx).Infof("LimitRange 同步成功: %s, Changes=%v", limitRangeName, changes)
	return true, nil
}

// checkLimitRangeChanges 检查 LimitRange 是否有变更，返回变更列表
// 检查所有 18 个字段，与增量同步保持一致
func (s *ClusterResourceSync) checkLimitRangeChanges(workspace *model.OnecProjectWorkspace, limits *types.LimitRangeLimits) []string {
	var changes []string

	// Pod 级别最大限制（3个）
	if workspace.PodMaxCpu != limits.PodMaxCPU {
		changes = append(changes, fmt.Sprintf("PodMaxCpu: %s -> %s", workspace.PodMaxCpu, limits.PodMaxCPU))
	}
	if workspace.PodMaxMemory != limits.PodMaxMemory {
		changes = append(changes, fmt.Sprintf("PodMaxMemory: %s -> %s", workspace.PodMaxMemory, limits.PodMaxMemory))
	}
	if workspace.PodMaxEphemeralStorage != limits.PodMaxEphemeralStorage {
		changes = append(changes, fmt.Sprintf("PodMaxEphemeralStorage: %s -> %s", workspace.PodMaxEphemeralStorage, limits.PodMaxEphemeralStorage))
	}

	// Pod 级别最小限制（3个）
	if workspace.PodMinCpu != limits.PodMinCPU {
		changes = append(changes, fmt.Sprintf("PodMinCpu: %s -> %s", workspace.PodMinCpu, limits.PodMinCPU))
	}
	if workspace.PodMinMemory != limits.PodMinMemory {
		changes = append(changes, fmt.Sprintf("PodMinMemory: %s -> %s", workspace.PodMinMemory, limits.PodMinMemory))
	}
	if workspace.PodMinEphemeralStorage != limits.PodMinEphemeralStorage {
		changes = append(changes, fmt.Sprintf("PodMinEphemeralStorage: %s -> %s", workspace.PodMinEphemeralStorage, limits.PodMinEphemeralStorage))
	}

	// Container 级别最大限制（3个）
	if workspace.ContainerMaxCpu != limits.ContainerMaxCPU {
		changes = append(changes, fmt.Sprintf("ContainerMaxCpu: %s -> %s", workspace.ContainerMaxCpu, limits.ContainerMaxCPU))
	}
	if workspace.ContainerMaxMemory != limits.ContainerMaxMemory {
		changes = append(changes, fmt.Sprintf("ContainerMaxMemory: %s -> %s", workspace.ContainerMaxMemory, limits.ContainerMaxMemory))
	}
	if workspace.ContainerMaxEphemeralStorage != limits.ContainerMaxEphemeralStorage {
		changes = append(changes, fmt.Sprintf("ContainerMaxEphemeralStorage: %s -> %s", workspace.ContainerMaxEphemeralStorage, limits.ContainerMaxEphemeralStorage))
	}

	// Container 级别最小限制（3个）
	if workspace.ContainerMinCpu != limits.ContainerMinCPU {
		changes = append(changes, fmt.Sprintf("ContainerMinCpu: %s -> %s", workspace.ContainerMinCpu, limits.ContainerMinCPU))
	}
	if workspace.ContainerMinMemory != limits.ContainerMinMemory {
		changes = append(changes, fmt.Sprintf("ContainerMinMemory: %s -> %s", workspace.ContainerMinMemory, limits.ContainerMinMemory))
	}
	if workspace.ContainerMinEphemeralStorage != limits.ContainerMinEphemeralStorage {
		changes = append(changes, fmt.Sprintf("ContainerMinEphemeralStorage: %s -> %s", workspace.ContainerMinEphemeralStorage, limits.ContainerMinEphemeralStorage))
	}

	// Container 默认限制（3个）
	if workspace.ContainerDefaultCpu != limits.ContainerDefaultCPU {
		changes = append(changes, fmt.Sprintf("ContainerDefaultCpu: %s -> %s", workspace.ContainerDefaultCpu, limits.ContainerDefaultCPU))
	}
	if workspace.ContainerDefaultMemory != limits.ContainerDefaultMemory {
		changes = append(changes, fmt.Sprintf("ContainerDefaultMemory: %s -> %s", workspace.ContainerDefaultMemory, limits.ContainerDefaultMemory))
	}
	if workspace.ContainerDefaultEphemeralStorage != limits.ContainerDefaultEphemeralStorage {
		changes = append(changes, fmt.Sprintf("ContainerDefaultEphemeralStorage: %s -> %s", workspace.ContainerDefaultEphemeralStorage, limits.ContainerDefaultEphemeralStorage))
	}

	// Container 默认请求（3个）
	if workspace.ContainerDefaultRequestCpu != limits.ContainerDefaultRequestCPU {
		changes = append(changes, fmt.Sprintf("ContainerDefaultRequestCpu: %s -> %s", workspace.ContainerDefaultRequestCpu, limits.ContainerDefaultRequestCPU))
	}
	if workspace.ContainerDefaultRequestMemory != limits.ContainerDefaultRequestMemory {
		changes = append(changes, fmt.Sprintf("ContainerDefaultRequestMemory: %s -> %s", workspace.ContainerDefaultRequestMemory, limits.ContainerDefaultRequestMemory))
	}
	if workspace.ContainerDefaultRequestEphemeralStorage != limits.ContainerDefaultRequestEphemeralStorage {
		changes = append(changes, fmt.Sprintf("ContainerDefaultRequestEphemeralStorage: %s -> %s", workspace.ContainerDefaultRequestEphemeralStorage, limits.ContainerDefaultRequestEphemeralStorage))
	}

	return changes
}

// ==================== 批量同步方法 ====================

// SyncAllWorkspaceResources 同步某个集群下所有活跃工作空间的资源配额和限制
func (s *ClusterResourceSync) SyncAllWorkspaceResources(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步集群所有工作空间资源, clusterUuid: %s, operator: %s", clusterUuid, operator)

	// 验证集群是否存在
	cluster, err := s.ClusterModel.FindOneByUuid(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询集群失败: %v", err)
		return fmt.Errorf("查询集群失败: %v", err)
	}

	// 查询该集群下的所有活跃工作空间（status=1表示正常）
	query := "cluster_uuid = ? AND status = 1"
	workspaces, err := s.ProjectWorkspaceModel.SearchNoPage(ctx, "", false, query, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询工作空间列表失败: %v", err)
		return fmt.Errorf("查询工作空间列表失败: %v", err)
	}

	if len(workspaces) == 0 {
		s.Logger.WithContext(ctx).Infof("该集群下没有活跃的工作空间")
		return nil
	}

	s.Logger.WithContext(ctx).Infof("找到 %d 个活跃工作空间", len(workspaces))

	// 并发处理所有工作空间
	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		successCnt int
		failCnt    int
		syncErrors []string
		semaphore  = make(chan struct{}, MaxNamespaceConcurrency)
	)

	for _, workspace := range workspaces {
		wg.Add(1)
		go func(ws *model.OnecProjectWorkspace) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			s.Logger.WithContext(ctx).Infof("同步工作空间资源: namespace=%s, id=%d", ws.Namespace, ws.Id)

			err := s.SyncNamespaceResources(ctx, clusterUuid, ws.Namespace, operator, false) // 批量同步时不记录每个工作空间的日志

			mu.Lock()
			if err != nil {
				failCnt++
				errMsg := fmt.Sprintf("WS[%s]", ws.Namespace)
				syncErrors = append(syncErrors, errMsg)
				s.Logger.WithContext(ctx).Errorf("同步工作空间[%s]资源失败: %v", ws.Namespace, err)
			} else {
				successCnt++
			}
			mu.Unlock()
		}(workspace)
	}

	wg.Wait()

	s.Logger.WithContext(ctx).Infof("工作空间资源同步完成: 总数=%d, 成功=%d, 失败=%d", len(workspaces), successCnt, failCnt)

	// 记录审计日志
	if enableAudit {
		status := int64(1)
		auditDetail := fmt.Sprintf("集群[%s]WS配额同步: 总数=%d, 成功=%d", cluster.Name, len(workspaces), successCnt)
		if failCnt > 0 {
			status = 2
			auditDetail = fmt.Sprintf("%s, 失败=%d", auditDetail, failCnt)
		}

		// 获取集群所属的项目集群绑定
		projectClusters, _ := s.ProjectClusterResourceModel.SearchNoPage(ctx, "", false, "cluster_uuid = ?", clusterUuid)
		for _, pc := range projectClusters {
			s.writeProjectAuditLog(ctx, 0, 0, pc.Id, operator, cluster.Name, "Workspace", "QUOTA_SYNC", auditDetail, status)
		}
	}

	if failCnt > 0 {
		return fmt.Errorf("部分工作空间资源同步失败: 成功=%d, 失败=%d", successCnt, failCnt)
	}

	return nil
}

// SyncAllClustersWorkspaceResources 同步所有集群的所有工作空间资源配额和限制
func (s *ClusterResourceSync) SyncAllClustersWorkspaceResources(ctx context.Context, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步所有集群的工作空间资源, operator: %s", operator)

	// 获取所有集群
	clusters, err := s.ClusterModel.GetAllClusters(ctx)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询集群列表失败: %v", err)
		return fmt.Errorf("查询集群列表失败: %v", err)
	}

	if len(clusters) == 0 {
		s.Logger.WithContext(ctx).Infof("没有找到任何集群")
		return nil
	}

	s.Logger.WithContext(ctx).Infof("找到 %d 个集群", len(clusters))

	// 并发处理所有集群
	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		successCnt int
		failCnt    int
		syncErrors []string
		semaphore  = make(chan struct{}, MaxClusterConcurrency)
	)

	for _, onecCluster := range clusters {
		wg.Add(1)
		go func(cluster *model.OnecCluster) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			s.Logger.WithContext(ctx).Infof("同步集群工作空间资源: name=%s, uuid=%s", cluster.Name, cluster.Uuid)

			err := s.SyncAllWorkspaceResources(ctx, cluster.Uuid, operator, false) // 批量同步时不记录每个集群的日志

			mu.Lock()
			if err != nil {
				failCnt++
				errMsg := fmt.Sprintf("集群[%s]", cluster.Name)
				syncErrors = append(syncErrors, errMsg)
				s.Logger.WithContext(ctx).Errorf("同步集群[%s]工作空间资源失败: %v", cluster.Name, err)
			} else {
				successCnt++
			}
			mu.Unlock()
		}(onecCluster)
	}

	wg.Wait()

	s.Logger.WithContext(ctx).Infof("所有集群工作空间资源同步完成: 总数=%d, 成功=%d, 失败=%d", len(clusters), successCnt, failCnt)

	// 记录批量同步审计日志
	if enableAudit {
		status := int64(1)
		auditDetail := fmt.Sprintf("批量WS配额同步: 集群总数=%d, 成功=%d", len(clusters), successCnt)
		if failCnt > 0 {
			status = 2
			auditDetail = fmt.Sprintf("%s, 失败=%d", auditDetail, failCnt)
		}
		s.writeProjectAuditLog(ctx, 0, 0, 0, operator, "批量同步", "Workspace", "QUOTA_SYNC_ALL", auditDetail, status)
	}

	if failCnt > 0 {
		return fmt.Errorf("部分集群工作空间资源同步失败: 成功=%d, 失败=%d", successCnt, failCnt)
	}

	return nil
}
