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

// ==================== 有注解分支

// handleNamespaceWithAnnotation 处理有项目注解的 Namespace
func (s *ClusterResourceSync) handleNamespaceWithAnnotation(ctx context.Context, cluster *model.OnecCluster, nsName string, projectUUID string, operator string) (bool, error) {
	s.Logger.WithContext(ctx).Infof("处理有注解的 Namespace: %s, projectUUID: %s", nsName, projectUUID)

	// ========== 第一层：确保项目存在 ==========
	project, err := s.ensureProjectExists(ctx, projectUUID, operator)
	if err != nil {
		return false, fmt.Errorf("确保项目存在失败: %v", err)
	}
	s.Logger.WithContext(ctx).Infof("项目已就绪: id=%d, uuid=%s, name=%s", project.Id, project.Uuid, project.Name)

	// ========== 第二层：确保项目-集群绑定存在 ==========
	projectCluster, err := s.ensureProjectClusterBindingExists(ctx, project.Id, cluster.Uuid, operator)
	if err != nil {
		return false, fmt.Errorf("确保项目-集群绑定存在失败: %v", err)
	}
	s.Logger.WithContext(ctx).Infof("项目-集群绑定已就绪: id=%d, projectId=%d, clusterUuid=%s", projectCluster.Id, projectCluster.ProjectId, projectCluster.ClusterUuid)

	// ========== 第三层：确保 Workspace 存在 ==========
	isNew, err := s.ensureWorkspaceExists(ctx, projectCluster.Id, cluster.Uuid, nsName, operator)
	if err != nil {
		return false, fmt.Errorf("确保 Workspace 存在失败: %v", err)
	}

	// ========== 清理其他项目的关联记录 ==========
	if err := s.deleteNamespaceFromOtherProjects(ctx, cluster.Uuid, nsName, project.Id); err != nil {
		s.Logger.WithContext(ctx).Errorf("删除其他项目的 Namespace 记录失败: %v", err)
		// 不返回错误，继续执行
	}

	return isNew, nil
}

// ensureProjectExists 确保项目存在（恢复软删除或创建新项目）
func (s *ClusterResourceSync) ensureProjectExists(ctx context.Context, projectUUID string, operator string) (*model.OnecProject, error) {
	// 1. 先查询项目（包含软删除的）
	project, err := s.ProjectModel.FindOneByUuidIncludeDeleted(ctx, projectUUID)

	if err == nil {
		// 项目存在
		if project.IsDeleted == 1 {
			// 软删除状态，恢复它
			s.Logger.WithContext(ctx).Infof("恢复软删除的项目: id=%d, uuid=%s", project.Id, projectUUID)
			if err := s.ProjectModel.RestoreSoftDeleted(ctx, project.Id, operator); err != nil {
				return nil, fmt.Errorf("恢复项目失败: %v", err)
			}
			project.IsDeleted = 0
		}
		return project, nil
	}

	// 2. 项目不存在，创建新项目
	s.Logger.WithContext(ctx).Infof("创建新项目: uuid=%s", projectUUID)

	// 生成项目名称：取 UUID 前 8 位作为标识
	projectName := fmt.Sprintf("自动创建项目-%s", projectUUID)
	if len(projectUUID) > 8 {
		projectName = fmt.Sprintf("自动创建项目-%s", projectUUID[:8])
	}

	newProject, err := s.ProjectModel.CreateWithUuid(ctx, projectUUID, projectName, operator)
	if err != nil {
		return nil, fmt.Errorf("创建项目失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("项目创建成功: id=%d, uuid=%s, name=%s", newProject.Id, newProject.Uuid, newProject.Name)
	return newProject, nil
}

// ensureProjectClusterBindingExists 确保项目-集群绑定存在（恢复软删除或创建新绑定）
func (s *ClusterResourceSync) ensureProjectClusterBindingExists(ctx context.Context, projectId uint64, clusterUuid string, operator string) (*model.OnecProjectCluster, error) {
	// 1. 先查询绑定（包含软删除的）
	binding, err := s.ProjectClusterResourceModel.FindOneByClusterUuidProjectIdIncludeDeleted(ctx, clusterUuid, projectId)

	if err == nil {
		// 绑定存在
		if binding.IsDeleted == 1 {
			// 软删除状态，恢复它
			s.Logger.WithContext(ctx).Infof("恢复软删除的项目-集群绑定: id=%d, projectId=%d, clusterUuid=%s", binding.Id, projectId, clusterUuid)
			if err := s.ProjectClusterResourceModel.RestoreSoftDeleted(ctx, binding.Id, operator); err != nil {
				return nil, fmt.Errorf("恢复项目-集群绑定失败: %v", err)
			}
			binding.IsDeleted = 0
		}
		return binding, nil
	}

	// 2. 绑定不存在，创建新绑定
	s.Logger.WithContext(ctx).Infof("创建项目-集群绑定: projectId=%d, clusterUuid=%s", projectId, clusterUuid)
	return s.createProjectClusterBinding(ctx, clusterUuid, projectId, operator)
}

// ensureWorkspaceExists 确保 Workspace 存在（恢复软删除或创建新 Workspace）
func (s *ClusterResourceSync) ensureWorkspaceExists(ctx context.Context, projectClusterId uint64, clusterUuid string, nsName string, operator string) (bool, error) {
	// 1. 先查询 Workspace（包含软删除的）
	workspace, err := s.ProjectWorkspaceModel.FindOneByProjectClusterIdNamespaceIncludeDeleted(ctx, projectClusterId, nsName)

	if err == nil {
		// Workspace 存在
		needUpdate := false

		if workspace.IsDeleted == 1 {
			// 软删除状态，恢复它
			s.Logger.WithContext(ctx).Infof("恢复软删除的 Workspace: id=%d, namespace=%s", workspace.Id, nsName)
			if err := s.ProjectWorkspaceModel.RestoreAndUpdateStatus(ctx, workspace.Id, 1, operator); err != nil {
				return false, fmt.Errorf("恢复 Workspace 失败: %v", err)
			}
			return false, nil // 恢复不算新建
		}

		if workspace.Status != 1 {
			workspace.Status = 1
			needUpdate = true
		}

		if needUpdate {
			workspace.UpdatedAt = time.Now()
			workspace.UpdatedBy = operator
			if err := s.ProjectWorkspaceModel.Update(ctx, workspace); err != nil {
				return false, fmt.Errorf("更新 Workspace 失败: %v", err)
			}
		}

		return false, nil // 不是新建
	}

	// 2. Workspace 不存在，创建新的
	s.Logger.WithContext(ctx).Infof("创建新的 Workspace: namespace=%s, projectClusterId=%d", nsName, projectClusterId)
	return true, s.createWorkspace(ctx, projectClusterId, clusterUuid, nsName, operator)
}

// ==================== 无注解分支 ====================

// handleNamespaceWithoutAnnotation 处理没有项目注解的 Namespace
func (s *ClusterResourceSync) handleNamespaceWithoutAnnotation(ctx context.Context, cluster *model.OnecCluster, nsName string, operator string) (bool, error) {
	s.Logger.WithContext(ctx).Infof("处理无注解的 Namespace: %s", nsName)

	// 1. 查询该 namespace 在数据库中的所有记录（包含软删除，用于判断归属）
	workspaces, err := s.ProjectWorkspaceModel.FindAllByClusterUuidNamespaceIncludeDeleted(ctx, cluster.Uuid, nsName)

	// 过滤出未删除的记录
	var activeWorkspaces []*model.OnecProjectWorkspace
	if err == nil && len(workspaces) > 0 {
		for _, ws := range workspaces {
			if ws.IsDeleted == 0 {
				activeWorkspaces = append(activeWorkspaces, ws)
			}
		}
	}

	if len(activeWorkspaces) == 0 {
		// 不存在任何有效记录，归到默认项目
		s.Logger.WithContext(ctx).Infof("Namespace 不在任何项目下，归到默认项目: %s", nsName)
		return s.assignNamespaceToDefaultProject(ctx, cluster.Uuid, nsName, operator)
	}

	if len(activeWorkspaces) == 1 {
		// 只存在于一个项目，更新状态并设置注解
		s.Logger.WithContext(ctx).Infof("Namespace 存在于一个项目下，更新注解: %s", nsName)
		workspace := activeWorkspaces[0]
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
	s.Logger.WithContext(ctx).Infof("Namespace 存在于多个项目下(%d个)，保留最近创建的: %s", len(activeWorkspaces), nsName)
	return false, s.resolveMultipleProjectConflict(ctx, cluster.Uuid, nsName, activeWorkspaces, operator)
}

// assignNamespaceToDefaultProject 将 Namespace 分配到默认项目
func (s *ClusterResourceSync) assignNamespaceToDefaultProject(ctx context.Context, clusterUUID string, nsName string, operator string) (bool, error) {
	// 1. 查询默认项目
	defaultProject, err := s.ProjectModel.FindOne(ctx, DefaultProjectID)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询默认项目失败: %v", err)
		return false, fmt.Errorf("查询默认项目失败: %v", err)
	}

	// 2. 确保项目-集群绑定存在
	projectCluster, err := s.ensureProjectClusterBindingExists(ctx, defaultProject.Id, clusterUUID, operator)
	if err != nil {
		return false, fmt.Errorf("确保项目-集群绑定存在失败: %v", err)
	}

	// 3. 确保 Workspace 存在
	isNew, err := s.ensureWorkspaceExists(ctx, projectCluster.Id, clusterUUID, nsName, operator)
	if err != nil {
		return false, err
	}

	// 4. 更新 namespace 注解
	return isNew, s.updateNamespaceAnnotationWithUUID(ctx, clusterUUID, nsName, defaultProject.Uuid)
}

// ==================== 辅助方法 ====================

// createWorkspace 创建 Workspace 记录
func (s *ClusterResourceSync) createWorkspace(ctx context.Context, projectClusterId uint64, clusterUuid string, nsName string, operator string) error {
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

	_, err := s.ProjectWorkspaceModel.Insert(ctx, workspace)
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
func (s *ClusterResourceSync) createProjectClusterBinding(ctx context.Context, clusterUuid string, projectId uint64, operator string) (*model.OnecProjectCluster, error) {
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
			existingBinding, findErr := s.ProjectClusterResourceModel.FindOneByClusterUuidProjectIdIncludeDeleted(ctx, clusterUuid, projectId)
			if findErr != nil {
				return nil, fmt.Errorf("查询已存在的项目集群绑定失败: %v", findErr)
			}
			// 如果查到的是软删除状态，恢复它
			if existingBinding.IsDeleted == 1 {
				if err := s.ProjectClusterResourceModel.RestoreSoftDeleted(ctx, existingBinding.Id, operator); err != nil {
					return nil, fmt.Errorf("恢复项目集群绑定失败: %v", err)
				}
				existingBinding.IsDeleted = 0
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
	// 查询所有记录（包含软删除的，以便彻底清理）
	workspaces, err := s.ProjectWorkspaceModel.FindAllByClusterUuidNamespaceIncludeDeleted(ctx, clusterUUID, nsName)
	if err != nil || len(workspaces) == 0 {
		return nil
	}

	for _, ws := range workspaces {
		projectCluster, err := s.ProjectClusterResourceModel.FindOne(ctx, ws.ProjectClusterId)
		if err != nil {
			// 可能是软删除的绑定，尝试查询包含软删除的
			s.Logger.WithContext(ctx).Debugf("查询项目集群关系失败(可能已删除): %v", err)
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

// checkAndUpdateMissingNamespaces 检查并更新数据库中存在但 K8s 不存在的 Namespace
func (s *ClusterResourceSync) checkAndUpdateMissingNamespaces(ctx context.Context, clusterUUID string, k8sNamespaces []corev1.Namespace) (int, error) {
	// 构建 K8s namespace 集合
	k8sNsMap := make(map[string]bool, len(k8sNamespaces))
	for i := range k8sNamespaces {
		k8sNsMap[k8sNamespaces[i].Name] = true
	}

	// 查询数据库中该集群的所有 workspace（只查未删除的）
	query := "cluster_uuid = ?"
	workspaces, err := s.ProjectWorkspaceModel.SearchNoPage(ctx, "", false, query, clusterUUID)
	if err != nil {
		return 0, fmt.Errorf("查询 Workspace 列表失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("检查缺失的 Namespace: 数据库中有 %d 个 Workspace", len(workspaces))

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
					updateCount++
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

			err := s.SyncClusterNamespaces(ctx, c.Uuid, operator, false)

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
