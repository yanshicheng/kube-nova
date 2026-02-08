// ================== project_workspace_del_logic.go ==================
package managerservicelogic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	"github.com/zeromicro/go-zero/core/logx"
)

// API 删除标记常量
const (
	AnnotationDeletedBy  = "kube-nova.io/deleted-by"
	AnnotationDeleteTime = "kube-nova.io/delete-time"
	DeletedByAPI         = "api"
)

type ProjectWorkspaceDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectWorkspaceDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectWorkspaceDelLogic {
	return &ProjectWorkspaceDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectWorkspaceDel 删除项目工作空间
func (l *ProjectWorkspaceDelLogic) ProjectWorkspaceDel(in *pb.DelOnecProjectWorkspaceReq) (*pb.DelOnecProjectWorkspaceResp, error) {
	// Step 1: 查询工作空间是否存在
	workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查询工作空间失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("工作空间不存在")
	}

	// Step 2: 保存项目集群ID，用于后续资源同步
	projectClusterId := workspace.ProjectClusterId
	clusterUuid := workspace.ClusterUuid

	// Step 3: 查询项目集群和项目信息（用于审计日志）
	projectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, projectClusterId)
	if err != nil {
		l.Errorf("查询项目集群失败，ProjectClusterId: %d, 错误: %v", projectClusterId, err)
		return nil, errorx.Msg("查询项目集群失败")
	}

	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, projectCluster.ProjectId)
	if err != nil {
		l.Errorf("查询项目失败，ProjectId: %d, 错误: %v", projectCluster.ProjectId, err)
		return nil, errorx.Msg("查询项目失败")
	}

	// Step 4: 获取集群客户端
	clusterClient, err := l.svcCtx.K8sManager.GetCluster(l.ctx, clusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败，ClusterUuid: %s, 错误: %v", clusterUuid, err)
		return nil, errorx.Msg("获取集群客户端失败")
	}

	// Step 5: 标记并删除所有 K8s 工作负载资源
	if err := l.markAndDeleteAllWorkloads(workspace, clusterClient); err != nil {
		l.Errorf("标记并删除工作负载失败，WorkspaceID: %d, 错误: %v", workspace.Id, err)
		// 不返回错误，继续执行后续删除流程
	}

	// Step 6: 给 Namespace 打上 API 删除标记
	err = clusterClient.Namespaces().UpdateAnnotations(workspace.Namespace, map[string]string{
		AnnotationDeletedBy:  DeletedByAPI,
		AnnotationDeleteTime: time.Now().Format(time.RFC3339),
	})
	if err != nil {
		l.Errorf("标记 Namespace 删除来源失败: namespace=%s, 错误: %v", workspace.Namespace, err)
	} else {
		l.Infof("已标记 Namespace 为 API 删除: %s", workspace.Namespace)
	}

	// Step 7: 执行数据库级联硬删除：Version -> Application -> Workspace
	if err := l.cascadeDeleteWorkspace(workspace); err != nil {
		l.Errorf("级联删除工作空间失败，ID: %d, 错误: %v", in.Id, err)
		// 记录失败的审计日志
		l.recordWorkspaceAuditLog(workspace, project, projectCluster, false)
		return nil, errorx.Msg(fmt.Sprintf("级联删除工作空间失败: %v", err))
	}
	l.Infof("级联删除工作空间成功，ID: %d, 命名空间: %s", in.Id, workspace.Namespace)

	// Step 8: 删除 K8s Namespace
	err = clusterClient.Namespaces().Delete(workspace.Namespace)
	if err != nil {
		l.Errorf("删除 namespace 失败，命名空间: %s, 错误: %v", workspace.Namespace, err)
		return nil, errorx.Msg("删除 namespace 失败")
	}
	l.Infof("删除 namespace 成功，命名空间: %s", workspace.Namespace)

	// Step 9: 同步项目集群资源分配
	if projectClusterId > 0 {
		err = l.svcCtx.OnecProjectModel.SyncProjectClusterResourceAllocation(l.ctx, projectClusterId)
		if err != nil {
			l.Errorf("更新项目集群资源失败，项目集群ID: %d, 错误: %v", projectClusterId, err)
		} else {
			l.Infof("更新项目集群资源成功，项目集群ID: %d", projectClusterId)

			if err := l.svcCtx.OnecProjectClusterModel.DeleteCache(l.ctx, projectClusterId); err != nil {
				l.Errorf("清除项目集群缓存失败，项目集群ID: %d, 错误: %v", projectClusterId, err)
			}
		}
	} else {
		l.Infof("跳过资源同步：项目集群ID无效")
	}

	// Step 10: 记录审计日志
	l.recordWorkspaceAuditLog(workspace, project, projectCluster, true)

	return &pb.DelOnecProjectWorkspaceResp{}, nil
}

// markAndDeleteAllWorkloads 标记并删除工作空间下的所有 K8s 工作负载资源
func (l *ProjectWorkspaceDelLogic) markAndDeleteAllWorkloads(workspace *model.OnecProjectWorkspace, clusterClient cluster.Client) error {
	namespace := workspace.Namespace

	// 查询工作空间下所有应用
	apps, err := l.svcCtx.OnecProjectApplication.FindAllByWorkspaceId(l.ctx, workspace.Id)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("查询应用列表失败: %v", err)
	}

	// 如果没有应用，直接返回
	if len(apps) == 0 {
		l.Infof("[MarkAndDelete] 工作空间 %s 下没有应用，跳过", namespace)
		return nil
	}

	deleteAnnotations := map[string]string{
		AnnotationDeletedBy:  DeletedByAPI,
		AnnotationDeleteTime: time.Now().Format(time.RFC3339),
	}

	// 用于去重：key = "resourceType:resourceName"
	processedResources := make(map[string]bool)

	// 遍历每个应用
	for _, app := range apps {
		resourceType := app.ResourceType

		// 查询应用下所有版本
		versions, err := l.svcCtx.OnecProjectVersion.FindAllByApplicationId(l.ctx, app.Id)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			l.Errorf("[MarkAndDelete] 查询版本列表失败: AppID=%d, err=%v", app.Id, err)
			continue
		}

		// 遍历每个版本
		for _, version := range versions {
			resourceName := version.ResourceName

			// 去重检查
			resourceKey := fmt.Sprintf("%s:%s", resourceType, resourceName)
			if processedResources[resourceKey] {
				l.Infof("[MarkAndDelete] 跳过重复资源: %s/%s (type=%s)", namespace, resourceName, resourceType)
				continue
			}
			processedResources[resourceKey] = true

			l.Infof("[MarkAndDelete] 处理资源: type=%s, name=%s, namespace=%s", resourceType, resourceName, namespace)

			// 根据资源类型，标记并删除对应的 K8s 资源
			l.markAndDeleteK8sResource(clusterClient, resourceType, namespace, resourceName, deleteAnnotations)
		}
	}

	return nil
}

// markAndDeleteK8sResource 标记并删除单个 K8s 资源
func (l *ProjectWorkspaceDelLogic) markAndDeleteK8sResource(
	clusterClient cluster.Client,
	resourceType, namespace, resourceName string,
	annotations map[string]string,
) {
	switch resourceType {
	case "deployment":
		if err := clusterClient.Deployment().UpdateAnnotations(namespace, resourceName, annotations); err != nil {
			l.Errorf("[MarkAndDelete] 标记 Deployment 失败: %s/%s, err=%v", namespace, resourceName, err)
		} else {
			l.Infof("[MarkAndDelete] 已标记 Deployment: %s/%s", namespace, resourceName)
		}
		if err := clusterClient.Deployment().Delete(namespace, resourceName); err != nil {
			l.Errorf("[MarkAndDelete] 删除 Deployment 失败: %s/%s, err=%v", namespace, resourceName, err)
		} else {
			l.Infof("[MarkAndDelete] 已删除 Deployment: %s/%s", namespace, resourceName)
		}

	case "statefulset":
		if err := clusterClient.StatefulSet().UpdateAnnotations(namespace, resourceName, annotations); err != nil {
			l.Errorf("[MarkAndDelete] 标记 StatefulSet 失败: %s/%s, err=%v", namespace, resourceName, err)
		} else {
			l.Infof("[MarkAndDelete] 已标记 StatefulSet: %s/%s", namespace, resourceName)
		}
		if err := clusterClient.StatefulSet().Delete(namespace, resourceName); err != nil {
			l.Errorf("[MarkAndDelete] 删除 StatefulSet 失败: %s/%s, err=%v", namespace, resourceName, err)
		} else {
			l.Infof("[MarkAndDelete] 已删除 StatefulSet: %s/%s", namespace, resourceName)
		}

	case "daemonset":
		if err := clusterClient.DaemonSet().UpdateAnnotations(namespace, resourceName, annotations); err != nil {
			l.Errorf("[MarkAndDelete] 标记 DaemonSet 失败: %s/%s, err=%v", namespace, resourceName, err)
		} else {
			l.Infof("[MarkAndDelete] 已标记 DaemonSet: %s/%s", namespace, resourceName)
		}
		if err := clusterClient.DaemonSet().Delete(namespace, resourceName); err != nil {
			l.Errorf("[MarkAndDelete] 删除 DaemonSet 失败: %s/%s, err=%v", namespace, resourceName, err)
		} else {
			l.Infof("[MarkAndDelete] 已删除 DaemonSet: %s/%s", namespace, resourceName)
		}

	case "cronjob":
		if err := clusterClient.CronJob().UpdateAnnotations(namespace, resourceName, annotations); err != nil {
			l.Errorf("[MarkAndDelete] 标记 CronJob 失败: %s/%s, err=%v", namespace, resourceName, err)
		} else {
			l.Infof("[MarkAndDelete] 已标记 CronJob: %s/%s", namespace, resourceName)
		}
		if err := clusterClient.CronJob().Delete(namespace, resourceName); err != nil {
			l.Errorf("[MarkAndDelete] 删除 CronJob 失败: %s/%s, err=%v", namespace, resourceName, err)
		} else {
			l.Infof("[MarkAndDelete] 已删除 CronJob: %s/%s", namespace, resourceName)
		}

	default:
		l.Infof("[MarkAndDelete] 跳过未知资源类型: type=%s, name=%s", resourceType, resourceName)
	}
}

// cascadeDeleteWorkspace 级联硬删除工作空间及其下属资源
func (l *ProjectWorkspaceDelLogic) cascadeDeleteWorkspace(workspace *model.OnecProjectWorkspace) error {
	workspaceId := workspace.Id

	// 查询工作空间下所有应用
	apps, err := l.svcCtx.OnecProjectApplication.FindAllByWorkspaceId(l.ctx, workspaceId)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("查询应用列表失败: %v", err)
	}

	// 遍历删除
	for _, app := range apps {
		// 查询该应用下的所有版本
		versions, err := l.svcCtx.OnecProjectVersion.FindAllByApplicationId(l.ctx, app.Id)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			l.Errorf("[CascadeDelete] 查询版本列表失败: AppID=%d, err=%v", app.Id, err)
			continue
		}

		// 硬删除所有版本
		for _, version := range versions {
			if err := l.svcCtx.OnecProjectVersion.Delete(l.ctx, version.Id); err != nil {
				l.Errorf("[CascadeDelete] 硬删除版本失败: VersionID=%d, err=%v", version.Id, err)
				continue
			}
			l.Infof("[CascadeDelete] 硬删除版本成功: VersionID=%d, ResourceName=%s", version.Id, version.ResourceName)
		}

		// 硬删除应用
		if err := l.svcCtx.OnecProjectApplication.Delete(l.ctx, app.Id); err != nil {
			l.Errorf("[CascadeDelete] 硬删除应用失败: AppID=%d, err=%v", app.Id, err)
			continue
		}
		l.Infof("[CascadeDelete] 硬删除应用成功: AppID=%d, AppName=%s", app.Id, app.NameEn)
	}

	// 硬删除工作空间
	if err := l.svcCtx.OnecProjectWorkspaceModel.Delete(l.ctx, workspaceId); err != nil {
		return fmt.Errorf("硬删除工作空间失败: %v", err)
	}
	l.Infof("[CascadeDelete] 硬删除工作空间成功: WorkspaceID=%d, Namespace=%s", workspaceId, workspace.Namespace)

	return nil
}

// recordWorkspaceAuditLog 记录工作空间删除的审计日志
func (l *ProjectWorkspaceDelLogic) recordWorkspaceAuditLog(
	workspace *model.OnecProjectWorkspace,
	project *model.OnecProject,
	projectCluster *model.OnecProjectCluster,
	success bool,
) {
	// 构造状态值
	var status int64 = 1
	if !success {
		status = 0
	}

	// 集群名称，默认使用 UUID（如果有集群表可以查询名称）
	clusterName := workspace.ClusterUuid

	// 构造审计日志
	auditLog := &model.OnecProjectAuditLog{
		ClusterName:     clusterName,
		ClusterUuid:     workspace.ClusterUuid,
		ProjectId:       projectCluster.ProjectId,
		ProjectName:     project.Name,
		WorkspaceId:     workspace.Id,
		WorkspaceName:   workspace.Name,
		ApplicationId:   0,  // 工作空间级别操作，无具体应用
		ApplicationName: "", // 工作空间级别操作，无具体应用
		Title:           "删除工作空间",
		ActionDetail: fmt.Sprintf("删除工作空间[%s]，命名空间: %s，包含的所有应用和版本已级联删除",
			workspace.Name, workspace.Namespace),
		Status:       status,
		OperatorId:   0,  // TODO: 从上下文获取操作人ID
		OperatorName: "", // TODO: 从上下文获取操作人名称
	}

	_, err := l.svcCtx.OnecProjectAuditLog.Insert(l.ctx, auditLog)
	if err != nil {
		l.Errorf("[AuditLog] 记录工作空间删除审计日志失败: %v", err)
	} else {
		l.Infof("[AuditLog] 工作空间删除审计日志记录成功: WorkspaceID=%d, Namespace=%s", workspace.Id, workspace.Namespace)
	}
}
