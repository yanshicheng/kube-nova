package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/rsync/operator"
)

const (
	// AnnotationApplicationEn 应用英文名注解
	AnnotationApplicationEn = "ikubeops.com/application-en"
	// AnnotationApplication 应用中文名注解
	AnnotationApplication = "ikubeops.com/application"
	// AnnotationVersion 版本号注解
	AnnotationVersion = "ikubeops.com/version"
	// AnnotationVersionRole 版本角色注解
	AnnotationVersionRole = "ikubeops.com/version-role"

	// DefaultVersionRole 默认版本角色
	DefaultVersionRole = "stable"
	// DefaultVersion 默认版本号
	DefaultVersion = "v1"

	// AnnotationDeletedBy API 删除标记注解 Key（用于 Workload 资源）
	AnnotationDeletedBy = "kube-nova.io/deleted-by"
	// AnnotationDeleteTime API 删除时间注解 Key
	AnnotationDeleteTime = "kube-nova.io/delete-time"
	// DeletedByAPI API 删除标记值
	DeletedByAPI = "api"

	// VersionStatusNormal 版本状态：正常
	VersionStatusNormal int64 = 1
	// VersionStatusAbnormal 版本状态：异常
	VersionStatusAbnormal int64 = 0
)

// CalculateDeploymentStatus 计算 Deployment 的健康状态
// 正常条件：ReadyReplicas >= Spec.Replicas 且 AvailableReplicas >= Spec.Replicas
func CalculateDeploymentStatus(deploy *appsv1.Deployment) int64 {
	if deploy.Spec.Replicas == nil {
		return VersionStatusNormal
	}

	desired := *deploy.Spec.Replicas

	if desired == 0 {
		return VersionStatusNormal
	}

	if deploy.Status.ReadyReplicas >= desired &&
		deploy.Status.AvailableReplicas >= desired {
		return VersionStatusNormal
	}

	return VersionStatusAbnormal
}

// CalculateStatefulSetStatus 计算 StatefulSet 的健康状态
func CalculateStatefulSetStatus(sts *appsv1.StatefulSet) int64 {
	if sts.Spec.Replicas == nil {
		return VersionStatusNormal
	}

	desired := *sts.Spec.Replicas

	if desired == 0 {
		return VersionStatusNormal
	}

	if sts.Status.ReadyReplicas >= desired {
		return VersionStatusNormal
	}

	return VersionStatusAbnormal
}

// CalculateDaemonSetStatus 计算 DaemonSet 的健康状态
func CalculateDaemonSetStatus(ds *appsv1.DaemonSet) int64 {
	desired := ds.Status.DesiredNumberScheduled

	if desired == 0 {
		return VersionStatusNormal
	}

	if ds.Status.NumberReady >= desired {
		return VersionStatusNormal
	}

	return VersionStatusAbnormal
}

// CalculateCronJobStatus 计算 CronJob 的健康状态
func CalculateCronJobStatus(cj *batchv1.CronJob) int64 {
	return VersionStatusNormal
}

// SyncResult 同步结果，用于标识是否有实际变更
type SyncResult struct {
	ApplicationCreated  bool     // 应用是否新创建
	ApplicationRestored bool     // 应用是否从软删除恢复
	ApplicationUpdated  bool     // 应用是否有更新（如 NameCn 变更）
	VersionCreated      bool     // 版本是否新创建
	VersionRestored     bool     // 版本是否从软删除恢复
	VersionUpdated      bool     // 版本是否有更新
	ChangeDetails       []string // 变更详情
}

// HasChange 判断是否有实际变更
func (r *SyncResult) HasChange() bool {
	return r.ApplicationCreated || r.ApplicationRestored || r.ApplicationUpdated ||
		r.VersionCreated || r.VersionRestored || r.VersionUpdated
}

// AddChangeDetail 添加变更详情
func (r *SyncResult) AddChangeDetail(detail string) {
	r.ChangeDetails = append(r.ChangeDetails, detail)
}

// syncWorkloadToDatabase 同步工作负载到数据库
func (h *DefaultEventHandler) syncWorkloadToDatabase(ctx context.Context, clusterUUID, namespace, resourceName, resourceType string, labels, annotations map[string]string, status int64) error {
	logger := logx.WithContext(ctx)

	// Step 1: 查询工作空间
	workspaces, err := h.svcCtx.ProjectWorkspaceModel.FindAllByClusterUuidNamespaceIncludeDeleted(ctx, clusterUUID, namespace)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("查询工作空间失败: %v", err)
	}

	var workspace *model.OnecProjectWorkspace
	for _, ws := range workspaces {
		if ws.IsDeleted == 0 {
			workspace = ws
			break
		}
	}

	if workspace == nil {
		logger.Debugf("[Workload-SYNC] 未找到对应的工作空间，跳过: ClusterUUID=%s, Namespace=%s", clusterUUID, namespace)
		return nil
	}

	// Step 2: 从注解确定应用的 NameEn 和 NameCn
	appNameEn, appNameCn := h.determineAppNames(resourceName, annotations)

	// Step 3: 确定版本角色
	versionRole := h.determineVersionRole(resourceName, labels, annotations, clusterUUID, namespace, ctx, logger)

	logger.Debugf("[Workload-SYNC] 处理资源: resourceName=%s, type=%s, appNameEn=%s, appNameCn=%s, role=%s, status=%d",
		resourceName, resourceType, appNameEn, appNameCn, versionRole, status)

	// 初始化同步结果
	syncResult := &SyncResult{}

	// Step 4: 检查版本是否存在（通过 resourceName 查找）
	existingVersion, err := h.findVersionByResourceName(ctx, workspace.Id, resourceName, resourceType)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("查询现有版本失败: %v", err)
	}

	if existingVersion != nil {
		// 版本存在，检查并更新版本信息
		logger.Debugf("[Workload-SYNC] 版本已存在: VersionID=%d, ResourceName=%s", existingVersion.Id, resourceName)

		// 获取关联的应用
		existingApp, err := h.svcCtx.ProjectApplication.FindOne(ctx, existingVersion.ApplicationId)
		if err != nil {
			return fmt.Errorf("查询版本关联的应用失败: %v", err)
		}

		// 检查并修正现有绑定
		shouldSkip, err := h.checkAndCorrectExistingBinding(ctx, existingApp, existingVersion, versionRole, annotations, status, syncResult)
		if err != nil {
			return fmt.Errorf("检查现有绑定失败: %v", err)
		}

		if shouldSkip && !syncResult.HasChange() {
			logger.Debugf("[Workload-SYNC] 版本已正确绑定，无变更，跳过: resourceName=%s", resourceName)
			return nil
		}

		// 有变更，创建审计日志
		if syncResult.HasChange() {
			logger.Infof("[Workload-SYNC] 版本信息已更新: ClusterUUID=%s, Namespace=%s, ResourceName=%s, AppNameEn=%s, AppID=%d, Changes=%v",
				clusterUUID, namespace, resourceName, existingApp.NameEn, existingApp.Id, syncResult.ChangeDetails)

			// 创建审计日志
			projectId, projectName := h.getProjectInfo(ctx, workspace.ProjectClusterId)
			actionDetail := h.buildSyncAuditDetail(resourceType, existingApp.NameEn, resourceName, status, syncResult)

			h.createAuditLog(ctx, &AuditLogInfo{
				ClusterName:     h.getClusterName(ctx, clusterUUID),
				ClusterUuid:     clusterUUID,
				ProjectId:       projectId,
				ProjectName:     projectName,
				WorkspaceId:     workspace.Id,
				WorkspaceName:   workspace.Name,
				ApplicationId:   existingApp.Id,
				ApplicationName: existingApp.NameEn,
				Title:           "应用版本同步",
				ActionDetail:    actionDetail,
				Status:          1,
			})
		}

		return nil
	}

	// Step 5: 版本不存在，检查应用是否存在（通过应用名称查找）
	application, err := h.findOrCreateApplication(ctx, workspace.Id, appNameEn, appNameCn, resourceType, syncResult)
	if err != nil {
		return fmt.Errorf("查找或创建应用失败: %v", err)
	}

	// Step 6: 创建版本
	err = h.ensureVersion(ctx, application.Id, resourceName, versionRole, annotations, status, syncResult)
	if err != nil {
		return fmt.Errorf("创建版本失败: %v", err)
	}

	// Step 7: 创建审计日志
	logger.Infof("[Workload-SYNC] 同步成功: ClusterUUID=%s, Namespace=%s, ResourceName=%s, AppNameEn=%s, AppID=%d, Status=%d, Changes=%v",
		clusterUUID, namespace, resourceName, appNameEn, application.Id, status, syncResult.ChangeDetails)

	// 创建审计日志
	projectId, projectName := h.getProjectInfo(ctx, workspace.ProjectClusterId)
	actionDetail := h.buildSyncAuditDetail(resourceType, appNameEn, resourceName, status, syncResult)

	h.createAuditLog(ctx, &AuditLogInfo{
		ClusterName:     h.getClusterName(ctx, clusterUUID),
		ClusterUuid:     clusterUUID,
		ProjectId:       projectId,
		ProjectName:     projectName,
		WorkspaceId:     workspace.Id,
		WorkspaceName:   workspace.Name,
		ApplicationId:   application.Id,
		ApplicationName: application.NameEn,
		Title:           "应用版本同步",
		ActionDetail:    actionDetail,
		Status:          1,
	})

	return nil
}

// buildSyncAuditDetail 根据同步结果构建审计日志详情
func (h *DefaultEventHandler) buildSyncAuditDetail(resourceType, appNameEn, resourceName string, status int64, syncResult *SyncResult) string {
	var actions []string

	if syncResult.ApplicationCreated {
		actions = append(actions, "创建应用")
	}
	if syncResult.ApplicationRestored {
		actions = append(actions, "恢复应用")
	}
	if syncResult.ApplicationUpdated {
		actions = append(actions, "更新应用")
	}
	if syncResult.VersionCreated {
		actions = append(actions, "创建版本")
	}
	if syncResult.VersionRestored {
		actions = append(actions, "恢复版本")
	}
	if syncResult.VersionUpdated {
		actions = append(actions, "更新版本")
	}

	actionStr := strings.Join(actions, "+")

	// 如果有详细变更信息，附加到后面
	if len(syncResult.ChangeDetails) > 0 {
		detailStr := strings.Join(syncResult.ChangeDetails, ", ")
		return fmt.Sprintf("从 K8s %s 自动同步(%s): %s/%s, 状态: %d, 详情: %s",
			resourceType, actionStr, appNameEn, resourceName, status, detailStr)
	}

	return fmt.Sprintf("从 K8s %s 自动同步(%s): %s/%s, 状态: %d", resourceType, actionStr, appNameEn, resourceName, status)
}

// updateWorkloadStatus 更新工作负载状态（用于 UPDATE 事件）
func (h *DefaultEventHandler) updateWorkloadStatus(ctx context.Context, clusterUUID, namespace, resourceName, resourceType string, newStatus int64) error {
	logger := logx.WithContext(ctx)

	// Step 1: 查询工作空间
	workspaces, err := h.svcCtx.ProjectWorkspaceModel.FindAllByClusterUuidNamespaceIncludeDeleted(ctx, clusterUUID, namespace)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("查询工作空间失败: %v", err)
	}

	var workspace *model.OnecProjectWorkspace
	for _, ws := range workspaces {
		if ws.IsDeleted == 0 {
			workspace = ws
			break
		}
	}

	if workspace == nil {
		logger.Debugf("[Workload-STATUS] 未找到对应的工作空间，跳过: ClusterUUID=%s, Namespace=%s", clusterUUID, namespace)
		return nil
	}

	// Step 2: 查找版本记录
	application, version, err := h.findApplicationAndVersionByResourceName(ctx, workspace.Id, resourceName, resourceType)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			logger.Debugf("[Workload-STATUS] 版本不存在，跳过状态更新: WorkspaceID=%d, ResourceName=%s",
				workspace.Id, resourceName)
			return nil
		}
		return fmt.Errorf("查询应用和版本失败: %v", err)
	}

	if application == nil || version == nil {
		logger.Debugf("[Workload-STATUS] 应用或版本不存在，跳过: WorkspaceID=%d, ResourceName=%s",
			workspace.Id, resourceName)
		return nil
	}

	// Step 3: 检查状态是否变化
	if version.Status == newStatus {
		logger.Debugf("[Workload-STATUS] 状态未变化，跳过: VersionID=%d, Status=%d",
			version.Id, newStatus)
		return nil
	}

	// Step 4: 更新状态
	oldStatus := version.Status
	version.Status = newStatus
	version.UpdatedBy = SystemOperator

	if err := h.svcCtx.ProjectVersion.Update(ctx, version); err != nil {
		return fmt.Errorf("更新版本状态失败: %v", err)
	}

	logger.Infof("[Workload-STATUS] 状态更新成功: VersionID=%d, ResourceName=%s, Status: %d -> %d",
		version.Id, resourceName, oldStatus, newStatus)

	return nil
}

// determineAppNames 从注解确定应用的 NameEn 和 NameCn
func (h *DefaultEventHandler) determineAppNames(resourceName string, annotations map[string]string) (appNameEn, appNameCn string) {
	appNameEn = resourceName
	appNameCn = resourceName

	if annotations == nil {
		return appNameEn, appNameCn
	}

	if nameEn, ok := annotations[AnnotationApplicationEn]; ok && nameEn != "" {
		appNameEn = nameEn
		appNameCn = nameEn
	}

	if nameCn, ok := annotations[AnnotationApplication]; ok && nameCn != "" {
		appNameCn = nameCn
	}

	return appNameEn, appNameCn
}

// determineVersionRole 确定版本角色
func (h *DefaultEventHandler) determineVersionRole(resourceName string, labels, annotations map[string]string, clusterUUID, namespace string, ctx context.Context, logger logx.Logger) string {
	if annotations != nil {
		if role, ok := annotations[AnnotationVersionRole]; ok && role != "" {
			return role
		}
	}

	var flaggerHelper *operator.FlaggerHelper
	if h.svcCtx.K8sManager != nil {
		k8sClient, err := h.svcCtx.K8sManager.GetCluster(ctx, clusterUUID)
		if err != nil {
			logger.Debugf("[Workload-SYNC] 获取 K8s 客户端失败，使用默认角色: %v", err)
		} else {
			flaggerHelper = operator.NewFlaggerHelper(k8sClient)
		}
	}

	if flaggerHelper != nil {
		flaggerAvailable := flaggerHelper.CheckFlaggerCRD(ctx)
		if flaggerAvailable {
			if err := flaggerHelper.LoadNamespaceCanaries(ctx, namespace); err != nil {
				logger.Debugf("[Workload-SYNC] 加载 Flagger Canary 失败: %v", err)
			}
			resourceKind := resourceTypeToKind(strings.ToLower(resourceName))
			flaggerInfo := flaggerHelper.IdentifyResource(resourceName, resourceKind, labels, annotations)
			if flaggerInfo != nil && flaggerInfo.VersionRole != "" {
				return flaggerInfo.VersionRole
			}
		}
	}

	if strings.HasSuffix(resourceName, "-primary") || strings.HasSuffix(resourceName, "-canary") {
		return operator.VersionRoleStable
	}

	return DefaultVersionRole
}

// deleteWorkloadFromDatabase 从数据库删除工作负载（硬删除）
func (h *DefaultEventHandler) deleteWorkloadFromDatabase(ctx context.Context, clusterUUID, namespace, resourceName, resourceType string, annotations map[string]string) error {
	logger := logx.WithContext(ctx)

	// 检查是否是 API 触发的删除
	if annotations != nil {
		if deletedBy, ok := annotations[AnnotationDeletedBy]; ok && deletedBy == DeletedByAPI {
			logger.Infof("[Workload-DELETE] 检测到 API 删除标记，跳过 Watch 处理: ClusterUUID=%s, Namespace=%s, ResourceName=%s",
				clusterUUID, namespace, resourceName)
			return nil
		}
	}

	workspaces, err := h.svcCtx.ProjectWorkspaceModel.FindAllByClusterUuidNamespaceIncludeDeleted(ctx, clusterUUID, namespace)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("查询工作空间失败: %v", err)
	}

	var workspace *model.OnecProjectWorkspace
	for _, ws := range workspaces {
		if ws.IsDeleted == 0 {
			workspace = ws
			break
		}
	}

	if workspace == nil {
		logger.Debugf("[Workload-DELETE] 未找到对应的工作空间，跳过: ClusterUUID=%s, Namespace=%s", clusterUUID, namespace)
		return nil
	}

	application, version, err := h.findApplicationAndVersionByResourceName(ctx, workspace.Id, resourceName, resourceType)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			logger.Debugf("[Workload-DELETE] 版本不存在，跳过: WorkspaceID=%d, ResourceName=%s, Type=%s",
				workspace.Id, resourceName, resourceType)
			return nil
		}
		return fmt.Errorf("查询应用和版本失败: %v", err)
	}

	if application == nil || version == nil {
		logger.Debugf("[Workload-DELETE] 应用或版本不存在，跳过: WorkspaceID=%d, ResourceName=%s",
			workspace.Id, resourceName)
		return nil
	}

	if err := h.svcCtx.ProjectVersion.Delete(ctx, version.Id); err != nil {
		return fmt.Errorf("硬删除版本失败: %v", err)
	}

	logger.Infof("[Workload-DELETE] 硬删除版本成功: VersionID=%d, AppID=%d, ResourceName=%s",
		version.Id, application.Id, resourceName)

	remainingVersions, err := h.svcCtx.ProjectVersion.FindAllByApplicationId(ctx, application.Id)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		logger.Errorf("[Workload-DELETE] 查询剩余版本失败: %v", err)
	}

	if len(remainingVersions) == 0 {
		if err := h.svcCtx.ProjectApplication.Delete(ctx, application.Id); err != nil {
			logger.Errorf("[Workload-DELETE] 硬删除应用失败: %v", err)
		} else {
			logger.Infof("[Workload-DELETE] 硬删除应用成功（无剩余版本）: AppID=%d, AppName=%s", application.Id, application.NameEn)
		}
	}

	projectId, projectName := h.getProjectInfo(ctx, workspace.ProjectClusterId)
	h.createAuditLog(ctx, &AuditLogInfo{
		ClusterName:     h.getClusterName(ctx, clusterUUID),
		ClusterUuid:     clusterUUID,
		ProjectId:       projectId,
		ProjectName:     projectName,
		WorkspaceId:     workspace.Id,
		WorkspaceName:   workspace.Name,
		ApplicationId:   application.Id,
		ApplicationName: application.NameEn,
		Title:           "应用版本删除",
		ActionDetail:    fmt.Sprintf("从 K8s %s 删除事件触发硬删除版本: %s/%s", resourceType, application.NameEn, resourceName),
		Status:          1,
	})

	return nil
}

// findApplicationAndVersionByResourceName 通过 resourceName 查找应用和版本
func (h *DefaultEventHandler) findApplicationAndVersionByResourceName(ctx context.Context, workspaceId uint64, resourceName, resourceType string) (*model.OnecProjectApplication, *model.OnecProjectVersion, error) {
	apps, err := h.svcCtx.ProjectApplication.SearchNoPage(ctx, "", false, "`workspace_id` = ? AND `resource_type` = ?", workspaceId, resourceType)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, nil, model.ErrNotFound
		}
		return nil, nil, err
	}

	for _, app := range apps {
		version, err := h.svcCtx.ProjectVersion.FindOneByApplicationIdResourceNameIncludeDeleted(ctx, app.Id, resourceName)
		if err == nil {
			return app, version, nil
		}
		if !errors.Is(err, model.ErrNotFound) {
			return nil, nil, err
		}
	}

	return nil, nil, model.ErrNotFound
}

// findVersionByResourceName 通过 resourceName 查找版本（在所有应用中搜索）
func (h *DefaultEventHandler) findVersionByResourceName(ctx context.Context, workspaceId uint64, resourceName, resourceType string) (*model.OnecProjectVersion, error) {
	apps, err := h.svcCtx.ProjectApplication.SearchNoPage(ctx, "", false, "`workspace_id` = ? AND `resource_type` = ?", workspaceId, resourceType)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}

	for _, app := range apps {
		version, err := h.svcCtx.ProjectVersion.FindOneByApplicationIdResourceNameIncludeDeleted(ctx, app.Id, resourceName)
		if err == nil {
			return version, nil
		}
		if !errors.Is(err, model.ErrNotFound) {
			return nil, err
		}
	}

	return nil, model.ErrNotFound
}

// findOrCreateApplication 查找或创建应用
func (h *DefaultEventHandler) findOrCreateApplication(ctx context.Context, workspaceId uint64, appNameEn, appNameCn, resourceType string, syncResult *SyncResult) (*model.OnecProjectApplication, error) {
	logger := logx.WithContext(ctx)

	// 首先尝试通过英文名查找应用
	application, err := h.svcCtx.ProjectApplication.FindOneByWorkspaceIdNameEnResourceTypeIncludeDeleted(ctx, workspaceId, appNameEn, resourceType)
	if err == nil {
		if application.IsDeleted == 0 {
			// 未删除，检查是否需要更新 NameCn
			if application.NameCn != appNameCn {
				oldNameCn := application.NameCn
				application.NameCn = appNameCn
				application.UpdatedBy = SystemOperator
				if err := h.svcCtx.ProjectApplication.Update(ctx, application); err != nil {
					logger.Errorf("[Application-UPDATE] 更新应用中文名失败: %v", err)
				} else {
					syncResult.ApplicationUpdated = true
					syncResult.AddChangeDetail(fmt.Sprintf("应用中文名: %s -> %s", oldNameCn, appNameCn))
					logger.Infof("[Application-UPDATE] 更新应用中文名成功: ID=%d, NameCn: %s -> %s",
						application.Id, oldNameCn, appNameCn)
				}
			}
			return application, nil
		}

		// 软删除状态，恢复
		logger.Infof("[Application-RESTORE] 恢复软删除的应用: ID=%d, NameEn=%s", application.Id, appNameEn)
		if err := h.svcCtx.ProjectApplication.RestoreSoftDeleted(ctx, application.Id, SystemOperator); err != nil {
			return nil, fmt.Errorf("恢复应用失败: %v", err)
		}

		syncResult.ApplicationRestored = true
		syncResult.AddChangeDetail("恢复软删除应用")

		application, err = h.svcCtx.ProjectApplication.FindOneByWorkspaceIdNameEnResourceType(ctx, workspaceId, appNameEn, resourceType)
		if err != nil {
			return nil, fmt.Errorf("查询恢复后的应用失败: %v", err)
		}

		if application.NameCn != appNameCn {
			application.NameCn = appNameCn
			application.UpdatedBy = SystemOperator
			if err := h.svcCtx.ProjectApplication.Update(ctx, application); err != nil {
				logger.Errorf("[Application-UPDATE] 更新恢复后的应用中文名失败: %v", err)
			}
		}

		return application, nil
	}

	if !errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("查询应用失败: %v", err)
	}

	// 应用不存在，创建新应用
	newApp := &model.OnecProjectApplication{
		WorkspaceId:  workspaceId,
		NameCn:       appNameCn,
		NameEn:       appNameEn,
		ResourceType: resourceType,
		Description:  fmt.Sprintf("从 K8s %s 自动同步", resourceType),
		CreatedBy:    SystemOperator,
		UpdatedBy:    SystemOperator,
		IsDeleted:    0,
	}

	result, err := h.svcCtx.ProjectApplication.Insert(ctx, newApp)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "1062") {
			return h.svcCtx.ProjectApplication.FindOneByWorkspaceIdNameEnResourceType(ctx, workspaceId, appNameEn, resourceType)
		}
		return nil, fmt.Errorf("创建应用失败: %v", err)
	}

	insertId, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("获取插入ID失败: %v", err)
	}
	newApp.Id = uint64(insertId)

	syncResult.ApplicationCreated = true
	syncResult.AddChangeDetail("创建新应用")

	logger.Infof("[Application-CREATE] 创建应用成功: ID=%d, WorkspaceID=%d, NameEn=%s, NameCn=%s, Type=%s",
		newApp.Id, workspaceId, appNameEn, appNameCn, resourceType)

	return newApp, nil
}

// resourceTypeToKind 将资源类型转换为 Kind 格式
func resourceTypeToKind(resourceType string) string {
	switch strings.ToLower(resourceType) {
	case "deployment":
		return "Deployment"
	case "statefulset":
		return "StatefulSet"
	case "daemonset":
		return "DaemonSet"
	case "cronjob":
		return "CronJob"
	default:
		return resourceType
	}
}

// ensureApplication 确保应用存在，不存在则创建，软删除则恢复
func (h *DefaultEventHandler) ensureApplication(ctx context.Context, workspaceId uint64, nameEn, nameCn, resourceType string, syncResult *SyncResult) (*model.OnecProjectApplication, error) {
	logger := logx.WithContext(ctx)

	application, err := h.svcCtx.ProjectApplication.FindOneByWorkspaceIdNameEnResourceTypeIncludeDeleted(ctx, workspaceId, nameEn, resourceType)
	if err == nil {
		if application.IsDeleted == 0 {
			// 未删除，检查是否需要更新 NameCn
			if application.NameCn != nameCn {
				oldNameCn := application.NameCn
				application.NameCn = nameCn
				application.UpdatedBy = SystemOperator
				if err := h.svcCtx.ProjectApplication.Update(ctx, application); err != nil {
					logger.Errorf("[Application-UPDATE] 更新应用中文名失败: %v", err)
				} else {
					// 【修复】NameCn 变更也记录
					syncResult.ApplicationUpdated = true
					syncResult.AddChangeDetail(fmt.Sprintf("应用中文名: %s -> %s", oldNameCn, nameCn))
					logger.Infof("[Application-UPDATE] 更新应用中文名成功: ID=%d, NameCn: %s -> %s",
						application.Id, oldNameCn, nameCn)
				}
			}
			return application, nil
		}

		// 软删除状态，恢复
		logger.Infof("[Application-RESTORE] 恢复软删除的应用: ID=%d, NameEn=%s", application.Id, nameEn)
		if err := h.svcCtx.ProjectApplication.RestoreSoftDeleted(ctx, application.Id, SystemOperator); err != nil {
			return nil, fmt.Errorf("恢复应用失败: %v", err)
		}

		syncResult.ApplicationRestored = true
		syncResult.AddChangeDetail("恢复软删除应用")

		application, err = h.svcCtx.ProjectApplication.FindOneByWorkspaceIdNameEnResourceType(ctx, workspaceId, nameEn, resourceType)
		if err != nil {
			return nil, fmt.Errorf("查询恢复后的应用失败: %v", err)
		}

		if application.NameCn != nameCn {
			application.NameCn = nameCn
			application.UpdatedBy = SystemOperator
			if err := h.svcCtx.ProjectApplication.Update(ctx, application); err != nil {
				logger.Errorf("[Application-UPDATE] 更新恢复后的应用中文名失败: %v", err)
			}
		}

		return application, nil
	}

	if !errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("查询应用失败: %v", err)
	}

	newApp := &model.OnecProjectApplication{
		WorkspaceId:  workspaceId,
		NameCn:       nameCn,
		NameEn:       nameEn,
		ResourceType: resourceType,
		Description:  fmt.Sprintf("从 K8s %s 自动同步", resourceType),
		CreatedBy:    SystemOperator,
		UpdatedBy:    SystemOperator,
		IsDeleted:    0,
	}

	result, err := h.svcCtx.ProjectApplication.Insert(ctx, newApp)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "1062") {
			return h.svcCtx.ProjectApplication.FindOneByWorkspaceIdNameEnResourceType(ctx, workspaceId, nameEn, resourceType)
		}
		return nil, fmt.Errorf("创建应用失败: %v", err)
	}

	insertId, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("获取插入ID失败: %v", err)
	}
	newApp.Id = uint64(insertId)

	syncResult.ApplicationCreated = true
	syncResult.AddChangeDetail("创建新应用")

	logger.Infof("[Application-CREATE] 创建应用成功: ID=%d, WorkspaceID=%d, NameEn=%s, NameCn=%s, Type=%s",
		newApp.Id, workspaceId, nameEn, nameCn, resourceType)

	return newApp, nil
}

// ensureVersion 确保版本存在，不存在则创建，软删除则恢复
func (h *DefaultEventHandler) ensureVersion(ctx context.Context, applicationId uint64, resourceName, versionRole string, annotations map[string]string, status int64, syncResult *SyncResult) error {
	logger := logx.WithContext(ctx)

	existingVersion, err := h.svcCtx.ProjectVersion.FindOneByApplicationIdResourceNameIncludeDeleted(ctx, applicationId, resourceName)
	if err == nil {
		if existingVersion.IsDeleted == 0 {
			// 未删除，检查是否需要更新
			var changes []string

			if existingVersion.Status != status {
				changes = append(changes, fmt.Sprintf("状态: %d -> %d", existingVersion.Status, status))
				existingVersion.Status = status
			}

			if existingVersion.VersionRole != versionRole {
				changes = append(changes, fmt.Sprintf("角色: %s -> %s", existingVersion.VersionRole, versionRole))
				existingVersion.VersionRole = versionRole
			}

			if annotations != nil {
				if v, ok := annotations[AnnotationVersion]; ok && v != "" && existingVersion.Version != v {
					changes = append(changes, fmt.Sprintf("版本号: %s -> %s", existingVersion.Version, v))
					existingVersion.Version = v
				}
			}

			if existingVersion.ParentAppName != "" {
				changes = append(changes, "清除ParentAppName")
				existingVersion.ParentAppName = ""
			}

			if len(changes) > 0 {
				existingVersion.UpdatedBy = SystemOperator
				if err := h.svcCtx.ProjectVersion.Update(ctx, existingVersion); err != nil {
					return fmt.Errorf("更新版本失败: %v", err)
				}
				syncResult.VersionUpdated = true
				for _, c := range changes {
					syncResult.AddChangeDetail(c)
				}
				logger.Infof("[Version-UPDATE] 版本已更新: VersionID=%d, Changes=%v", existingVersion.Id, changes)
			} else {
				logger.Debugf("[Version-NOCHANGE] 版本无变化，跳过: VersionID=%d, Role=%s, Status=%d",
					existingVersion.Id, versionRole, status)
			}
			return nil
		}

		// 软删除状态，恢复
		logger.Infof("[Version-RESTORE] 恢复软删除的版本: ID=%d, ResourceName=%s", existingVersion.Id, resourceName)
		if err := h.svcCtx.ProjectVersion.RestoreSoftDeleted(ctx, existingVersion.Id, SystemOperator); err != nil {
			return fmt.Errorf("恢复版本失败: %v", err)
		}

		syncResult.VersionRestored = true
		syncResult.AddChangeDetail("恢复软删除版本")

		needUpdate := false
		if existingVersion.VersionRole != versionRole {
			existingVersion.VersionRole = versionRole
			needUpdate = true
		}
		if existingVersion.Status != status {
			existingVersion.Status = status
			needUpdate = true
		}
		if annotations != nil {
			if v, ok := annotations[AnnotationVersion]; ok && v != "" && existingVersion.Version != v {
				existingVersion.Version = v
				needUpdate = true
			}
		}

		if needUpdate {
			existingVersion.UpdatedBy = SystemOperator
			existingVersion.IsDeleted = 0
			if err := h.svcCtx.ProjectVersion.Update(ctx, existingVersion); err != nil {
				logger.Errorf("[Version-RESTORE] 更新恢复后的版本失败: %v", err)
			}
		}

		return nil
	}

	if !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("查询版本失败: %v", err)
	}

	version := DefaultVersion
	if annotations != nil {
		if v, ok := annotations[AnnotationVersion]; ok && v != "" {
			version = v
		}
	}

	newVersion := &model.OnecProjectVersion{
		ApplicationId: applicationId,
		Version:       version,
		VersionRole:   versionRole,
		ResourceName:  resourceName,
		ParentAppName: "",
		CreatedBy:     SystemOperator,
		UpdatedBy:     SystemOperator,
		IsDeleted:     0,
		Status:        status,
	}

	_, err = h.svcCtx.ProjectVersion.Insert(ctx, newVersion)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "1062") {
			return nil
		}
		return fmt.Errorf("创建版本失败: %v", err)
	}

	syncResult.VersionCreated = true
	syncResult.AddChangeDetail("创建新版本")

	logger.Infof("[Version-CREATE] 创建版本成功: AppID=%d, ResourceName=%s, Version=%s, Role=%s, Status=%d",
		applicationId, resourceName, version, versionRole, status)

	return nil
}

// checkAndCorrectExistingBinding 检查并修正现有的版本绑定
// 返回值: shouldSkip (是否应该跳过后续处理), error
func (h *DefaultEventHandler) checkAndCorrectExistingBinding(
	ctx context.Context,
	existingApp *model.OnecProjectApplication,
	existingVersion *model.OnecProjectVersion,
	expectedVersionRole string,
	annotations map[string]string,
	expectedStatus int64,
	syncResult *SyncResult,
) (bool, error) {
	logger := logx.WithContext(ctx)

	// 1. 检查应用是否处于删除状态
	if existingApp.IsDeleted == 1 {
		// 应用是删除状态，修复为正常状态
		logger.Infof("[Workload-BINDING] 恢复软删除的应用: AppID=%d, NameEn=%s", existingApp.Id, existingApp.NameEn)
		if err := h.svcCtx.ProjectApplication.RestoreSoftDeleted(ctx, existingApp.Id, SystemOperator); err != nil {
			return false, fmt.Errorf("恢复应用失败: %v", err)
		}
		syncResult.ApplicationRestored = true
		syncResult.AddChangeDetail("恢复软删除应用")
		existingApp.IsDeleted = 0 // 更新内存中的状态
	}

	// 2. 检查版本是否处于删除状态
	if existingVersion.IsDeleted == 1 {
		// 版本是删除状态，修复为正常状态
		logger.Infof("[Workload-BINDING] 恢复软删除的版本: VersionID=%d, ResourceName=%s", existingVersion.Id, existingVersion.ResourceName)
		if err := h.svcCtx.ProjectVersion.RestoreSoftDeleted(ctx, existingVersion.Id, SystemOperator); err != nil {
			return false, fmt.Errorf("恢复版本失败: %v", err)
		}
		syncResult.VersionRestored = true
		syncResult.AddChangeDetail("恢复软删除版本")
		existingVersion.IsDeleted = 0 // 更新内存中的状态
	}

	// 3. 检查是否有明确的应用注解
	hasExplicitAppAnnotation := false
	annotationAppNameEn := ""
	annotationAppNameCn := ""

	if annotations != nil {
		if nameEn, ok := annotations[AnnotationApplicationEn]; ok && nameEn != "" {
			hasExplicitAppAnnotation = true
			annotationAppNameEn = nameEn
		}
		if nameCn, ok := annotations[AnnotationApplication]; ok && nameCn != "" {
			hasExplicitAppAnnotation = true
			annotationAppNameCn = nameCn
		}
	}

	// 4. 如果有明确的应用注解，检查应用的中英文名是否正确
	if hasExplicitAppAnnotation {
		needUpdateApp := false

		// 检查英文名
		if annotationAppNameEn != "" && existingApp.NameEn != annotationAppNameEn {
			logger.Infof("[Workload-BINDING] 应用英文名不匹配，需要修正: 当前=%s, 期望=%s", existingApp.NameEn, annotationAppNameEn)
			oldNameEn := existingApp.NameEn
			existingApp.NameEn = annotationAppNameEn
			needUpdateApp = true
			syncResult.ApplicationUpdated = true
			syncResult.AddChangeDetail(fmt.Sprintf("应用英文名: %s -> %s", oldNameEn, annotationAppNameEn))
		}

		// 检查中文名
		if annotationAppNameCn != "" && existingApp.NameCn != annotationAppNameCn {
			logger.Infof("[Workload-BINDING] 应用中文名不匹配，需要修正: 当前=%s, 期望=%s", existingApp.NameCn, annotationAppNameCn)
			oldNameCn := existingApp.NameCn
			existingApp.NameCn = annotationAppNameCn
			needUpdateApp = true
			syncResult.ApplicationUpdated = true
			syncResult.AddChangeDetail(fmt.Sprintf("应用中文名: %s -> %s", oldNameCn, annotationAppNameCn))
		}

		// 如果需要更新应用，执行更新
		if needUpdateApp {
			existingApp.UpdatedBy = SystemOperator
			if err := h.svcCtx.ProjectApplication.Update(ctx, existingApp); err != nil {
				return false, fmt.Errorf("更新应用失败: %v", err)
			}
			logger.Infof("[Workload-BINDING] 应用信息已修正: AppID=%d", existingApp.Id)
		}
	} else {
		// 5. 如果没有注解，应用正常则跳过
		logger.Debugf("[Workload-BINDING] 资源无应用注解，保持当前绑定: AppID=%d, NameEn=%s", existingApp.Id, existingApp.NameEn)
	}

	// 6. 检查版本信息是否需要更新
	needUpdateVersion := false
	var versionChanges []string

	// 检查版本角色
	if existingVersion.VersionRole != expectedVersionRole {
		versionChanges = append(versionChanges, fmt.Sprintf("角色: %s -> %s", existingVersion.VersionRole, expectedVersionRole))
		existingVersion.VersionRole = expectedVersionRole
		needUpdateVersion = true
	}

	// 检查版本状态
	if existingVersion.Status != expectedStatus {
		versionChanges = append(versionChanges, fmt.Sprintf("状态: %d -> %d", existingVersion.Status, expectedStatus))
		existingVersion.Status = expectedStatus
		needUpdateVersion = true
	}

	// 检查版本号（从注解）
	if annotations != nil {
		if v, ok := annotations[AnnotationVersion]; ok && v != "" && existingVersion.Version != v {
			versionChanges = append(versionChanges, fmt.Sprintf("版本号: %s -> %s", existingVersion.Version, v))
			existingVersion.Version = v
			needUpdateVersion = true
		}
	}

	// 清空不再使用的 ParentAppName
	if existingVersion.ParentAppName != "" {
		versionChanges = append(versionChanges, "清除ParentAppName")
		existingVersion.ParentAppName = ""
		needUpdateVersion = true
	}

	// 如果需要更新版本，执行更新
	if needUpdateVersion {
		existingVersion.UpdatedBy = SystemOperator
		if err := h.svcCtx.ProjectVersion.Update(ctx, existingVersion); err != nil {
			return false, fmt.Errorf("更新版本失败: %v", err)
		}
		syncResult.VersionUpdated = true
		for _, change := range versionChanges {
			syncResult.AddChangeDetail(change)
		}
		logger.Infof("[Workload-BINDING] 版本信息已修正: VersionID=%d, Changes=%v", existingVersion.Id, versionChanges)
	}

	// 7. 返回是否应该跳过后续处理
	// 如果版本已正确绑定到应用，跳过后续的应用创建和版本创建逻辑
	return true, nil
}
