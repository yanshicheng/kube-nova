package operator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/rsync/types"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	"github.com/yanshicheng/kube-nova/common/utils"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
)

// 支持的资源类型常量
const (
	ResourceTypeDeployment  = "deployment"
	ResourceTypeStatefulSet = "statefulset"
	ResourceTypeDaemonSet   = "daemonset"
	ResourceTypeCronJob     = "cronjob"
)

// SyncClusterApplications 同步某个集群的所有应用资源
func (s *ClusterResourceSync) SyncClusterApplications(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步集群应用资源, clusterUuid: %s, operator: %s, enableHardDelete: %v", clusterUuid, operator, types.EnableHardDelete)

	// 验证集群是否存在
	onecCluster, err := s.ClusterModel.FindOneByUuid(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询集群失败, clusterUuid: %s, error: %v", clusterUuid, err)
		return fmt.Errorf("查询集群失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("集群信息: name=%s, uuid=%s", onecCluster.Name, onecCluster.Uuid)

	// 获取 K8s 客户端
	k8sClient, err := s.K8sManager.GetCluster(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("获取K8s客户端失败, clusterUuid: %s, error: %v", clusterUuid, err)
		return fmt.Errorf("获取K8s客户端失败: %v", err)
	}

	// 创建 Flagger 辅助工具并检测 CRD
	flaggerHelper := NewFlaggerHelper(k8sClient)
	flaggerAvailable := flaggerHelper.CheckFlaggerCRD(ctx)
	s.Logger.WithContext(ctx).Infof("Flagger CRD 可用状态: %v", flaggerAvailable)

	// 查询集群所有 Namespace
	nsList, err := k8sClient.Namespaces().ListAll()
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询K8s Namespace列表失败, clusterUuid: %s, error: %v", clusterUuid, err)
		return fmt.Errorf("查询K8s Namespace列表失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("查询到 %d 个 Namespace", len(nsList))

	// 并发处理每个 Namespace
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

			syncNew, syncUpdate, syncDelete, err := s.syncNamespaceApplicationsWithFlagger(
				ctx, clusterUuid, ns, operator, false, k8sClient, flaggerHelper,
			)

			mu.Lock()
			if err != nil {
				failCnt++
				errMsg := fmt.Sprintf("NS[%s]", ns)
				syncErrors = append(syncErrors, errMsg)
				s.Logger.WithContext(ctx).Errorf("同步 Namespace[%s] 应用资源失败: %v", ns, err)
			} else {
				successCnt++
				newCnt += syncNew
				updateCnt += syncUpdate
				deleteCnt += syncDelete
				s.Logger.WithContext(ctx).Infof("Namespace 应用资源同步成功: %s", ns)
			}
			mu.Unlock()
		}(nsList[i].Name)
	}

	wg.Wait()

	// 确定删除模式描述
	deleteMode := "软删除"
	if types.EnableHardDelete {
		deleteMode = "硬删除"
	}

	s.Logger.WithContext(ctx).Infof("集群应用资源同步完成: NS总数=%d, 成功=%d, 失败=%d, 新增=%d, 更新=%d, 删除=%d(%s)",
		len(nsList), successCnt, failCnt, newCnt, updateCnt, deleteCnt, deleteMode)

	// 记录审计日志
	if enableAudit {
		status := int64(1)
		auditDetail := fmt.Sprintf("集群[%s]应用同步: NS总数=%d, 成功=%d, 新增App=%d, 更新=%d, 删除=%d(%s)",
			onecCluster.Name, len(nsList), successCnt, newCnt, updateCnt, deleteCnt, deleteMode)
		if failCnt > 0 {
			status = 2
			auditDetail = fmt.Sprintf("%s, 失败=%d", auditDetail, failCnt)
		}

		projectClusters, _ := s.ProjectClusterResourceModel.SearchNoPage(ctx, "", false, "cluster_uuid = ?", clusterUuid)
		for _, pc := range projectClusters {
			s.writeProjectAuditLog(ctx, 0, 0, pc.Id, operator, onecCluster.Name, "Application", "SYNC", auditDetail, status)
		}
	}

	return nil
}

// SyncNamespaceApplications 同步某个 Namespace 的应用资源（公开方法）
func (s *ClusterResourceSync) SyncNamespaceApplications(ctx context.Context, clusterUuid string, namespace string, operator string, enableAudit bool) (int, int, int, error) {
	// 获取 K8s 客户端
	k8sClient, err := s.K8sManager.GetCluster(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("获取K8s客户端失败: %v", err)
		return 0, 0, 0, fmt.Errorf("获取K8s客户端失败: %v", err)
	}

	// 创建 Flagger 辅助工具并检测 CRD
	flaggerHelper := NewFlaggerHelper(k8sClient)
	flaggerHelper.CheckFlaggerCRD(ctx)

	return s.syncNamespaceApplicationsWithFlagger(ctx, clusterUuid, namespace, operator, enableAudit, k8sClient, flaggerHelper)
}

// syncNamespaceApplicationsWithFlagger 内部方法：使用 FlaggerHelper 同步 Namespace 应用
func (s *ClusterResourceSync) syncNamespaceApplicationsWithFlagger(
	ctx context.Context,
	clusterUuid string,
	namespace string,
	operator string,
	enableAudit bool,
	k8sClient cluster.Client,
	flaggerHelper *FlaggerHelper,
) (int, int, int, error) {
	s.Logger.WithContext(ctx).Infof("开始同步 Namespace 应用资源, clusterUuid: %s, namespace: %s, enableHardDelete: %v",
		clusterUuid, namespace, types.EnableHardDelete)

	// 1. 查询数据库中的 Workspace 记录
	query := "cluster_uuid = ? AND namespace = ?"
	workspaces, err := s.ProjectWorkspaceModel.SearchNoPage(ctx, "", false, query, clusterUuid, namespace)
	if err != nil || len(workspaces) == 0 {
		s.Logger.WithContext(ctx).Infof("Workspace 不存在, 跳过 namespace: %s", namespace)
		return 0, 0, 0, nil // 不存在 workspace 不算错误，跳过
	}

	workspace := workspaces[0]

	// 2. 如果 Flagger 可用，加载该命名空间的 Canary 策略
	if flaggerHelper.IsFlaggerAvailable() {
		if err := flaggerHelper.LoadNamespaceCanaries(ctx, namespace); err != nil {
			s.Logger.WithContext(ctx).Infof("加载 Flagger Canary 失败, namespace: %s, error: %v", namespace, err)
			// 继续执行，不影响普通资源同步
		}
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
		newCnt, updateCnt, deleteCnt, err := s.syncDeploymentsWithFlagger(ctx, k8sClient, workspace, operator, flaggerHelper)
		mu.Lock()
		totalNew += newCnt
		totalUpdate += updateCnt
		totalDelete += deleteCnt
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("Deployment失败: %v", err))
		}
		mu.Unlock()
	}()

	// 同步 StatefulSet
	wg.Add(1)
	go func() {
		defer wg.Done()
		newCnt, updateCnt, deleteCnt, err := s.syncStatefulSetsWithFlagger(ctx, k8sClient, workspace, operator, flaggerHelper)
		mu.Lock()
		totalNew += newCnt
		totalUpdate += updateCnt
		totalDelete += deleteCnt
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("StatefulSet失败: %v", err))
		}
		mu.Unlock()
	}()

	// 同步 DaemonSet
	wg.Add(1)
	go func() {
		defer wg.Done()
		newCnt, updateCnt, deleteCnt, err := s.syncDaemonSetsWithFlagger(ctx, k8sClient, workspace, operator, flaggerHelper)
		mu.Lock()
		totalNew += newCnt
		totalUpdate += updateCnt
		totalDelete += deleteCnt
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("DaemonSet失败: %v", err))
		}
		mu.Unlock()
	}()

	// 同步 CronJob（CronJob 不受 Flagger 管理，但保持接口一致）
	wg.Add(1)
	go func() {
		defer wg.Done()
		newCnt, updateCnt, deleteCnt, err := s.syncCronJobsWithFlagger(ctx, k8sClient, workspace, operator, flaggerHelper)
		mu.Lock()
		totalNew += newCnt
		totalUpdate += updateCnt
		totalDelete += deleteCnt
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("CronJob失败: %v", err))
		}
		mu.Unlock()
	}()

	wg.Wait()

	// 记录审计日志
	if enableAudit && (totalNew > 0 || totalUpdate > 0 || totalDelete > 0) {
		status := int64(1)
		deleteMode := "软删除"
		if types.EnableHardDelete {
			deleteMode = "硬删除"
		}
		auditDetail := fmt.Sprintf("NS[%s]应用同步: 新增=%d, 更新=%d, 删除=%d(%s)",
			namespace, totalNew, totalUpdate, totalDelete, deleteMode)
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

// syncDeploymentsWithFlagger 同步 Deployment 资源
func (s *ClusterResourceSync) syncDeploymentsWithFlagger(
	ctx context.Context,
	k8sClient cluster.Client,
	workspace *model.OnecProjectWorkspace,
	operator string,
	flaggerHelper *FlaggerHelper,
) (int, int, int, error) {
	namespace := workspace.Namespace

	deployments, err := k8sClient.Deployment().ListAll(namespace)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询 Deployment 列表失败: %v", err)
		return 0, 0, 0, fmt.Errorf("查询 Deployment 列表失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("查询到 %d 个 Deployment 资源", len(deployments))

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

			isNew, err := s.syncSingleResourceWithFlagger(
				ctx, workspace, deploy.Name, ResourceTypeDeployment,
				deploy.Labels, deploy.Annotations, operator, flaggerHelper,
			)
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

	// 检查并删除缺失资源
	deleteCnt, err := s.checkMissingResourcesGeneric(ctx, workspace.Id, ResourceTypeDeployment, deployments, operator)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("检查缺失的 Deployment 失败: %v", err)
	}

	return newCnt, updateCnt, deleteCnt, nil
}

// syncStatefulSetsWithFlagger 同步 StatefulSet 资源
func (s *ClusterResourceSync) syncStatefulSetsWithFlagger(
	ctx context.Context,
	k8sClient cluster.Client,
	workspace *model.OnecProjectWorkspace,
	operator string,
	flaggerHelper *FlaggerHelper,
) (int, int, int, error) {
	namespace := workspace.Namespace

	statefulSets, err := k8sClient.StatefulSet().ListAll(namespace)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询 StatefulSet 列表失败: %v", err)
		return 0, 0, 0, fmt.Errorf("查询 StatefulSet 列表失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("查询到 %d 个 StatefulSet 资源", len(statefulSets))

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

			isNew, err := s.syncSingleResourceWithFlagger(
				ctx, workspace, sts.Name, ResourceTypeStatefulSet,
				sts.Labels, sts.Annotations, operator, flaggerHelper,
			)
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

	deleteCnt, err := s.checkMissingResourcesGeneric(ctx, workspace.Id, ResourceTypeStatefulSet, statefulSets, operator)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("检查缺失的 StatefulSet 失败: %v", err)
	}

	return newCnt, updateCnt, deleteCnt, nil
}

// syncDaemonSetsWithFlagger 同步 DaemonSet 资源
func (s *ClusterResourceSync) syncDaemonSetsWithFlagger(
	ctx context.Context,
	k8sClient cluster.Client,
	workspace *model.OnecProjectWorkspace,
	operator string,
	flaggerHelper *FlaggerHelper,
) (int, int, int, error) {
	namespace := workspace.Namespace

	daemonSets, err := k8sClient.DaemonSet().ListAll(namespace)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询 DaemonSet 列表失败: %v", err)
		return 0, 0, 0, fmt.Errorf("查询 DaemonSet 列表失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("查询到 %d 个 DaemonSet 资源", len(daemonSets))

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

			isNew, err := s.syncSingleResourceWithFlagger(
				ctx, workspace, ds.Name, ResourceTypeDaemonSet,
				ds.Labels, ds.Annotations, operator, flaggerHelper,
			)
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

	deleteCnt, err := s.checkMissingResourcesGeneric(ctx, workspace.Id, ResourceTypeDaemonSet, daemonSets, operator)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("检查缺失的 DaemonSet 失败: %v", err)
	}

	return newCnt, updateCnt, deleteCnt, nil
}

// syncCronJobsWithFlagger 同步 CronJob 资源
func (s *ClusterResourceSync) syncCronJobsWithFlagger(
	ctx context.Context,
	k8sClient cluster.Client,
	workspace *model.OnecProjectWorkspace,
	operator string,
	flaggerHelper *FlaggerHelper,
) (int, int, int, error) {
	namespace := workspace.Namespace

	cronJobs, err := k8sClient.CronJob().ListAll(namespace)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询 CronJob 列表失败: %v", err)
		return 0, 0, 0, fmt.Errorf("查询 CronJob 列表失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("查询到 %d 个 CronJob 资源", len(cronJobs))

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

			// CronJob 不受 Flagger 管理，但保持接口一致
			isNew, err := s.syncSingleResourceWithFlagger(
				ctx, workspace, cj.Name, ResourceTypeCronJob,
				cj.Labels, cj.Annotations, operator, flaggerHelper,
			)
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

	deleteCnt, err := s.checkMissingResourcesGeneric(ctx, workspace.Id, ResourceTypeCronJob, cronJobs, operator)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("检查缺失的 CronJob 失败: %v", err)
	}

	return newCnt, updateCnt, deleteCnt, nil
}

// syncSingleResourceWithFlagger 同步单个资源（使用 FlaggerHelper）
// 返回值: isNew 表示是否为新创建的记录
func (s *ClusterResourceSync) syncSingleResourceWithFlagger(
	ctx context.Context,
	workspace *model.OnecProjectWorkspace,
	resourceName string,
	resourceType string,
	labels map[string]string,
	annotations map[string]string,
	operator string,
	flaggerHelper *FlaggerHelper,
) (bool, error) {
	resourceType = strings.ToLower(resourceType)

	// 将 resourceType 转换为 Kind 格式用于 Flagger 查询
	resourceKind := resourceTypeToKind(resourceType)

	// 1. 使用 FlaggerHelper 识别资源
	flaggerInfo := flaggerHelper.IdentifyResource(resourceName, resourceKind, labels, annotations)

	// 2. 确定应用名称
	// 优先使用注解中指定的名称，否则使用 Flagger 识别的原始名称
	appName := flaggerInfo.OriginalAppName
	if annotations != nil && annotations[utils.AnnotationApplicationName] != "" {
		appName = annotations[utils.AnnotationApplicationName]
	}

	s.Logger.WithContext(ctx).Debugf(
		"处理资源: name=%s, type=%s, isFlagger=%v, versionRole=%s, appName=%s",
		resourceName, resourceType, flaggerInfo.IsFlaggerManaged,
		flaggerInfo.VersionRole, appName,
	)

	// 3. 查找或创建应用（包含软删除恢复逻辑）
	app, isAppNew, err := s.ensureApplicationExists(ctx, workspace, appName, resourceType, operator)
	if err != nil {
		return false, fmt.Errorf("确保应用存在失败: %v", err)
	}

	// 4. 查找或创建版本（包含软删除恢复和迁移逻辑）
	isVersionNew, err := s.ensureVersionExists(ctx, workspace, app, resourceName, resourceType, flaggerInfo.VersionRole, operator)
	if err != nil {
		return false, fmt.Errorf("确保版本存在失败: %v", err)
	}

	// 如果应用或版本是新创建的，返回 true
	return isAppNew || isVersionNew, nil
}

// ensureApplicationExists 确保应用存在（包含软删除恢复逻辑）
// 返回值: app, isNew, error
func (s *ClusterResourceSync) ensureApplicationExists(
	ctx context.Context,
	workspace *model.OnecProjectWorkspace,
	appNameEn string,
	resourceType string,
	operator string,
) (*model.OnecProjectApplication, bool, error) {

	// 使用包含软删除的查询方法
	app, err := s.ProjectApplication.FindOneByWorkspaceIdNameEnResourceTypeIncludeDeleted(
		ctx, workspace.Id, appNameEn, resourceType,
	)

	if err == nil {
		// 应用存在
		if app.IsDeleted == 1 {
			// 软删除状态，恢复它
			s.Logger.WithContext(ctx).Infof("恢复软删除的应用: id=%d, name=%s", app.Id, appNameEn)
			if err := s.ProjectApplication.RestoreSoftDeleted(ctx, app.Id, operator); err != nil {
				return nil, false, fmt.Errorf("恢复应用失败: %v", err)
			}

			// 重新查询恢复后的应用
			app, err = s.ProjectApplication.FindOneByWorkspaceIdNameEnResourceType(
				ctx, workspace.Id, appNameEn, resourceType,
			)
			if err != nil {
				return nil, false, fmt.Errorf("查询恢复后的应用失败: %v", err)
			}

			return app, false, nil // 恢复不算新建
		}

		// 应用存在且未删除，直接返回
		return app, false, nil
	}

	// 应用不存在，创建新应用
	if errors.Is(err, model.ErrNotFound) {
		s.Logger.WithContext(ctx).Infof("创建新应用: %s, type=%s", appNameEn, resourceType)

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
			// 处理并发创建导致的重复键错误
			if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "1062") {
				s.Logger.WithContext(ctx).Infof("应用已存在（并发创建），重新查询: %s", appNameEn)

				// 重新查询（包含软删除）
				app, err = s.ProjectApplication.FindOneByWorkspaceIdNameEnResourceTypeIncludeDeleted(
					ctx, workspace.Id, appNameEn, resourceType,
				)
				if err != nil {
					return nil, false, fmt.Errorf("并发创建后查询应用失败: %v", err)
				}

				// 如果查到的是软删除状态，恢复它
				if app.IsDeleted == 1 {
					if err := s.ProjectApplication.RestoreSoftDeleted(ctx, app.Id, operator); err != nil {
						return nil, false, fmt.Errorf("恢复并发创建的应用失败: %v", err)
					}
					app.IsDeleted = 0
				}

				return app, false, nil
			}
			return nil, false, fmt.Errorf("创建应用失败: %v", err)
		}

		appId, _ := result.LastInsertId()
		app.Id = uint64(appId)
		s.Logger.WithContext(ctx).Infof("应用创建成功: appId=%d", app.Id)
		return app, true, nil
	}

	return nil, false, fmt.Errorf("查询应用失败: %v", err)
}

// ensureVersionExists 确保版本存在（包含软删除恢复和迁移逻辑）
// 返回值: isNew, error
func (s *ClusterResourceSync) ensureVersionExists(
	ctx context.Context,
	workspace *model.OnecProjectWorkspace,
	correctApp *model.OnecProjectApplication,
	resourceName string,
	resourceType string,
	versionRole string,
	operator string,
) (bool, error) {

	// 1. 先在正确的应用下查找（包含软删除）
	version, err := s.ProjectApplicationVersion.FindOneByApplicationIdResourceNameIncludeDeleted(
		ctx, correctApp.Id, resourceName,
	)

	if err == nil {
		// 版本存在于正确的应用下
		if version.IsDeleted == 1 {
			// 软删除状态，恢复它
			s.Logger.WithContext(ctx).Infof("恢复软删除的版本: id=%d, resourceName=%s", version.Id, resourceName)
			if err := s.ProjectApplicationVersion.RestoreSoftDeleted(ctx, version.Id, operator); err != nil {
				return false, fmt.Errorf("恢复版本失败: %v", err)
			}

			// 恢复后检查是否需要更新其他字段
			if version.VersionRole != versionRole {
				version.VersionRole = versionRole
				version.UpdatedAt = time.Now()
				version.UpdatedBy = operator
				if err := s.ProjectApplicationVersion.Update(ctx, version); err != nil {
					s.Logger.WithContext(ctx).Errorf("更新恢复后的版本角色失败: %v", err)
				}
			}

			return false, nil // 恢复不算新建
		}

		// 版本存在且未删除，检查是否需要更新
		changed := false

		if version.Status != 1 {
			version.Status = 1
			changed = true
		}

		if version.VersionRole != versionRole {
			s.Logger.WithContext(ctx).Infof("更新版本角色: %s -> %s", version.VersionRole, versionRole)
			version.VersionRole = versionRole
			changed = true
		}

		// 清空不再使用的 ParentAppName
		if version.ParentAppName != "" {
			version.ParentAppName = ""
			changed = true
		}

		if changed {
			version.UpdatedAt = time.Now()
			version.UpdatedBy = operator
			if err := s.ProjectApplicationVersion.Update(ctx, version); err != nil {
				return false, fmt.Errorf("更新版本失败: %v", err)
			}
			s.Logger.WithContext(ctx).Debugf("版本已更新: versionId=%d", version.Id)
		}

		return false, nil
	}

	// 2. 在正确应用下找不到，检查是否在其他应用下（需要迁移）
	if errors.Is(err, model.ErrNotFound) {
		// 在所有应用中搜索该 resourceName
		migratedVersion, oldAppId, needMigrate := s.findVersionInOtherApps(ctx, workspace.Id, resourceType, resourceName)

		if needMigrate && migratedVersion != nil {
			// 需要迁移
			s.Logger.WithContext(ctx).Infof(
				"开始迁移版本: versionId=%d, 从应用ID=%d 迁移到 应用ID=%d",
				migratedVersion.Id, oldAppId, correctApp.Id,
			)

			// 如果版本是软删除状态，先恢复
			if migratedVersion.IsDeleted == 1 {
				if err := s.ProjectApplicationVersion.RestoreSoftDeleted(ctx, migratedVersion.Id, operator); err != nil {
					return false, fmt.Errorf("恢复待迁移版本失败: %v", err)
				}
			}

			migratedVersion.ApplicationId = correctApp.Id
			migratedVersion.VersionRole = versionRole
			migratedVersion.ParentAppName = ""
			migratedVersion.Status = 1
			migratedVersion.UpdatedAt = time.Now()
			migratedVersion.UpdatedBy = operator

			if err := s.ProjectApplicationVersion.Update(ctx, migratedVersion); err != nil {
				return false, fmt.Errorf("迁移版本失败: %v", err)
			}

			s.Logger.WithContext(ctx).Infof("版本迁移成功: versionId=%d", migratedVersion.Id)

			// 清理旧的空应用
			s.cleanupEmptyApplicationWithDeleteMode(ctx, oldAppId, operator)

			return false, nil
		}

		// 3. 完全不存在，创建新版本
		s.Logger.WithContext(ctx).Infof("创建新版本: resource=%s, role=%s", resourceName, versionRole)

		newVersion := &model.OnecProjectVersion{
			ApplicationId: correctApp.Id,
			Version:       resourceName,
			VersionRole:   versionRole,
			ResourceName:  resourceName,
			ParentAppName: "",
			Status:        1,
			CreatedBy:     operator,
			UpdatedBy:     operator,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			IsDeleted:     0,
		}

		result, err := s.ProjectApplicationVersion.Insert(ctx, newVersion)
		if err != nil {
			// 处理并发创建
			if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "1062") {
				s.Logger.WithContext(ctx).Infof("版本已存在（并发创建）: %s", resourceName)
				return false, nil
			}
			return false, fmt.Errorf("创建版本失败: %v", err)
		}

		versionId, _ := result.LastInsertId()
		s.Logger.WithContext(ctx).Infof("版本创建成功: versionId=%d, role=%s", versionId, versionRole)

		return true, nil
	}

	return false, fmt.Errorf("查询版本失败: %v", err)
}

// findVersionInOtherApps 在其他应用中查找版本（用于迁移场景）
// 返回: version, oldAppId, needMigrate
func (s *ClusterResourceSync) findVersionInOtherApps(
	ctx context.Context,
	workspaceId uint64,
	resourceType string,
	resourceName string,
) (*model.OnecProjectVersion, uint64, bool) {

	// 查询该 workspace 下该类型的所有应用
	appQuery := "workspace_id = ? AND resource_type = ?"
	apps, err := s.ProjectApplication.SearchNoPage(ctx, "", false, appQuery, workspaceId, resourceType)
	if err != nil || len(apps) == 0 {
		return nil, 0, false
	}

	// 遍历每个应用查找版本
	for _, app := range apps {
		version, err := s.ProjectApplicationVersion.FindOneByApplicationIdResourceNameIncludeDeleted(
			ctx, app.Id, resourceName,
		)
		if err == nil {
			// 找到了
			return version, app.Id, true
		}
	}

	return nil, 0, false
}

// checkMissingResourcesGeneric 检查并删除数据库中存在但 K8s 不存在的资源
// 根据 types.EnableHardDelete 控制软删除还是硬删除
// 删除版本后会检查并清理空应用
func (s *ClusterResourceSync) checkMissingResourcesGeneric(
	ctx context.Context,
	workspaceId uint64,
	resourceType string,
	k8sResources interface{},
	operator string,
) (int, error) {
	resourceType = strings.ToLower(resourceType)

	// 1. 构建 K8s 资源名称 Map
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

	// 2. 查询数据库中该 workspace 该类型的所有应用
	appQuery := "workspace_id = ? AND resource_type = ?"
	applications, err := s.ProjectApplication.SearchNoPage(ctx, "", false, appQuery, workspaceId, resourceType)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return 0, fmt.Errorf("查询应用列表失败: %v", err)
	}

	deleteMode := "软删除"
	if types.EnableHardDelete {
		deleteMode = "硬删除"
	}

	var (
		deletedVersionCount int
		appsToCheck         = make(map[uint64]bool) // 记录需要检查是否为空的应用
	)

	// 3. 遍历每个应用，检查其下的版本
	for _, app := range applications {
		// 查询该应用下的所有版本（只查未删除的）
		versionQuery := "application_id = ?"
		versions, err := s.ProjectApplicationVersion.SearchNoPage(ctx, "", false, versionQuery, app.Id)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			s.Logger.WithContext(ctx).Errorf("查询应用[%d]的版本列表失败: %v", app.Id, err)
			continue
		}

		for _, version := range versions {
			// 检查该版本对应的资源是否在 K8s 中存在
			if !k8sResourceMap[version.ResourceName] {
				// K8s 中不存在该资源，执行删除
				s.Logger.WithContext(ctx).Infof(
					"资源在K8s中不存在，执行%s: resourceName=%s, versionId=%d, appId=%d",
					deleteMode, version.ResourceName, version.Id, app.Id,
				)

				if types.EnableHardDelete {
					// 硬删除版本
					if err := s.ProjectApplicationVersion.Delete(ctx, version.Id); err != nil {
						s.Logger.WithContext(ctx).Errorf("硬删除版本失败: versionId=%d, error=%v", version.Id, err)
						continue
					}
					s.Logger.WithContext(ctx).Infof("硬删除版本成功: versionId=%d, resourceName=%s", version.Id, version.ResourceName)
				} else {
					// 软删除版本
					if err := s.ProjectApplicationVersion.DeleteSoft(ctx, version.Id); err != nil {
						s.Logger.WithContext(ctx).Errorf("软删除版本失败: versionId=%d, error=%v", version.Id, err)
						continue
					}
					s.Logger.WithContext(ctx).Infof("软删除版本成功: versionId=%d, resourceName=%s", version.Id, version.ResourceName)
				}

				deletedVersionCount++
				// 记录该应用需要检查是否为空
				appsToCheck[app.Id] = true
			}
		}
	}

	// 4. 检查并清理空应用
	for appId := range appsToCheck {
		s.cleanupEmptyApplicationWithDeleteMode(ctx, appId, operator)
	}

	s.Logger.WithContext(ctx).Infof("检查缺失资源完成: type=%s, %s了 %d 个版本", resourceType, deleteMode, deletedVersionCount)

	return deletedVersionCount, nil
}

// cleanupEmptyApplicationWithDeleteMode 清理空应用（根据 types.EnableHardDelete 控制删除模式）
func (s *ClusterResourceSync) cleanupEmptyApplicationWithDeleteMode(
	ctx context.Context,
	appId uint64,
	operator string,
) {
	// 查询该应用下是否还有未删除的版本
	remainingVersions, err := s.ProjectApplicationVersion.FindAllByApplicationId(ctx, appId)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		s.Logger.WithContext(ctx).Errorf("查询应用[%d]剩余版本失败: %v", appId, err)
		return
	}

	// 如果还有版本，不删除应用
	if len(remainingVersions) > 0 {
		return
	}

	// 应用下没有版本了，根据删除模式处理
	app, err := s.ProjectApplication.FindOne(ctx, appId)
	if err != nil {
		// 可能应用已经被删除了
		s.Logger.WithContext(ctx).Debugf("查询应用失败(可能已删除): appId=%d, error=%v", appId, err)
		return
	}

	deleteMode := "软删除"
	if types.EnableHardDelete {
		deleteMode = "硬删除"
	}

	s.Logger.WithContext(ctx).Infof("清理空应用(%s): appId=%d, name=%s", deleteMode, appId, app.NameEn)

	if types.EnableHardDelete {
		// 硬删除应用
		if err := s.ProjectApplication.Delete(ctx, appId); err != nil {
			s.Logger.WithContext(ctx).Errorf("硬删除空应用失败: appId=%d, error=%v", appId, err)
		} else {
			s.Logger.WithContext(ctx).Infof("硬删除空应用成功: appId=%d, name=%s", appId, app.NameEn)
		}
	} else {
		// 软删除应用
		if err := s.ProjectApplication.DeleteSoft(ctx, appId); err != nil {
			s.Logger.WithContext(ctx).Errorf("软删除空应用失败: appId=%d, error=%v", appId, err)
		} else {
			s.Logger.WithContext(ctx).Infof("软删除空应用成功: appId=%d, name=%s", appId, app.NameEn)
		}
	}
}

// cleanupEmptyApplication 清理空应用（保持向后兼容，使用 types.EnableHardDelete 控制）
// 已弃用：请使用 cleanupEmptyApplicationWithDeleteMode
func (s *ClusterResourceSync) cleanupEmptyApplication(
	ctx context.Context,
	appId uint64,
	operator string,
) {
	s.cleanupEmptyApplicationWithDeleteMode(ctx, appId, operator)
}

// resourceTypeToKind 将资源类型转换为 Kind
func resourceTypeToKind(resourceType string) string {
	switch strings.ToLower(resourceType) {
	case ResourceTypeDeployment:
		return "Deployment"
	case ResourceTypeStatefulSet:
		return "StatefulSet"
	case ResourceTypeDaemonSet:
		return "DaemonSet"
	case ResourceTypeCronJob:
		return "CronJob"
	default:
		return resourceType
	}
}

// SyncAllClusterApplications 同步所有集群的应用资源
func (s *ClusterResourceSync) SyncAllClusterApplications(ctx context.Context, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步所有集群的应用资源, operator: %s, enableHardDelete: %v", operator, types.EnableHardDelete)

	clusters, err := s.ClusterModel.GetAllClusters(ctx)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询集群列表失败: %v", err)
		return fmt.Errorf("查询集群列表失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("共查询到 %d 个集群", len(clusters))

	if len(clusters) == 0 {
		return nil
	}

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

			err := s.SyncClusterApplications(ctx, cluster.Uuid, operator, false)

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

	deleteMode := "软删除"
	if types.EnableHardDelete {
		deleteMode = "硬删除"
	}

	s.Logger.WithContext(ctx).Infof("所有集群应用资源同步完成: 总数=%d, 成功=%d, 失败=%d, 删除模式=%s",
		len(clusters), successCnt, failCnt, deleteMode)

	if enableAudit {
		status := int64(1)
		auditDetail := fmt.Sprintf("批量应用同步: 集群总数=%d, 成功=%d, 删除模式=%s", len(clusters), successCnt, deleteMode)
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
