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
// 如果 K8s 中不存在，则设置默认值并创建 K8s 资源
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

// syncResourceQuotaFromK8s 从 K8s 读取 ResourceQuota 并同步到数据库，返回是否有变更
func (s *ClusterResourceSync) syncResourceQuotaFromK8s(ctx context.Context, k8sClient cluster.Client, cluster *model.OnecCluster, workspace *model.OnecProjectWorkspace, operator string) (bool, error) {
	namespace := workspace.Namespace
	resourceQuotaName := ResourceQuotaNamePrefix + namespace

	s.Logger.WithContext(ctx).Infof("从 K8s 读取 ResourceQuota: %s/%s", namespace, resourceQuotaName)
	// 获取 NamespaceOperator
	resourceQuotaOp := k8sClient.ResourceQuota()

	// 从 K8s 读取 ResourceQuota
	allocated, err := resourceQuotaOp.GetAllocated(namespace, resourceQuotaName)
	if err != nil {
		// K8s 中不存在 ResourceQuota，设置默认值并创建
		s.Logger.WithContext(ctx).Infof("K8s 中不存在 ResourceQuota，创建默认资源: %s", resourceQuotaName)

		// 设置默认值
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

		// 在 K8s 中创建 ResourceQuota
		//request := s.buildResourceQuotaRequestFromDefaults(workspace, resourceQuotaName)

		//if err := resourceQuotaOp.CreateOrUpdateResourceQuota(request); err != nil {
		//}

	}

	// 检查是否有变更
	changed := workspace.CpuAllocated != allocated.CPUAllocated ||
		workspace.MemAllocated != allocated.MemoryAllocated ||
		workspace.StorageAllocated != allocated.StorageAllocated ||
		workspace.GpuAllocated != allocated.GPUAllocated ||
		workspace.PodsAllocated != allocated.PodsAllocated

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

	s.Logger.WithContext(ctx).Infof("ResourceQuota 同步成功: %s", resourceQuotaName)
	return changed, nil
}

// buildResourceQuotaRequestFromDefaults 从默认值构建 ResourceQuota 请求
func (s *ClusterResourceSync) buildResourceQuotaRequestFromDefaults(workspace *model.OnecProjectWorkspace, resourceQuotaName string) *types.ResourceQuotaRequest {
	return &types.ResourceQuotaRequest{
		Name:      resourceQuotaName,
		Namespace: workspace.Namespace,
		Labels: map[string]string{
			"managed-by": "ikubeops",
		},
		Annotations: map[string]string{
			"workspace-id":         fmt.Sprintf("%d", workspace.Id),
			"project-cluster-id":   fmt.Sprintf("%d", workspace.ProjectClusterId),
			"ikubeops.com/managed": "true",
		},
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

// syncLimitRangeFromK8s 从 K8s 读取 LimitRange 并同步到数据库，返回是否有变更
func (s *ClusterResourceSync) syncLimitRangeFromK8s(ctx context.Context, k8sClient cluster.Client, cluster *model.OnecCluster, workspace *model.OnecProjectWorkspace, operator string) (bool, error) {
	namespace := workspace.Namespace
	limitRangeName := LimitRangeNamePrefix + namespace

	s.Logger.WithContext(ctx).Infof("从 K8s 读取 LimitRange: %s/%s", namespace, limitRangeName)

	// 获取 NamespaceOperator
	limitRangeOp := k8sClient.LimitRange()

	// 从 K8s 读取 LimitRange
	limits, err := limitRangeOp.GetLimits(namespace, limitRangeName)
	if err != nil {
		// K8s 中不存在 LimitRange，设置默认值并创建
		s.Logger.WithContext(ctx).Infof("K8s 中不存在 LimitRange，创建默认资源: %s", limitRangeName)

		// 设置默认值
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
			ContainerDefaultCPU:                     "0",
			ContainerDefaultMemory:                  "0Mi",
			ContainerDefaultEphemeralStorage:        "0Gi",
			ContainerDefaultRequestCPU:              "0",
			ContainerDefaultRequestMemory:           "0Mi",
			ContainerDefaultRequestEphemeralStorage: "0Mi",
		}

		// 在 K8s 中创建 LimitRange
		//request := s.buildLimitRangeRequestFromDefaults(workspace, limitRangeName)
	}

	// 检查是否有变更
	changed := workspace.PodMaxCpu != limits.PodMaxCPU ||
		workspace.PodMaxMemory != limits.PodMaxMemory ||
		workspace.ContainerMaxCpu != limits.ContainerMaxCPU ||
		workspace.ContainerMaxMemory != limits.ContainerMaxMemory

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

	s.Logger.WithContext(ctx).Infof("LimitRange 同步成功: %s", limitRangeName)
	return changed, nil
}

// buildLimitRangeRequestFromDefaults 从默认值构建 LimitRange 请求
func (s *ClusterResourceSync) buildLimitRangeRequestFromDefaults(workspace *model.OnecProjectWorkspace, limitRangeName string) *types.LimitRangeRequest {
	return &types.LimitRangeRequest{
		Name:      limitRangeName,
		Namespace: workspace.Namespace,
		Labels: map[string]string{
			"managed-by": "ikubeops",
		},
		Annotations: map[string]string{
			"workspace-id":         fmt.Sprintf("%d", workspace.Id),
			"project-cluster-id":   fmt.Sprintf("%d", workspace.ProjectClusterId),
			"ikubeops.com/managed": "true",
		},
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
		ContainerDefaultCPU:                     "0",
		ContainerDefaultMemory:                  "0Mi",
		ContainerDefaultEphemeralStorage:        "0Gi",
		ContainerDefaultRequestCPU:              "0",
		ContainerDefaultRequestMemory:           "0Mi",
		ContainerDefaultRequestEphemeralStorage: "0Mi",
	}
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
