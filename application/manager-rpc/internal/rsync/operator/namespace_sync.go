package operator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	corev1 "k8s.io/api/core/v1"
)

const (
	// ProjectUUIDAnnotation 项目UUID注解key
	ProjectUUIDAnnotation = "ikubeops.com/project-uuid"
	// DefaultProjectID 默认项目ID（系统项目）
	DefaultProjectID uint64 = 3
)

// ==================== 1. Namespace 资源同步 ====================

// SyncClusterNamespaces 同步某个集群的所有 Namespace 资源
func (s *ClusterResourceSync) SyncClusterNamespaces(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步集群 Namespace 资源, clusterUuid: %s, operator: %s", clusterUuid, operator)

	// 1. 验证集群是否存在
	cluster, err := s.ClusterModel.FindOneByUuid(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询集群失败, clusterUuid: %s, error: %v", clusterUuid, err)
		return fmt.Errorf("查询集群失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("集群信息: name=%s, uuid=%s", cluster.Name, cluster.Uuid)

	// 2. 获取 K8s 客户端
	k8sClient, err := s.K8sManager.GetCluster(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("获取K8s客户端失败, clusterUuid: %s, error: %v", clusterUuid, err)
		return fmt.Errorf("获取K8s客户端失败: %v", err)
	}

	// 3. 查询集群所有 Namespace
	nsList, err := k8sClient.Namespaces().ListAll()
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询K8s Namespace列表失败, clusterUuid: %s, error: %v", clusterUuid, err)
		return fmt.Errorf("查询K8s Namespace列表失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("查询到 %d 个 Namespace", len(nsList))

	// 4. 并发遍历每个 Namespace 进行同步
	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		successCnt int
		failCnt    int
		newCnt     int
		updateCnt  int
		syncErrors []string
		semaphore  = make(chan struct{}, MaxNamespaceConcurrency)
	)

	for i := range nsList {
		wg.Add(1)
		go func(ns *corev1.Namespace) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			s.Logger.WithContext(ctx).Infof("开始处理 Namespace: %s", ns.Name)

			isNew, err := s.syncSingleNamespace(ctx, cluster, ns, operator)

			mu.Lock()
			if err != nil {
				failCnt++
				errMsg := fmt.Sprintf("NS[%s]同步失败", ns.Name)
				syncErrors = append(syncErrors, errMsg)
				s.Logger.WithContext(ctx).Errorf("同步 Namespace[%s] 失败: %v", ns.Name, err)
			} else {
				successCnt++
				if isNew {
					newCnt++
				} else {
					updateCnt++
				}
				s.Logger.WithContext(ctx).Infof("Namespace 同步成功: %s", ns.Name)
			}
			mu.Unlock()
		}(&nsList[i])
	}

	wg.Wait()

	// 5. 检查数据库中存在但 K8s 不存在的 Namespace
	deletedCnt, err := s.checkAndUpdateMissingNamespaces(ctx, clusterUuid, nsList)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("检查缺失的 Namespace 失败: %v", err)
		syncErrors = append(syncErrors, fmt.Sprintf("检查缺失NS失败: %v", err))
	}

	s.Logger.WithContext(ctx).Infof("集群 Namespace 同步完成: 总数=%d, 成功=%d, 失败=%d, 新增=%d, 更新=%d, 删除=%d",
		len(nsList), successCnt, failCnt, newCnt, updateCnt, deletedCnt)

	// 6. 记录审计日志
	if enableAudit {
		status := int64(1)
		auditDetail := fmt.Sprintf("NS同步: 总数=%d, 成功=%d, 新增=%d, 更新=%d, 删除=%d",
			len(nsList), successCnt, newCnt, updateCnt, deletedCnt)
		if failCnt > 0 {
			status = 2
			auditDetail = fmt.Sprintf("%s, 失败=%d", auditDetail, failCnt)
		}

		// 获取集群所属的项目集群绑定
		projectClusters, _ := s.ProjectClusterResourceModel.SearchNoPage(ctx, "", false, "cluster_uuid = ?", clusterUuid)
		for _, pc := range projectClusters {
			s.writeProjectAuditLog(ctx, 0, 0, pc.Id, operator, cluster.Name, "Namespace", "SYNC", auditDetail, status)
		}
	}

	if failCnt > 0 {
		s.Logger.WithContext(ctx).Infof("Namespace同步完成(部分失败): 成功=%d, 失败=%d", successCnt, failCnt)
	}

	return nil
}

// syncSingleNamespace 同步单个 Namespace，返回是否为新建
func (s *ClusterResourceSync) syncSingleNamespace(ctx context.Context, cluster *model.OnecCluster, ns *corev1.Namespace, operator string) (bool, error) {
	projectUUID, hasAnnotation := ns.Annotations[ProjectUUIDAnnotation]

	if hasAnnotation && projectUUID != "" {
		return s.handleNamespaceWithAnnotation(ctx, cluster, ns.Name, projectUUID, operator)
	}

	return s.handleNamespaceWithoutAnnotation(ctx, cluster, ns.Name, operator)
}

// handleNamespaceWithAnnotation 处理有项目注解的 Namespace
func (s *ClusterResourceSync) handleNamespaceWithAnnotation(ctx context.Context, cluster *model.OnecCluster, nsName string, projectUUID string, operator string) (bool, error) {
	s.Logger.WithContext(ctx).Infof("处理有注解的 Namespace: %s, projectUUID: %s", nsName, projectUUID)

	// 1. 查询项目是否存在
	project, err := s.ProjectModel.FindOneByUuid(ctx, projectUUID)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询项目失败, projectUUID: %s, error: %v", projectUUID, err)
		// 项目不存在，归到默认项目，同时更新注解
		return s.assignNamespaceToDefaultProject(ctx, cluster.Uuid, nsName, operator)
	}

	// 2. 检查项目是否绑定了该集群
	projectCluster, err := s.ProjectClusterResourceModel.FindOneByClusterUuidProjectId(ctx, cluster.Uuid, project.Id)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("项目未绑定集群, projectId: %d, clusterUuid: %s", project.Id, cluster.Uuid)
		// 项目未绑定集群，归到默认项目，同时更新注解
		return s.assignNamespaceToDefaultProject(ctx, cluster.Uuid, nsName, operator)
	}

	// 3. 检查 workspace 是否存在
	existingWorkspace, err := s.ProjectWorkspaceModel.FindOneByProjectClusterIdNamespace(ctx, projectCluster.Id, nsName)
	if err == nil {
		// workspace 已存在，更新状态
		if existingWorkspace.Status != 1 {
			existingWorkspace.Status = 1
			existingWorkspace.UpdatedAt = time.Now()
			existingWorkspace.UpdatedBy = operator
			if err := s.ProjectWorkspaceModel.Update(ctx, existingWorkspace); err != nil {
				s.Logger.WithContext(ctx).Errorf("更新 Workspace 状态失败: %v", err)
			}
		}
		return false, nil
	}

	// 4. workspace 不存在，创建新的
	s.Logger.WithContext(ctx).Infof("创建新的 Workspace: %s", nsName)

	err = s.createWorkspace(ctx, projectCluster.Id, cluster.Uuid, nsName, operator)
	if err != nil {
		return false, err
	}

	// 5. 删除其他项目下的该 namespace 记录（硬删除）
	if err := s.deleteNamespaceFromOtherProjects(ctx, cluster.Uuid, nsName, project.Id); err != nil {
		s.Logger.WithContext(ctx).Errorf("删除其他项目的 Namespace 记录失败: %v", err)
	}

	return true, nil
}

// handleNamespaceWithoutAnnotation 处理没有项目注解的 Namespace
func (s *ClusterResourceSync) handleNamespaceWithoutAnnotation(ctx context.Context, cluster *model.OnecCluster, nsName string, operator string) (bool, error) {
	s.Logger.WithContext(ctx).Infof("处理无注解的 Namespace: %s", nsName)

	// 1. 查询该 namespace 是否在数据库中
	query := "cluster_uuid = ? AND namespace = ?"
	workspaces, err := s.ProjectWorkspaceModel.SearchNoPage(ctx, "", false, query, cluster.Uuid, nsName)

	if err != nil || len(workspaces) == 0 {
		// 不存在任何记录，归到默认项目
		s.Logger.WithContext(ctx).Infof("Namespace 不在任何项目下，归到默认项目: %s", nsName)
		return s.assignNamespaceToDefaultProject(ctx, cluster.Uuid, nsName, operator)
	}

	if len(workspaces) == 1 {
		// 只存在于一个项目，更新状态并设置注解
		s.Logger.WithContext(ctx).Infof("Namespace 存在于一个项目下，更新注解: %s", nsName)
		workspace := workspaces[0]
		if workspace.Status != 1 {
			workspace.Status = 1
			workspace.UpdatedAt = time.Now()
			workspace.UpdatedBy = operator
			if err := s.ProjectWorkspaceModel.Update(ctx, workspace); err != nil {
				s.Logger.WithContext(ctx).Errorf("更新 Workspace 状态失败: %v", err)
			}
		}
		return false, s.updateNamespaceAnnotationForWorkspace(ctx, cluster.Uuid, nsName, workspace)
	}

	// 存在于多个项目，解决冲突
	s.Logger.WithContext(ctx).Infof("Namespace 存在于多个项目下(%d个)，保留最近创建的: %s", len(workspaces), nsName)
	return false, s.resolveMultipleProjectConflict(ctx, cluster.Uuid, nsName, workspaces, operator)
}

// assignNamespaceToDefaultProject 将 Namespace 分配到默认项目
func (s *ClusterResourceSync) assignNamespaceToDefaultProject(ctx context.Context, clusterUUID string, nsName string, operator string) (bool, error) {
	// 1. 查询默认项目
	defaultProject, err := s.ProjectModel.FindOne(ctx, DefaultProjectID)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询默认项目失败: %v", err)
		return false, fmt.Errorf("查询默认项目失败: %v", err)
	}

	// 2. 查询或创建项目集群绑定
	projectCluster, err := s.ProjectClusterResourceModel.FindOneByClusterUuidProjectId(ctx, clusterUUID, defaultProject.Id)
	if err != nil {
		s.Logger.WithContext(ctx).Infof("默认项目未绑定集群，创建绑定")
		projectCluster, err = s.createProjectClusterBinding(ctx, clusterUUID, defaultProject.Id, operator)
		if err != nil {
			return false, fmt.Errorf("创建项目集群绑定失败: %v", err)
		}
	}

	// 3. 创建 workspace
	if err := s.createWorkspace(ctx, projectCluster.Id, clusterUUID, nsName, operator); err != nil {
		return false, err
	}

	// 4. 更新 namespace 注解
	return true, s.updateNamespaceAnnotationWithUUID(ctx, clusterUUID, nsName, defaultProject.Uuid)
}

// createWorkspace 创建 Workspace 记录
func (s *ClusterResourceSync) createWorkspace(ctx context.Context, projectClusterId uint64, clusterUuid string, nsName string, operator string) error {
	// 1. 检查是否已存在
	existingWorkspace, err := s.ProjectWorkspaceModel.FindOneByProjectClusterIdNamespace(ctx, projectClusterId, nsName)
	if err == nil {
		// 已存在，更新状态
		if existingWorkspace.Status != 1 {
			existingWorkspace.Status = 1
			existingWorkspace.UpdatedAt = time.Now()
			existingWorkspace.UpdatedBy = operator
			if err := s.ProjectWorkspaceModel.Update(ctx, existingWorkspace); err != nil {
				s.Logger.WithContext(ctx).Errorf("更新已存在的 Workspace 状态失败: %v", err)
			}
		}
		s.Logger.WithContext(ctx).Infof("Workspace 已存在，已更新状态: %s", nsName)
		return nil
	}

	// 2. 创建新的 workspace
	workspace := &model.OnecProjectWorkspace{
		ProjectClusterId:                        projectClusterId,
		ClusterUuid:                             clusterUuid,
		Name:                                    nsName,
		Namespace:                               nsName,
		Description:                             fmt.Sprintf("从集群同步的命名空间: %s", nsName),
		Status:                                  1,
		IsSystem:                                0,
		AppCreateTime:                           time.Now(),
		CreatedBy:                               operator,
		UpdatedBy:                               operator,
		CreatedAt:                               time.Now(),
		UpdatedAt:                               time.Now(),
		IsDeleted:                               0,
		CpuAllocated:                            "0",
		MemAllocated:                            "0Gi",
		StorageAllocated:                        "0Gi",
		GpuAllocated:                            "0",
		PodsAllocated:                           0,
		ConfigmapAllocated:                      0,
		SecretAllocated:                         0,
		PvcAllocated:                            0,
		EphemeralStorageAllocated:               "0Gi",
		ServiceAllocated:                        0,
		LoadbalancersAllocated:                  0,
		NodeportsAllocated:                      0,
		DeploymentsAllocated:                    0,
		JobsAllocated:                           0,
		CronjobsAllocated:                       0,
		DaemonsetsAllocated:                     0,
		StatefulsetsAllocated:                   0,
		IngressesAllocated:                      0,
		PodMaxCpu:                               "0",
		PodMaxMemory:                            "0Gi",
		PodMaxEphemeralStorage:                  "0Gi",
		PodMinCpu:                               "0",
		PodMinMemory:                            "0Mi",
		PodMinEphemeralStorage:                  "0Mi",
		ContainerMaxCpu:                         "0",
		ContainerMaxMemory:                      "0Gi",
		ContainerMaxEphemeralStorage:            "0Gi",
		ContainerMinCpu:                         "0",
		ContainerMinMemory:                      "0Mi",
		ContainerMinEphemeralStorage:            "0Mi",
		ContainerDefaultCpu:                     "0",
		ContainerDefaultMemory:                  "0Mi",
		ContainerDefaultEphemeralStorage:        "0Gi",
		ContainerDefaultRequestCpu:              "0",
		ContainerDefaultRequestMemory:           "0Mi",
		ContainerDefaultRequestEphemeralStorage: "0Mi",
	}

	_, err = s.ProjectWorkspaceModel.Insert(ctx, workspace)
	if err != nil {
		// 处理并发创建导致的重复键错误
		if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "1062") {
			s.Logger.WithContext(ctx).Infof("Workspace 已存在（并发创建）: %s", nsName)
			return nil
		}
		s.Logger.WithContext(ctx).Errorf("创建 Workspace 失败: %v", err)
		return fmt.Errorf("创建 Workspace 失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("Workspace 创建成功: %s", nsName)
	return nil
}

// createProjectClusterBinding 创建项目集群绑定
// createProjectClusterBinding 创建项目集群绑定
func (s *ClusterResourceSync) createProjectClusterBinding(ctx context.Context, clusterUuid string, projectId uint64, operator string) (*model.OnecProjectCluster, error) {
	// 验证集群是否存在
	_, err := s.ClusterModel.FindOneByUuid(ctx, clusterUuid)
	if err != nil {
		return nil, fmt.Errorf("查询集群失败: %v", err)
	}

	projectCluster := &model.OnecProjectCluster{
		ProjectId:                 projectId,
		ClusterUuid:               clusterUuid,
		CreatedBy:                 operator,
		UpdatedBy:                 operator,
		CreatedAt:                 time.Now(),
		UpdatedAt:                 time.Now(),
		IsDeleted:                 0,
		CpuLimit:                  "0",
		CpuOvercommitRatio:        1.0,
		CpuCapacity:               "0",
		CpuAllocated:              "0",
		MemLimit:                  "0Gi",
		MemOvercommitRatio:        1.0,
		MemCapacity:               "0Gi",
		MemAllocated:              "0Gi",
		StorageLimit:              "0Gi",
		StorageAllocated:          "0Gi",
		GpuLimit:                  "0",
		GpuOvercommitRatio:        1.0,
		GpuCapacity:               "0",
		GpuAllocated:              "0",
		PodsLimit:                 0,
		PodsAllocated:             0,
		ConfigmapLimit:            0,
		ConfigmapAllocated:        0,
		SecretLimit:               0,
		SecretAllocated:           0,
		PvcLimit:                  0,
		PvcAllocated:              0,
		EphemeralStorageLimit:     "0Gi",
		EphemeralStorageAllocated: "0Gi",
		ServiceLimit:              0,
		ServiceAllocated:          0,
		LoadbalancersLimit:        0,
		LoadbalancersAllocated:    0,
		NodeportsLimit:            0,
		NodeportsAllocated:        0,
		DeploymentsLimit:          0,
		DeploymentsAllocated:      0,
		JobsLimit:                 0,
		JobsAllocated:             0,
		CronjobsLimit:             0,
		CronjobsAllocated:         0,
		DaemonsetsLimit:           0,
		DaemonsetsAllocated:       0,
		StatefulsetsLimit:         0,
		StatefulsetsAllocated:     0,
		IngressesLimit:            0,
		IngressesAllocated:        0,
	}

	result, err := s.ProjectClusterResourceModel.Insert(ctx, projectCluster)
	if err != nil {
		// 处理并发创建导致的重复键错误
		if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "1062") {
			s.Logger.WithContext(ctx).Infof("项目集群绑定已存在（并发创建），重新查询: projectId=%d, clusterUuid=%s", projectId, clusterUuid)
			existingBinding, findErr := s.ProjectClusterResourceModel.FindOneByClusterUuidProjectId(ctx, clusterUuid, projectId)
			if findErr != nil {
				return nil, fmt.Errorf("查询已存在的项目集群绑定失败: %v", findErr)
			}
			return existingBinding, nil
		}
		return nil, fmt.Errorf("插入项目集群绑定失败: %v", err)
	}

	insertId, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("获取插入ID失败: %v", err)
	}
	projectCluster.Id = uint64(insertId)

	s.Logger.WithContext(ctx).Infof("项目集群绑定创建成功: projectId=%d, clusterUuid=%s", projectId, clusterUuid)
	return projectCluster, nil
}

// updateNamespaceAnnotationForWorkspace 为 Workspace 更新 Namespace 注解
func (s *ClusterResourceSync) updateNamespaceAnnotationForWorkspace(ctx context.Context, clusterUUID string, nsName string, workspace *model.OnecProjectWorkspace) error {
	projectCluster, err := s.ProjectClusterResourceModel.FindOne(ctx, workspace.ProjectClusterId)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询项目集群关系失败: %v", err)
		return fmt.Errorf("查询项目集群关系失败: %v", err)
	}

	project, err := s.ProjectModel.FindOne(ctx, projectCluster.ProjectId)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询项目失败: %v", err)
		return fmt.Errorf("查询项目失败: %v", err)
	}

	return s.updateNamespaceAnnotationWithUUID(ctx, clusterUUID, nsName, project.Uuid)
}

// updateNamespaceAnnotationWithUUID 使用项目UUID更新 Namespace 注解
func (s *ClusterResourceSync) updateNamespaceAnnotationWithUUID(ctx context.Context, clusterUUID string, nsName string, projectUUID string) error {
	k8sClient, err := s.K8sManager.GetCluster(ctx, clusterUUID)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("获取K8s客户端失败: %v", err)
		return fmt.Errorf("获取K8s客户端失败: %v", err)
	}

	ns, err := k8sClient.Namespaces().Get(nsName)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("获取 Namespace 失败: %v", err)
		return fmt.Errorf("获取 Namespace 失败: %v", err)
	}

	// 检查注解是否已经正确
	if ns.Annotations != nil && ns.Annotations[ProjectUUIDAnnotation] == projectUUID {
		return nil
	}

	// 更新注解
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}
	ns.Annotations[ProjectUUIDAnnotation] = projectUUID

	_, err = k8sClient.Namespaces().Update(ns)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("更新 Namespace 注解失败: %v", err)
		return fmt.Errorf("更新 Namespace 注解失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("更新 Namespace 注解成功: %s -> %s", nsName, projectUUID)

	return nil
}

// resolveMultipleProjectConflict 解决多项目冲突
func (s *ClusterResourceSync) resolveMultipleProjectConflict(ctx context.Context, clusterUUID string, nsName string, workspaces []*model.OnecProjectWorkspace, operator string) error {
	if len(workspaces) == 0 {
		return fmt.Errorf("workspaces 列表为空")
	}

	// 按创建时间排序，保留最新的
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].CreatedAt.After(workspaces[j].CreatedAt)
	})
	latestWorkspace := workspaces[0]

	s.Logger.WithContext(ctx).Infof("保留最近创建的 Workspace: id=%d, createdAt=%s", latestWorkspace.Id, latestWorkspace.CreatedAt.Format(time.RFC3339))

	// 硬删除其他的 workspace
	for i := 1; i < len(workspaces); i++ {
		ws := workspaces[i]
		s.Logger.WithContext(ctx).Infof("硬删除冲突的 Workspace: id=%d, namespace=%s", ws.Id, ws.Namespace)
		if err := s.ProjectWorkspaceModel.Delete(ctx, ws.Id); err != nil {
			s.Logger.WithContext(ctx).Errorf("删除冲突的 Workspace 失败: id=%d, error=%v", ws.Id, err)
		}
	}

	// 更新保留的 workspace 状态
	if latestWorkspace.Status != 1 {
		latestWorkspace.Status = 1
		latestWorkspace.UpdatedAt = time.Now()
		latestWorkspace.UpdatedBy = operator
		if err := s.ProjectWorkspaceModel.Update(ctx, latestWorkspace); err != nil {
			s.Logger.WithContext(ctx).Errorf("更新 Workspace 状态失败: %v", err)
		}
	}

	// 更新 namespace 注解
	return s.updateNamespaceAnnotationForWorkspace(ctx, clusterUUID, nsName, latestWorkspace)
}

// deleteNamespaceFromOtherProjects 删除该 Namespace 在其他项目下的记录（硬删除）
func (s *ClusterResourceSync) deleteNamespaceFromOtherProjects(ctx context.Context, clusterUUID string, nsName string, keepProjectId uint64) error {
	query := "cluster_uuid = ? AND namespace = ?"
	workspaces, err := s.ProjectWorkspaceModel.SearchNoPage(ctx, "", false, query, clusterUUID, nsName)
	if err != nil || len(workspaces) == 0 {
		return nil
	}

	for _, ws := range workspaces {
		projectCluster, err := s.ProjectClusterResourceModel.FindOne(ctx, ws.ProjectClusterId)
		if err != nil {
			s.Logger.WithContext(ctx).Errorf("查询项目集群关系失败: %v", err)
			continue
		}

		// 如果不是要保留的项目，硬删除该记录
		if projectCluster.ProjectId != keepProjectId {
			s.Logger.WithContext(ctx).Infof("硬删除其他项目的 Namespace 记录: workspaceId=%d, projectId=%d", ws.Id, projectCluster.ProjectId)
			if err := s.ProjectWorkspaceModel.Delete(ctx, ws.Id); err != nil {
				s.Logger.WithContext(ctx).Errorf("删除其他项目的 Namespace 记录失败: %v", err)
			}
		}
	}

	return nil
}

// checkAndUpdateMissingNamespaces 检查并更新数据库中存在但 K8s 不存在的 Namespace，返回删除数量
func (s *ClusterResourceSync) checkAndUpdateMissingNamespaces(ctx context.Context, clusterUUID string, k8sNamespaces []corev1.Namespace) (int, error) {
	// 构建 K8s namespace 集合
	k8sNsMap := make(map[string]bool, len(k8sNamespaces))
	for i := range k8sNamespaces {
		k8sNsMap[k8sNamespaces[i].Name] = true
	}

	// 查询数据库中该集群的所有 workspace
	query := "cluster_uuid = ?"
	workspaces, err := s.ProjectWorkspaceModel.SearchNoPage(ctx, "", false, query, clusterUUID)
	if err != nil {
		return 0, fmt.Errorf("查询 Workspace 列表失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("检查缺失的 Namespace: 数据库中有 %d 个 Workspace", len(workspaces))

	var mu sync.Mutex
	updateCount := 0

	for _, ws := range workspaces {
		// 如果 K8s 中不存在该 namespace，更新状态为 0
		if !k8sNsMap[ws.Namespace] {
			if ws.Status != 0 {
				s.Logger.WithContext(ctx).Infof("Namespace 在 K8s 中不存在，更新状态: %s", ws.Namespace)
				ws.Status = 0
				ws.UpdatedAt = time.Now()
				ws.UpdatedBy = "system_rsync"
				if err := s.ProjectWorkspaceModel.Update(ctx, ws); err != nil {
					s.Logger.WithContext(ctx).Errorf("更新 Workspace 状态失败: %v", err)
				} else {
					mu.Lock()
					updateCount++
					mu.Unlock()
				}
			}
		}
	}

	s.Logger.WithContext(ctx).Infof("更新了 %d 个缺失 Namespace 的状态", updateCount)

	return updateCount, nil
}

// SyncAllClusterNamespaces 同步所有集群的 Namespace 资源
func (s *ClusterResourceSync) SyncAllClusterNamespaces(ctx context.Context, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步所有集群的 Namespace 资源, operator: %s", operator)

	// 获取所有集群
	clusters, err := s.ClusterModel.GetAllClusters(ctx)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询集群列表失败: %v", err)
		return fmt.Errorf("查询集群列表失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("共查询到 %d 个集群", len(clusters))

	if len(clusters) == 0 {
		return nil
	}

	// 并发处理所有集群
	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		successCnt int
		failCnt    int
		syncErrors []string
		semaphore  = make(chan struct{}, MaxClusterConcurrency)
	)

	for _, cluster := range clusters {
		wg.Add(1)
		go func(c *model.OnecCluster) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			s.Logger.WithContext(ctx).Infof("开始同步集群 Namespace: id=%d, name=%s, uuid=%s", c.Id, c.Name, c.Uuid)

			err := s.SyncClusterNamespaces(ctx, c.Uuid, operator, false) // 批量同步时不记录每个集群的日志

			mu.Lock()
			if err != nil {
				failCnt++
				errMsg := fmt.Sprintf("集群[%s]", c.Name)
				syncErrors = append(syncErrors, errMsg)
				s.Logger.WithContext(ctx).Errorf("同步集群[%s, id=%d] Namespace 失败: %v", c.Name, c.Id, err)
			} else {
				successCnt++
				s.Logger.WithContext(ctx).Infof("集群 Namespace 同步成功: id=%d, name=%s", c.Id, c.Name)
			}
			mu.Unlock()
		}(cluster)
	}

	wg.Wait()

	s.Logger.WithContext(ctx).Infof("所有集群 Namespace 同步完成: 总数=%d, 成功=%d, 失败=%d", len(clusters), successCnt, failCnt)

	// 记录批量同步审计日志
	if enableAudit {
		status := int64(1)
		auditDetail := fmt.Sprintf("批量NS同步: 集群总数=%d, 成功=%d", len(clusters), successCnt)
		if failCnt > 0 {
			status = 2
			auditDetail = fmt.Sprintf("%s, 失败=%d", auditDetail, failCnt)
		}
		s.writeProjectAuditLog(ctx, 0, 0, 0, operator, "批量同步", "Namespace", "SYNC_ALL", auditDetail, status)
	}

	if failCnt > 0 {
		return fmt.Errorf("部分集群 Namespace 同步失败: 成功=%d, 失败=%d", successCnt, failCnt)
	}

	return nil
}
