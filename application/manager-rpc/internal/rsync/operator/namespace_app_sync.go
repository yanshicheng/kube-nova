package operator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
)

const (
	// ApplicationNameAnnotation 应用名称注解key
	ApplicationNameAnnotation = "ikubeops.com/application-name-en"
)

// 支持的资源类型常量（统一小写）
const (
	ResourceTypeDeployment  = "deployment"
	ResourceTypeStatefulSet = "statefulset"
	ResourceTypeDaemonSet   = "daemonset"
	ResourceTypeCronJob     = "cronjob"
)

// ==================== 应用资源同步 ====================

// SyncClusterApplications 同步某个集群的所有应用资源
func (s *ClusterResourceSync) SyncClusterApplications(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步集群应用资源, clusterUuid: %s, operator: %s", clusterUuid, operator)

	// 验证集群是否存在
	onecCluster, err := s.ClusterModel.FindOneByUuid(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询集群失败, clusterUuid: %s, error: %v", clusterUuid, err)
		return fmt.Errorf("查询集群失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("集群信息: name=%s, uuid=%s", onecCluster.Name, onecCluster.Uuid)

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

	// 4. 并发处理每个 Namespace
	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		successCnt int
		failCnt    int
		newCnt     int
		updateCnt  int
		deleteCnt  int
		syncErrors []string
		semaphore  = make(chan struct{}, MaxNamespaceConcurrency)
	)

	for i := range nsList {
		wg.Add(1)
		go func(ns string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			s.Logger.WithContext(ctx).Infof("开始同步 Namespace 应用资源: %s", ns)

			syncNamespaceApplications, update, d, err := s.SyncNamespaceApplications(ctx, clusterUuid, ns, operator, false) // 批量同步时不记录每个命名空间的日志

			mu.Lock()
			if err != nil {
				failCnt++
				errMsg := fmt.Sprintf("NS[%s]", ns)
				syncErrors = append(syncErrors, errMsg)
				s.Logger.WithContext(ctx).Errorf("同步 Namespace[%s] 应用资源失败: %v", ns, err)
			} else {
				successCnt++
				newCnt += syncNamespaceApplications
				updateCnt += update
				deleteCnt += d
				s.Logger.WithContext(ctx).Infof("Namespace 应用资源同步成功: %s", ns)
			}
			mu.Unlock()
		}(nsList[i].Name)
	}

	wg.Wait()

	s.Logger.WithContext(ctx).Infof("集群应用资源同步完成: NS总数=%d, 成功=%d, 失败=%d, 新增=%d, 更新=%d, 删除=%d",
		len(nsList), successCnt, failCnt, newCnt, updateCnt, deleteCnt)

	// 记录审计日志
	if enableAudit {
		status := int64(1)
		auditDetail := fmt.Sprintf("集群[%s]应用同步: NS总数=%d, 成功=%d, 新增App=%d, 更新=%d, 删除=%d",
			onecCluster.Name, len(nsList), successCnt, newCnt, updateCnt, deleteCnt)
		if failCnt > 0 {
			status = 2
			auditDetail = fmt.Sprintf("%s, 失败=%d", auditDetail, failCnt)
		}

		// 获取集群所属的项目集群绑定
		projectClusters, _ := s.ProjectClusterResourceModel.SearchNoPage(ctx, "", false, "cluster_uuid = ?", clusterUuid)
		for _, pc := range projectClusters {
			s.writeProjectAuditLog(ctx, 0, 0, pc.Id, operator, onecCluster.Name, "Application", "SYNC", auditDetail, status)
		}
	}

	return nil
}

// SyncNamespaceApplications 同步某个 Namespace 的应用资源，返回新增、更新、删除数量
func (s *ClusterResourceSync) SyncNamespaceApplications(ctx context.Context, clusterUuid string, namespace string, operator string, enableAudit bool) (int, int, int, error) {
	s.Logger.WithContext(ctx).Infof("开始同步 Namespace 应用资源, clusterUuid: %s, namespace: %s, operator: %s", clusterUuid, namespace, operator)

	// 1. 查询数据库中的 Workspace 记录
	query := "cluster_uuid = ? AND namespace = ?"
	workspaces, err := s.ProjectWorkspaceModel.SearchNoPage(ctx, "", false, query, clusterUuid, namespace)
	if err != nil || len(workspaces) == 0 {
		s.Logger.WithContext(ctx).Errorf("查询 Workspace 失败或不存在, namespace: %s", namespace)
		return 0, 0, 0, fmt.Errorf("查询 Workspace 失败或不存在")
	}

	workspace := workspaces[0]

	// 2. 获取 K8s 客户端
	k8sClient, err := s.K8sManager.GetCluster(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("获取K8s客户端失败: %v", err)
		return 0, 0, 0, fmt.Errorf("获取K8s客户端失败: %v", err)
	}

	// 3. 并发同步各类资源
	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		totalNew    int
		totalUpdate int
		totalDelete int
		syncErrors  []string
	)

	// 同步 Deployment
	wg.Add(1)
	go func() {
		defer wg.Done()
		syncDeployments, update, d, err := s.syncDeployments(ctx, k8sClient, workspace, operator)
		mu.Lock()
		totalNew += syncDeployments
		totalUpdate += update
		totalDelete += d
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("Deployment失败: %v", err))
		}
		mu.Unlock()
	}()

	// 同步 StatefulSet
	wg.Add(1)
	go func() {
		defer wg.Done()
		sets, update, d, err := s.syncStatefulSets(ctx, k8sClient, workspace, operator)
		mu.Lock()
		totalNew += sets
		totalUpdate += update
		totalDelete += d
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("StatefulSet失败: %v", err))
		}
		mu.Unlock()
	}()

	// 同步 DaemonSet
	wg.Add(1)
	go func() {
		defer wg.Done()
		syncDaemonSets, update, d, err := s.syncDaemonSets(ctx, k8sClient, workspace, operator)
		mu.Lock()
		totalNew += syncDaemonSets
		totalUpdate += update
		totalDelete += d
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("DaemonSet失败: %v", err))
		}
		mu.Unlock()
	}()

	// 同步 CronJob
	wg.Add(1)
	go func() {
		defer wg.Done()
		syncCronJobs, update, d, err := s.syncCronJobs(ctx, k8sClient, workspace, operator)
		mu.Lock()
		totalNew += syncCronJobs
		totalUpdate += update
		totalDelete += d
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("CronJob失败: %v", err))
		}
		mu.Unlock()
	}()

	wg.Wait()

	// 记录审计日志
	if enableAudit && (totalNew > 0 || totalUpdate > 0 || totalDelete > 0) {
		status := int64(1)
		auditDetail := fmt.Sprintf("NS[%s]应用同步: 新增=%d, 更新=%d, 删除=%d",
			namespace, totalNew, totalUpdate, totalDelete)
		if len(syncErrors) > 0 {
			status = 2
		}

		s.writeProjectAuditLog(ctx, workspace.Id, 0, workspace.ProjectClusterId, operator,
			namespace, "Application", "SYNC", auditDetail, status)
	}

	if len(syncErrors) > 0 {
		return totalNew, totalUpdate, totalDelete, fmt.Errorf("部分资源同步失败: %v", syncErrors)
	}

	s.Logger.WithContext(ctx).Infof("Namespace 应用资源同步完成: %s, 新增=%d, 更新=%d, 删除=%d",
		namespace, totalNew, totalUpdate, totalDelete)

	return totalNew, totalUpdate, totalDelete, nil
}

// syncDeployments 同步 Deployment 资源，返回新增、更新、删除数量
func (s *ClusterResourceSync) syncDeployments(ctx context.Context, k8sClient cluster.Client, workspace *model.OnecProjectWorkspace, operator string) (int, int, int, error) {
	namespace := workspace.Namespace

	// 获取所有 Deployment
	deployments, err := k8sClient.Deployment().ListAll(namespace)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询 Deployment 列表失败: %v", err)
		return 0, 0, 0, fmt.Errorf("查询 Deployment 列表失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("查询到 %d 个 Deployment 资源", len(deployments))

	// 并发处理每个 Deployment
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		newCnt    int
		updateCnt int
		semaphore = make(chan struct{}, MaxResourceConcurrency)
	)

	for i := range deployments {
		wg.Add(1)
		go func(deploy *appsv1.Deployment) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			isNew, err := s.syncSingleResource(ctx, workspace, deploy.Name, ResourceTypeDeployment, deploy.Labels, deploy.Annotations, operator)
			mu.Lock()
			if err == nil {
				if isNew {
					newCnt++
				} else {
					updateCnt++
				}
			} else {
				s.Logger.WithContext(ctx).Errorf("同步 Deployment[%s] 失败: %v", deploy.Name, err)
			}
			mu.Unlock()
		}(&deployments[i])
	}

	wg.Wait()

	// 检查数据库中存在但 K8s 不存在的 Deployment
	deleteCnt, err := s.checkMissingResources(ctx, workspace.Id, ResourceTypeDeployment, deployments)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("检查缺失的 Deployment 失败: %v", err)
	}

	return newCnt, updateCnt, deleteCnt, nil
}

// syncStatefulSets 同步 StatefulSet 资源，返回新增、更新、删除数量
func (s *ClusterResourceSync) syncStatefulSets(ctx context.Context, k8sClient cluster.Client, workspace *model.OnecProjectWorkspace, operator string) (int, int, int, error) {
	namespace := workspace.Namespace

	// 获取所有 StatefulSet
	statefulSets, err := k8sClient.StatefulSet().ListAll(namespace)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询 StatefulSet 列表失败: %v", err)
		return 0, 0, 0, fmt.Errorf("查询 StatefulSet 列表失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("查询到 %d 个 StatefulSet 资源", len(statefulSets))

	// 并发处理每个 StatefulSet
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		newCnt    int
		updateCnt int
		semaphore = make(chan struct{}, MaxResourceConcurrency)
	)

	for i := range statefulSets {
		wg.Add(1)
		go func(sts *appsv1.StatefulSet) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			isNew, err := s.syncSingleResource(ctx, workspace, sts.Name, ResourceTypeStatefulSet, sts.Labels, sts.Annotations, operator)
			mu.Lock()
			if err == nil {
				if isNew {
					newCnt++
				} else {
					updateCnt++
				}
			} else {
				s.Logger.WithContext(ctx).Errorf("同步 StatefulSet[%s] 失败: %v", sts.Name, err)
			}
			mu.Unlock()
		}(&statefulSets[i])
	}

	wg.Wait()

	// 检查数据库中存在但 K8s 不存在的 StatefulSet
	deleteCnt, err := s.checkMissingResources(ctx, workspace.Id, ResourceTypeStatefulSet, statefulSets)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("检查缺失的 StatefulSet 失败: %v", err)
	}

	return newCnt, updateCnt, deleteCnt, nil
}

// syncDaemonSets 同步 DaemonSet 资源，返回新增、更新、删除数量
func (s *ClusterResourceSync) syncDaemonSets(ctx context.Context, k8sClient cluster.Client, workspace *model.OnecProjectWorkspace, operator string) (int, int, int, error) {
	namespace := workspace.Namespace

	// 获取所有 DaemonSet
	daemonSets, err := k8sClient.DaemonSet().ListAll(namespace)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询 DaemonSet 列表失败: %v", err)
		return 0, 0, 0, fmt.Errorf("查询 DaemonSet 列表失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("查询到 %d 个 DaemonSet 资源", len(daemonSets))

	// 并发处理每个 DaemonSet
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		newCnt    int
		updateCnt int
		semaphore = make(chan struct{}, MaxResourceConcurrency)
	)

	for i := range daemonSets {
		wg.Add(1)
		go func(ds *appsv1.DaemonSet) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			isNew, err := s.syncSingleResource(ctx, workspace, ds.Name, ResourceTypeDaemonSet, ds.Labels, ds.Annotations, operator)
			mu.Lock()
			if err == nil {
				if isNew {
					newCnt++
				} else {
					updateCnt++
				}
			} else {
				s.Logger.WithContext(ctx).Errorf("同步 DaemonSet[%s] 失败: %v", ds.Name, err)
			}
			mu.Unlock()
		}(&daemonSets[i])
	}

	wg.Wait()

	// 检查数据库中存在但 K8s 不存在的 DaemonSet
	deleteCnt, err := s.checkMissingResources(ctx, workspace.Id, ResourceTypeDaemonSet, daemonSets)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("检查缺失的 DaemonSet 失败: %v", err)
	}

	return newCnt, updateCnt, deleteCnt, nil
}

// syncCronJobs 同步 CronJob 资源，返回新增、更新、删除数量
func (s *ClusterResourceSync) syncCronJobs(ctx context.Context, k8sClient cluster.Client, workspace *model.OnecProjectWorkspace, operator string) (int, int, int, error) {
	namespace := workspace.Namespace

	// 获取所有 CronJob
	cronJobs, err := k8sClient.CronJob().ListAll(namespace)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询 CronJob 列表失败: %v", err)
		return 0, 0, 0, fmt.Errorf("查询 CronJob 列表失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("查询到 %d 个 CronJob 资源", len(cronJobs))

	// 并发处理每个 CronJob
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		newCnt    int
		updateCnt int
		semaphore = make(chan struct{}, MaxResourceConcurrency)
	)

	for i := range cronJobs {
		wg.Add(1)
		go func(cj *batchv1.CronJob) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			isNew, err := s.syncSingleResource(ctx, workspace, cj.Name, ResourceTypeCronJob, cj.Labels, cj.Annotations, operator)
			mu.Lock()
			if err == nil {
				if isNew {
					newCnt++
				} else {
					updateCnt++
				}
			} else {
				s.Logger.WithContext(ctx).Errorf("同步 CronJob[%s] 失败: %v", cj.Name, err)
			}
			mu.Unlock()
		}(&cronJobs[i])
	}

	wg.Wait()

	// 检查数据库中存在但 K8s 不存在的 CronJob
	deleteCnt, err := s.checkMissingResources(ctx, workspace.Id, ResourceTypeCronJob, cronJobs)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("检查缺失的 CronJob 失败: %v", err)
	}

	return newCnt, updateCnt, deleteCnt, nil
}

// syncSingleResource 同步单个资源，返回是否为新建
func (s *ClusterResourceSync) syncSingleResource(
	ctx context.Context,
	workspace *model.OnecProjectWorkspace,
	resourceName string,
	resourceType string,
	labels map[string]string,
	annotations map[string]string,
	operator string,
) (bool, error) {
	resourceType = strings.ToLower(resourceType)

	// 1. 识别 Flagger 资源
	flaggerInfo := s.identifyFlaggerResource(resourceName, annotations, labels)

	// 2. 确定正确的应用名
	correctAppName := flaggerInfo.OriginalAppName
	if annotations != nil && annotations[ApplicationNameAnnotation] != "" {
		correctAppName = annotations[ApplicationNameAnnotation]
	}

	s.Logger.WithContext(ctx).Infof(
		"处理资源: name=%s, type=%s, isFlagger=%v, role=%s, correctAppName=%s, parentName=%s",
		resourceName, resourceType, flaggerInfo.IsFlaggerManaged,
		flaggerInfo.VersionRole, correctAppName, flaggerInfo.ParentName,
	)

	// 3. 查找或创建正确的应用
	correctApp, err := s.findOrCreateApplication(ctx, workspace, correctAppName, resourceType, operator)
	if err != nil {
		return false, fmt.Errorf("查找或创建应用失败: %v", err)
	}

	// 4. === 关键：查找该资源的版本记录（可能在任何应用下） ===
	// 先在正确的应用下查找
	versionQuery := "application_id = ? AND resource_name = ?"
	versions, err := s.ProjectApplicationVersion.SearchNoPage(
		ctx, "", false, versionQuery, correctApp.Id, resourceName,
	)

	var existingVersion *model.OnecProjectVersion
	var needMigrate = false

	if err == nil && len(versions) > 0 {
		existingVersion = versions[0]
	} else {
		allVersionsQuery := "resource_name = ?"
		allVersions, err := s.ProjectApplicationVersion.SearchNoPage(
			ctx, "", false, allVersionsQuery, resourceName,
		)

		if err == nil && len(allVersions) > 0 {
			for _, v := range allVersions {
				oldApp, err := s.ProjectApplication.FindOne(ctx, v.ApplicationId)
				if err != nil {
					continue
				}

				if oldApp.WorkspaceId == workspace.Id && oldApp.ResourceType == resourceType {
					existingVersion = v

					// 检查是否在错误的应用下
					if v.ApplicationId != correctApp.Id {
						needMigrate = true
						s.Logger.WithContext(ctx).Errorf(
							"发现历史数据问题: 版本[%s]在错误的应用[%s]下，应该在[%s]下",
							resourceName, oldApp.NameEn, correctApp.NameEn,
						)
					}
					break
				}
			}
		}
	}

	// 6. === 如果需要迁移，执行迁移操作 ===
	if needMigrate && existingVersion != nil {
		oldAppId := existingVersion.ApplicationId

		s.Logger.WithContext(ctx).Infof(
			"开始迁移版本: versionId=%d, 从应用ID=%d 迁移到 应用ID=%d",
			existingVersion.Id, oldAppId, correctApp.Id,
		)

		// 更新版本记录
		existingVersion.ApplicationId = correctApp.Id
		existingVersion.VersionRole = flaggerInfo.VersionRole
		existingVersion.ParentAppName = ""
		if flaggerInfo.IsFlaggerManaged {
			existingVersion.ParentAppName = flaggerInfo.ParentName
		}
		existingVersion.Status = 1
		existingVersion.UpdatedAt = time.Now()
		existingVersion.UpdatedBy = operator

		if err := s.ProjectApplicationVersion.Update(ctx, existingVersion); err != nil {
			return false, fmt.Errorf("迁移版本失败: %v", err)
		}

		s.Logger.WithContext(ctx).Infof("版本迁移成功: versionId=%d", existingVersion.Id)

		// 7. === 检查旧应用是否变空，如果空了就删除 ===
		s.cleanupEmptyApplication(ctx, oldAppId, operator)

		return false, nil
	}

	// 8. 版本存在且在正确的应用下，更新信息
	if existingVersion != nil {
		changed := false

		if existingVersion.Status != 1 {
			existingVersion.Status = 1
			changed = true
		}

		// 更新 Flagger 相关字段
		if flaggerInfo.IsFlaggerManaged {
			if existingVersion.VersionRole != flaggerInfo.VersionRole {
				s.Logger.WithContext(ctx).Infof("更新版本角色: %s -> %s",
					existingVersion.VersionRole, flaggerInfo.VersionRole)
				existingVersion.VersionRole = flaggerInfo.VersionRole
				changed = true
			}

			if existingVersion.ParentAppName != flaggerInfo.ParentName {
				existingVersion.ParentAppName = flaggerInfo.ParentName
				changed = true
			}
		} else {
			// 非 Flagger 资源
			if existingVersion.VersionRole != VersionRoleStable {
				existingVersion.VersionRole = VersionRoleStable
				changed = true
			}
			if existingVersion.ParentAppName != "" {
				existingVersion.ParentAppName = ""
				changed = true
			}
		}

		if changed {
			existingVersion.UpdatedAt = time.Now()
			existingVersion.UpdatedBy = operator
			if err := s.ProjectApplicationVersion.Update(ctx, existingVersion); err != nil {
				return false, fmt.Errorf("更新版本失败: %v", err)
			}
			s.Logger.WithContext(ctx).Infof("版本已更新: versionId=%d", existingVersion.Id)
		}

		return false, nil
	}

	// 9. 版本不存在，创建新版本
	s.Logger.WithContext(ctx).Infof("创建新版本: resource=%s, role=%s", resourceName, flaggerInfo.VersionRole)

	newVersion := &model.OnecProjectVersion{
		ApplicationId: correctApp.Id,
		Version:       resourceName,
		VersionRole:   flaggerInfo.VersionRole,
		ResourceName:  resourceName,
		ParentAppName: "",
		Status:        1,
		CreatedBy:     operator,
		UpdatedBy:     operator,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		IsDeleted:     0,
	}

	if flaggerInfo.IsFlaggerManaged {
		newVersion.ParentAppName = flaggerInfo.ParentName
	}

	result, err := s.ProjectApplicationVersion.Insert(ctx, newVersion)
	if err != nil {
		return false, fmt.Errorf("创建版本失败: %v", err)
	}

	versionId, _ := result.LastInsertId()
	s.Logger.WithContext(ctx).Infof("版本创建成功: versionId=%d, role=%s", versionId, flaggerInfo.VersionRole)

	return true, nil
}

// findOrCreateApplication 查找或创建应用
func (s *ClusterResourceSync) findOrCreateApplication(
	ctx context.Context,
	workspace *model.OnecProjectWorkspace,
	appNameEn string,
	resourceType string,
	operator string,
) (*model.OnecProjectApplication, error) {
	// 查找应用
	app, err := s.ProjectApplication.FindOneByWorkspaceIdNameEnResourceType(
		ctx, workspace.Id, appNameEn, resourceType,
	)

	if err == nil {
		return app, nil
	}

	// 应用不存在，创建
	if errors.Is(err, model.ErrNotFound) {
		s.Logger.WithContext(ctx).Infof("应用不存在，创建: %s", appNameEn)

		app = &model.OnecProjectApplication{
			WorkspaceId:  workspace.Id,
			NameCn:       appNameEn,
			NameEn:       appNameEn,
			ResourceType: resourceType,
			Description:  fmt.Sprintf("从集群同步的%s资源", resourceType),
			CreatedBy:    operator,
			UpdatedBy:    operator,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
			IsDeleted:    0,
		}

		result, err := s.ProjectApplication.Insert(ctx, app)
		if err != nil {
			// 处理并发创建 - 这里已有处理，改为 Info 级别
			if strings.Contains(err.Error(), "Duplicate") {
				s.Logger.WithContext(ctx).Infof("应用已存在（并发创建），重新查询: %s", appNameEn) // ← 改为 Infof
				app, err = s.ProjectApplication.FindOneByWorkspaceIdNameEnResourceType(
					ctx, workspace.Id, appNameEn, resourceType,
				)
				if err == nil {
					return app, nil
				}
			}
			return nil, fmt.Errorf("创建应用失败: %v", err)
		}

		appId, _ := result.LastInsertId()
		app.Id = uint64(appId)
		s.Logger.WithContext(ctx).Infof("应用创建成功: appId=%d", app.Id)
		return app, nil
	}

	return nil, fmt.Errorf("查询应用失败: %v", err)
}

// cleanupEmptyApplication 清理空应用（新增）
func (s *ClusterResourceSync) cleanupEmptyApplication(
	ctx context.Context,
	appId uint64,
	operator string,
) {
	// 检查该应用下是否还有版本
	versionQuery := "application_id = ? AND is_deleted = 0"
	versions, err := s.ProjectApplicationVersion.SearchNoPage(ctx, "", false, versionQuery, appId)

	if err != nil || len(versions) > 0 {
		// 还有版本，不删除
		return
	}

	// 应用已经空了，软删除
	app, err := s.ProjectApplication.FindOne(ctx, appId)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询应用失败: appId=%d, error=%v", appId, err)
		return
	}

	s.Logger.WithContext(ctx).Infof("清理空应用: appId=%d, name=%s", appId, app.NameEn)

	app.IsDeleted = 1
	app.UpdatedAt = time.Now()
	app.UpdatedBy = operator

	if err := s.ProjectApplication.Update(ctx, app); err != nil {
		s.Logger.WithContext(ctx).Errorf("删除空应用失败: appId=%d, error=%v", appId, err)
	} else {
		s.Logger.WithContext(ctx).Infof("空应用已删除: appId=%d, name=%s", appId, app.NameEn)
	}
}

// checkMissingResources 检查数据库中存在但 K8s 不存在的资源，返回删除数量
func (s *ClusterResourceSync) checkMissingResources(ctx context.Context, workspaceId uint64, resourceType string, k8sResources interface{}) (int, error) {
	// 统一资源类型为小写
	resourceType = strings.ToLower(resourceType)

	// 构建 K8s 资源名称集合
	k8sResourceMap := make(map[string]bool)

	switch resources := k8sResources.(type) {
	case []appsv1.Deployment:
		for i := range resources {
			k8sResourceMap[resources[i].Name] = true
		}
	case []appsv1.StatefulSet:
		for i := range resources {
			k8sResourceMap[resources[i].Name] = true
		}
	case []appsv1.DaemonSet:
		for i := range resources {
			k8sResourceMap[resources[i].Name] = true
		}
	case []batchv1.CronJob:
		for i := range resources {
			k8sResourceMap[resources[i].Name] = true
		}
	}

	// 查询数据库中该类型的所有应用
	appQuery := "workspace_id = ? AND resource_type = ?"
	applications, err := s.ProjectApplication.SearchNoPage(ctx, "", false, appQuery, workspaceId, resourceType)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return 0, fmt.Errorf("查询应用列表失败: %v", err)
	}

	var mu sync.Mutex
	updateCount := 0

	for _, app := range applications {
		// 查询该应用的所有版本
		versionQuery := "application_id = ?"
		versions, err := s.ProjectApplicationVersion.SearchNoPage(ctx, "", false, versionQuery, app.Id)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			continue
		}

		for _, version := range versions {
			// 检查资源是否在 K8s 中存在
			if !k8sResourceMap[version.ResourceName] {
				// K8s 中不存在，更新状态为 0
				if version.Status != 0 {
					s.Logger.WithContext(ctx).Infof("资源在K8s中不存在，更新状态: resourceName=%s, versionId=%d", version.ResourceName, version.Id)
					version.Status = 0
					version.UpdatedAt = time.Now()
					version.UpdatedBy = "system_rsync"
					if err := s.ProjectApplicationVersion.Update(ctx, version); err != nil {
						s.Logger.WithContext(ctx).Errorf("更新版本状态失败: %v", err)
					} else {
						mu.Lock()
						updateCount++
						mu.Unlock()
					}
				}
			}
		}
	}

	s.Logger.WithContext(ctx).Infof("更新了 %d 个缺失资源的状态", updateCount)

	return updateCount, nil
}

// SyncAllClusterApplications 同步所有集群的应用资源
func (s *ClusterResourceSync) SyncAllClusterApplications(ctx context.Context, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步所有集群的应用资源, operator: %s", operator)

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

	for _, onecCluster := range clusters {
		wg.Add(1)
		go func(cluster *model.OnecCluster) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			s.Logger.WithContext(ctx).Infof("开始同步集群应用资源: id=%d, name=%s, uuid=%s", cluster.Id, cluster.Name, cluster.Uuid)

			err := s.SyncClusterApplications(ctx, cluster.Uuid, operator, false) // 批量同步时不记录每个集群的日志

			mu.Lock()
			if err != nil {
				failCnt++
				errMsg := fmt.Sprintf("集群[%s]", cluster.Name)
				syncErrors = append(syncErrors, errMsg)
				s.Logger.WithContext(ctx).Errorf("同步集群[%s, id=%d] 应用资源失败: %v", cluster.Name, cluster.Id, err)
			} else {
				successCnt++
				s.Logger.WithContext(ctx).Infof("集群应用资源同步成功: id=%d, name=%s", cluster.Id, cluster.Name)
			}
			mu.Unlock()
		}(onecCluster)
	}

	wg.Wait()

	s.Logger.WithContext(ctx).Infof("所有集群应用资源同步完成: 总数=%d, 成功=%d, 失败=%d", len(clusters), successCnt, failCnt)

	// 记录批量同步审计日志
	if enableAudit {
		status := int64(1)
		auditDetail := fmt.Sprintf("批量应用同步: 集群总数=%d, 成功=%d", len(clusters), successCnt)
		if failCnt > 0 {
			status = 2
			auditDetail = fmt.Sprintf("%s, 失败=%d", auditDetail, failCnt)
		}
		s.writeProjectAuditLog(ctx, 0, 0, 0, operator, "批量同步", "Application", "SYNC_ALL", auditDetail, status)
	}

	if failCnt > 0 {
		return fmt.Errorf("部分集群应用资源同步失败: 成功=%d, 失败=%d", successCnt, failCnt)
	}

	return nil
}
