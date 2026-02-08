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

type VersionDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVersionDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VersionDelLogic {
	return &VersionDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// VersionDel 删除版本
func (l *VersionDelLogic) VersionDel(in *pb.DelOnecProjectVersionReq) (*pb.DelOnecProjectVersionResp, error) {
	// ========== Step 1: 参数校验 ==========
	if in.Id == 0 {
		l.Errorf("参数校验失败: id 不能为空")
		return nil, errorx.Msg("版本ID不能为空")
	}

	// ========== Step 2: 查询版本是否存在 ==========
	version, err := l.svcCtx.OnecProjectVersion.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("版本不存在: id=%d", in.Id)
			return nil, errorx.Msg("版本不存在")
		}
		l.Errorf("查询版本失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("查询版本失败")
	}

	// ========== Step 3: 查询关联的 Application 获取 ResourceType ==========
	application, err := l.svcCtx.OnecProjectApplication.FindOne(l.ctx, version.ApplicationId)
	if err != nil {
		l.Errorf("查询应用失败: ApplicationId=%d, 错误: %v", version.ApplicationId, err)
		return nil, errorx.Msg("查询关联应用失败")
	}
	resourceType := application.ResourceType

	// ========== Step 4: 查询关联的 Workspace 获取 Namespace 和 ClusterUuid ==========
	workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, application.WorkspaceId)
	if err != nil {
		l.Errorf("查询工作空间失败: WorkspaceId=%d, 错误: %v", application.WorkspaceId, err)
		return nil, errorx.Msg("查询关联工作空间失败")
	}
	namespace := workspace.Namespace
	clusterUuid := workspace.ClusterUuid

	// ========== Step 5: 获取集群客户端 ==========
	clusterClient, err := l.svcCtx.K8sManager.GetCluster(l.ctx, clusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败，ClusterUuid: %s, 错误: %v", clusterUuid, err)
		return nil, errorx.Msg("获取集群客户端失败")
	}

	// ========== Step 6: 标记并删除 K8s 资源 ==========
	deleteAnnotations := map[string]string{
		AnnotationDeletedBy:  DeletedByAPI,
		AnnotationDeleteTime: time.Now().Format(time.RFC3339),
	}

	// 6.1 添加删除注解
	if err := l.updateK8sResourceAnnotations(clusterClient, resourceType, namespace, version.ResourceName, deleteAnnotations); err != nil {
		l.Errorf("[VersionDel] 标记 K8s 资源失败: type=%s, namespace=%s, name=%s, err=%v",
			resourceType, namespace, version.ResourceName, err)
		// 标记失败不阻止删除流程，继续执行
	} else {
		l.Infof("[VersionDel] 已标记 K8s 资源: type=%s, namespace=%s, name=%s",
			resourceType, namespace, version.ResourceName)
	}

	// 6.2 删除 K8s 资源
	if err := l.deleteK8sResource(clusterClient, resourceType, namespace, version.ResourceName); err != nil {
		l.Errorf("[VersionDel] 删除 K8s 资源失败: type=%s, namespace=%s, name=%s, err=%v",
			resourceType, namespace, version.ResourceName, err)
		// K8s 删除失败不阻止数据库删除，继续执行
	} else {
		l.Infof("[VersionDel] 已删除 K8s 资源: type=%s, namespace=%s, name=%s",
			resourceType, namespace, version.ResourceName)
	}

	// ========== Step 7: 删除数据库记录 ==========
	if err := l.svcCtx.OnecProjectVersion.Delete(l.ctx, in.Id); err != nil {
		l.Errorf("删除版本失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("删除版本失败")
	}
	l.Infof("[VersionDel] 数据库删除版本成功: VersionID=%d, ResourceName=%s", in.Id, version.ResourceName)

	// ========== Step 8: 记录审计日志 ==========
	l.recordAuditLog(workspace, application, version, true)

	l.Infof("版本删除成功: id=%d, applicationId=%d, version=%s, resourceName=%s",
		in.Id, version.ApplicationId, version.Version, version.ResourceName)

	return &pb.DelOnecProjectVersionResp{}, nil
}

// updateK8sResourceAnnotations 根据资源类型更新 K8s 资源注解
func (l *VersionDelLogic) updateK8sResourceAnnotations(
	clusterClient cluster.Client,
	resourceType, namespace, resourceName string,
	annotations map[string]string,
) error {
	switch resourceType {
	case "deployment":
		return clusterClient.Deployment().UpdateAnnotations(namespace, resourceName, annotations)
	case "statefulset":
		return clusterClient.StatefulSet().UpdateAnnotations(namespace, resourceName, annotations)
	case "daemonset":
		return clusterClient.DaemonSet().UpdateAnnotations(namespace, resourceName, annotations)
	case "cronjob":
		return clusterClient.CronJob().UpdateAnnotations(namespace, resourceName, annotations)
	default:
		l.Infof("[VersionDel] 跳过未知资源类型的注解更新: type=%s, name=%s", resourceType, resourceName)
		return nil
	}
}

// deleteK8sResource 根据资源类型删除 K8s 资源
func (l *VersionDelLogic) deleteK8sResource(
	clusterClient cluster.Client,
	resourceType, namespace, resourceName string,
) error {
	switch resourceType {
	case "deployment":
		return clusterClient.Deployment().Delete(namespace, resourceName)
	case "statefulset":
		return clusterClient.StatefulSet().Delete(namespace, resourceName)
	case "daemonset":
		return clusterClient.DaemonSet().Delete(namespace, resourceName)
	case "cronjob":
		return clusterClient.CronJob().Delete(namespace, resourceName)
	default:
		l.Infof("[VersionDel] 跳过未知资源类型的删除: type=%s, name=%s", resourceType, resourceName)
		return nil
	}
}

// recordAuditLog 记录审计日志
func (l *VersionDelLogic) recordAuditLog(
	workspace *model.OnecProjectWorkspace,
	application *model.OnecProjectApplication,
	version *model.OnecProjectVersion,
	success bool,
) {
	// 查询项目集群信息以获取项目相关信息
	projectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, workspace.ProjectClusterId)
	if err != nil {
		l.Errorf("[AuditLog] 查询项目集群失败: ProjectClusterId=%d, err=%v", workspace.ProjectClusterId, err)
		return
	}

	// 查询项目信息
	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, projectCluster.ProjectId)
	if err != nil {
		l.Errorf("[AuditLog] 查询项目失败: ProjectId=%d, err=%v", projectCluster.ProjectId, err)
		return
	}

	// 查询集群信息获取集群名称（如果有 ClusterModel）
	clusterName := workspace.ClusterUuid // 默认使用 UUID，如果有集群表可以查询名称

	// 构造状态值
	var status int64 = 1
	if !success {
		status = 0
	}

	// 构造审计日志
	auditLog := &model.OnecProjectAuditLog{
		ClusterName:     clusterName,
		ClusterUuid:     workspace.ClusterUuid,
		ProjectId:       projectCluster.ProjectId,
		ProjectName:     project.Name,
		WorkspaceId:     workspace.Id,
		WorkspaceName:   workspace.Name,
		ApplicationId:   application.Id,
		ApplicationName: application.NameEn,
		Title:           "删除应用版本",
		ActionDetail: fmt.Sprintf("删除应用[%s]的版本[%s]，资源类型: %s，资源名称: %s",
			application.NameEn, version.Version, application.ResourceType, version.ResourceName),
		Status:       status,
		OperatorId:   0,  // TODO: 从上下文获取操作人ID
		OperatorName: "", // TODO: 从上下文获取操作人名称
		IsDeleted:    0,
	}

	_, err = l.svcCtx.OnecProjectAuditLog.Insert(l.ctx, auditLog)
	if err != nil {
		l.Errorf("[AuditLog] 记录审计日志失败: %v", err)
	} else {
		l.Infof("[AuditLog] 审计日志记录成功: 删除版本 %s", version.Version)
	}
}
