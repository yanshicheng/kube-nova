package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/rsync/operator"
)

const (
	// AnnotationVersion 版本注解
	AnnotationVersion = "ikubeops.com/version"
	// AnnotationVersionRole 版本角色注解（用户可通过此注解显式指定角色）
	AnnotationVersionRole = "ikubeops.com/version-role"

	DefaultVersionRole = "stable"
	DefaultVersion     = "v1"
)

func (h *DefaultEventHandler) syncWorkloadToDatabase(ctx context.Context, clusterUUID, namespace, name, resourceType string, labels, annotations map[string]string) error {
	logger := logx.WithContext(ctx)

	workspaces, err := h.svcCtx.ProjectWorkspaceModel.FindAllByClusterUuidNamespaceIncludeDeleted(ctx, clusterUUID, namespace)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("查询工作空间失败: %v", err)
	}

	// 找到未删除的工作空间
	var workspace *model.OnecProjectWorkspace
	for _, ws := range workspaces {
		if ws.IsDeleted == 0 {
			workspace = ws
			break
		}
	}

	if workspace == nil {
		// 工作空间不存在，说明该 namespace 不在平台管理范围内，跳过
		logger.Debugf("[Workload-SYNC] 未找到对应的工作空间，跳过: ClusterUUID=%s, Namespace=%s", clusterUUID, namespace)
		return nil
	}

	var flaggerHelper *operator.FlaggerHelper
	if h.svcCtx.K8sManager != nil {
		k8sClient, err := h.svcCtx.K8sManager.GetCluster(ctx, clusterUUID)
		if err != nil {
			logger.Errorf("[Workload-SYNC] 获取 K8s 客户端失败，降级为普通资源处理: %v", err)
			// 获取客户端失败，降级处理
		} else {
			flaggerHelper = operator.NewFlaggerHelper(k8sClient)
		}
	}

	flaggerAvailable := false
	if flaggerHelper != nil {
		flaggerAvailable = flaggerHelper.CheckFlaggerCRD(ctx)
		if flaggerAvailable {
			// 加载该命名空间的所有 Canary 策略
			if err := flaggerHelper.LoadNamespaceCanaries(ctx, namespace); err != nil {
				logger.Infof("[Workload-SYNC] 加载 Flagger Canary 失败，继续处理: %v", err)
				// 加载失败不影响后续处理
			}
		}
	}

	logger.Debugf("[Workload-SYNC] Flagger 状态: available=%v, clusterUUID=%s, namespace=%s",
		flaggerAvailable, clusterUUID, namespace)

	// 将 resourceType 转换为 Kind 格式（Deployment, StatefulSet 等）
	resourceKind := resourceTypeToKind(resourceType)

	var flaggerInfo *operator.FlaggerResourceInfo
	if flaggerHelper != nil && flaggerAvailable {
		flaggerInfo = flaggerHelper.IdentifyResource(name, resourceKind, labels, annotations)
	} else {
		flaggerInfo = identifyFlaggerResourceByNaming(name, annotations)
	}

	appName := flaggerInfo.OriginalAppName

	// 用户可通过注解指定应用的中文名（不影响 appName）
	appNameCn := appName
	if annotations != nil {
		if customName, ok := annotations[utils.AnnotationApplicationName]; ok && customName != "" {
			appNameCn = customName
		}
	}

	logger.Infof("[Workload-SYNC] 处理资源: resourceName=%s, type=%s, isFlagger=%v, role=%s, appName=%s, flaggerAvailable=%v",
		name, resourceType, flaggerInfo.IsFlaggerManaged, flaggerInfo.VersionRole, appName, flaggerAvailable)

	application, err := h.ensureApplication(ctx, workspace.Id, appName, appNameCn, resourceType)
	if err != nil {
		return fmt.Errorf("确保应用存在失败: %v", err)
	}

	err = h.ensureVersion(ctx, application.Id, name, flaggerInfo.VersionRole, annotations)
	if err != nil {
		return fmt.Errorf("确保版本存在失败: %v", err)
	}

	logger.Infof("[Workload-SYNC] 同步成功: ClusterUUID=%s, Namespace=%s, ResourceName=%s, AppName=%s, AppID=%d, Role=%s",
		clusterUUID, namespace, name, appName, application.Id, flaggerInfo.VersionRole)

	return nil
}

// deleteWorkloadFromDatabase 从数据库删除工作负载（软删除版本）
func (h *DefaultEventHandler) deleteWorkloadFromDatabase(ctx context.Context, clusterUUID, namespace, name, resourceType string) error {
	logger := logx.WithContext(ctx)

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

	flaggerInfo := identifyFlaggerResourceByNaming(name, nil)
	appName := flaggerInfo.OriginalAppName

	application, err := h.svcCtx.ProjectApplication.FindOneByWorkspaceIdNameEnResourceType(ctx, workspace.Id, appName, resourceType)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			logger.Debugf("[Workload-DELETE] 应用不存在，跳过: WorkspaceID=%d, AppName=%s, Type=%s",
				workspace.Id, appName, resourceType)
			return nil
		}
		return fmt.Errorf("查询应用失败: %v", err)
	}

	version, err := h.svcCtx.ProjectVersion.FindOneByApplicationIdResourceName(ctx, application.Id, name)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			// 版本不存在，可能已经通过平台删除，跳过
			logger.Debugf("[Workload-DELETE] 版本不存在，跳过: AppID=%d, ResourceName=%s", application.Id, name)
			return nil
		}
		return fmt.Errorf("查询版本失败: %v", err)
	}

	// 软删除版本
	if err := h.svcCtx.ProjectVersion.DeleteSoft(ctx, version.Id); err != nil {
		return fmt.Errorf("软删除版本失败: %v", err)
	}

	logger.Infof("[Workload-DELETE] 软删除版本成功: VersionID=%d, AppID=%d, ResourceName=%s",
		version.Id, application.Id, name)

	return nil
}

// identifyFlaggerResourceByNaming 通过命名规则识别 Flagger 资源（简化版）
func identifyFlaggerResourceByNaming(resourceName string, annotations map[string]string) *operator.FlaggerResourceInfo {
	info := &operator.FlaggerResourceInfo{
		IsFlaggerManaged: false,
		OriginalAppName:  resourceName,
		VersionRole:      operator.VersionRoleStable, // 默认是 stable
	}

	// 通过命名规则检测可能的 Flagger 资源
	originalName := resourceName

	if strings.HasSuffix(resourceName, "-primary") {
		originalName = strings.TrimSuffix(resourceName, "-primary")
		info.IsFlaggerManaged = true
		info.OriginalAppName = originalName
		info.VersionRole = operator.VersionRoleStable
	} else if strings.HasSuffix(resourceName, "-canary") {
		originalName = strings.TrimSuffix(resourceName, "-canary")
		info.IsFlaggerManaged = true
		info.OriginalAppName = originalName
		info.VersionRole = operator.VersionRoleStable
	}

	if annotations != nil {
		if role, ok := annotations[AnnotationVersionRole]; ok && role != "" {
			info.VersionRole = role
		}
	}

	return info
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

// ensureApplication 确保应用存在，不存在则创建
func (h *DefaultEventHandler) ensureApplication(ctx context.Context, workspaceId uint64, nameEn, nameCn, resourceType string) (*model.OnecProjectApplication, error) {
	// 查找已存在的应用
	application, err := h.svcCtx.ProjectApplication.FindOneByWorkspaceIdNameEnResourceType(ctx, workspaceId, nameEn, resourceType)
	if err == nil {
		// 应用已存在，直接返回
		return application, nil
	}

	if !errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("查询应用失败: %v", err)
	}

	// 应用不存在，创建新应用
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
		// 处理并发创建的重复键错误
		if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "1062") {
			// 重新查询返回已存在的记录
			return h.svcCtx.ProjectApplication.FindOneByWorkspaceIdNameEnResourceType(ctx, workspaceId, nameEn, resourceType)
		}
		return nil, fmt.Errorf("创建应用失败: %v", err)
	}

	insertId, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("获取插入ID失败: %v", err)
	}
	newApp.Id = uint64(insertId)

	logx.WithContext(ctx).Infof("[Application-CREATE] 创建应用成功: ID=%d, WorkspaceID=%d, NameEn=%s, Type=%s",
		newApp.Id, workspaceId, nameEn, resourceType)

	return newApp, nil
}

// ensureVersion 确保版本存在，不存在则创建
func (h *DefaultEventHandler) ensureVersion(ctx context.Context, applicationId uint64, resourceName, versionRole string, annotations map[string]string) error {
	// 查找已存在的版本（通过 resource_name）
	existingVersion, err := h.svcCtx.ProjectVersion.FindOneByApplicationIdResourceName(ctx, applicationId, resourceName)
	if err == nil {
		// 版本已存在，检查是否需要更新
		changed := false

		// 更新状态为存在
		if existingVersion.Status != 1 {
			existingVersion.Status = 1
			changed = true
		}

		// 更新版本角色
		if existingVersion.VersionRole != versionRole {
			logx.WithContext(ctx).Infof("[Version-UPDATE] 更新版本角色: %s -> %s, VersionID=%d",
				existingVersion.VersionRole, versionRole, existingVersion.Id)
			existingVersion.VersionRole = versionRole
			changed = true
		}

		// 清空不再使用的 ParentAppName 字段
		if existingVersion.ParentAppName != "" {
			existingVersion.ParentAppName = ""
			changed = true
		}

		if changed {
			existingVersion.UpdatedBy = SystemOperator
			if err := h.svcCtx.ProjectVersion.Update(ctx, existingVersion); err != nil {
				return fmt.Errorf("更新版本失败: %v", err)
			}
			logx.WithContext(ctx).Infof("[Version-UPDATE] 版本已更新: VersionID=%d, Role=%s", existingVersion.Id, versionRole)
		}
		return nil
	}

	if !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("查询版本失败: %v", err)
	}

	// 版本不存在，创建新版本
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
		ParentAppName: "", // 不再使用此字段
		CreatedBy:     SystemOperator,
		UpdatedBy:     SystemOperator,
		IsDeleted:     0,
		Status:        1,
	}

	_, err = h.svcCtx.ProjectVersion.Insert(ctx, newVersion)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "1062") {
			return nil // 已存在，忽略
		}
		return fmt.Errorf("创建版本失败: %v", err)
	}

	logx.WithContext(ctx).Infof("[Version-CREATE] 创建版本成功: AppID=%d, ResourceName=%s, Role=%s",
		applicationId, resourceName, versionRole)

	return nil
}
