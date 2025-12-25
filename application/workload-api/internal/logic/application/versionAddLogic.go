package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
)

type VersionAddLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 添加版本
func NewVersionAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VersionAddLogic {
	return &VersionAddLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}
func (l *VersionAddLogic) VersionAdd(req *types.AddOnecProjectVersionReq) (resp string, err error) {
	// 获取当前用户信息
	username, _ := l.ctx.Value("username").(string)
	if username == "" {
		username = "system"
	}

	// 验证版本名称格式
	if err := utils.ValidateVersionName(req.Version); err != nil {
		l.Errorf("版本名称格式错误: %v", err)
		return "", fmt.Errorf("版本名称格式错误")
	}

	// 获取应用信息
	appResp, err := l.svcCtx.ManagerRpc.ApplicationGetById(l.ctx, &managerservice.GetOnecProjectApplicationByIdReq{
		Id: req.ApplicationId,
	})
	if err != nil {
		l.Errorf("应用不存在: %v", err)
		return "", fmt.Errorf("应用不存在")
	}
	app := appResp.Data

	// 获取工作空间信息
	workspace, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{
		Id: app.WorkspaceId,
	})
	if err != nil {
		l.Errorf("获取工作空间信息失败: %v", err)
		return "", fmt.Errorf("获取工作空间信息失败")
	}

	// 查询 project cluster
	projectCluster, err := l.svcCtx.ManagerRpc.ProjectClusterGetById(l.ctx, &managerservice.GetOnecProjectClusterByIdReq{
		Id: workspace.Data.ProjectClusterId,
	})
	if err != nil {
		l.Errorf("获取项目集群失败: %v", err)
		return "", fmt.Errorf("获取项目集群失败: %v", err)
	}

	// 查询 project
	project, err := l.svcCtx.ManagerRpc.ProjectGetById(l.ctx, &managerservice.GetOnecProjectByIdReq{
		Id: projectCluster.Data.ProjectId,
	})
	if err != nil {
		l.Errorf("获取项目失败: %v", err)
		return "", fmt.Errorf("获取项目失败: %v", err)
	}

	// 验证 namespace 是否正常
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workspace.Data.ClusterUuid)
	if err != nil {
		l.Errorf("集群管理器获取失败: %s", err.Error())
		return "", err
	}
	_, err = client.Namespaces().Get(workspace.Data.Namespace)
	if err != nil {
		l.Errorf("命名空间获取异常: %s", err.Error())
		return "", err
	}

	expectedResourceName := req.ResourceName

	// 解析 YAML
	k8sObj, err := utils.ParseAndConvertK8sResource(req.ResourceYamlStr, appResp.Data.ResourceType)
	if err != nil {
		l.Errorf("解析并转换 Kubernetes 资源失败: %v", err)
		return "", fmt.Errorf("解析并转换 Kubernetes 资源失败: %v", err)
	}
	l.Infof("资源解析和转换成功，类型: %s", appResp.Data.ResourceType)

	// 验证 K8s 资源
	validator := utils.K8sResourceValidator{
		ExpectedNamespace: workspace.Data.Namespace,
		ExpectedName:      req.ResourceName,
	}
	if err := validator.Validate(k8sObj); err != nil {
		l.Errorf("资源验证失败: %v", err)
		return "", fmt.Errorf("资源验证失败: %v", err)
	}

	// 注入注解（资源级别 + Pod 模板级别）
	injectAnnotations(k8sObj, appResp.Data.ResourceType, &utils.AnnotationsInfo{
		ServiceName:   req.ResourceName,
		ProjectName:   appResp.Data.NameCn,
		Version:       req.Version,
		Description:   appResp.Data.Description,
		ProjectUuid:   project.Data.Uuid,
		WorkspaceName: workspace.Data.Name,
	})

	addVersionResp, err := l.svcCtx.ManagerRpc.VersionAdd(l.ctx, &managerservice.AddOnecProjectVersionReq{
		ApplicationId: req.ApplicationId,
		Version:       req.Version,
		CreatedBy:     username,
		UpdatedBy:     username,
		ResourceName:  req.ResourceName,
	})
	if err != nil {
		l.Errorf("添加版本记录失败: %v", err)
		return "", fmt.Errorf("添加版本记录失败: %v", err)
	}

	if err := l.deployToK8sCluster(
		k8sObj,
		workspace.Data.ClusterUuid,
		workspace.Data.Namespace,
		app.ResourceType,
		expectedResourceName,
	); err != nil {
		l.Errorf("部署到 K8s 集群失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			VersionId:    addVersionResp.Id,
			Title:        "添加版本",
			ActionDetail: fmt.Sprintf("添加版本并部署资源失败: %s, 错误: %v", expectedResourceName, err),
			Status:       0,
		})

		return "", fmt.Errorf("部署到 K8s 集群失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		VersionId:    addVersionResp.Id,
		Title:        "添加版本",
		ActionDetail: fmt.Sprintf("添加版本并部署资源: %s", expectedResourceName),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	return "添加成功", nil
}

// deployToK8sCluster 部署资源到 K8s 集群
func (l *VersionAddLogic) deployToK8sCluster(
	obj interface{},
	clusterUuid string,
	namespace string,
	resourceType string,
	resourceName string,
) error {

	// 获取 K8s 集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, clusterUuid)
	if err != nil {
		return fmt.Errorf("获取集群客户端失败: %v", err)
	}
	l.Info("集群客户端获取成功")

	resourceType = strings.ToUpper(resourceType)

	switch resourceType {
	case "DEPLOYMENT":
		deployment := obj.(*appsv1.Deployment)
		l.Infof("创建 Deployment: %s", deployment.Name)
		_, err := client.Deployment().Create(deployment)
		if err != nil {
			return fmt.Errorf("创建 Deployment 失败: %v", err)
		}
		l.Infof("Deployment %s 创建成功", deployment.Name)

	case "STATEFULSET":
		statefulSet := obj.(*appsv1.StatefulSet)
		l.Infof("创建 StatefulSet: %s", statefulSet.Name)
		_, err := client.StatefulSet().Create(statefulSet)
		if err != nil {
			return fmt.Errorf("创建 StatefulSet 失败: %v", err)
		}
		l.Infof("StatefulSet %s 创建成功", statefulSet.Name)

	case "DAEMONSET":
		daemonSet := obj.(*appsv1.DaemonSet)
		l.Infof("创建 DaemonSet: %s", daemonSet.Name)
		_, err := client.DaemonSet().Create(daemonSet)
		if err != nil {
			return fmt.Errorf("创建 DaemonSet 失败: %v", err)
		}
		l.Infof("DaemonSet %s 创建成功", daemonSet.Name)

	case "CRONJOB":
		cronJob := obj.(*batchv1.CronJob)
		l.Infof("创建 CronJob: %s", cronJob.Name)
		_, err := client.CronJob().Create(cronJob)
		if err != nil {
			return fmt.Errorf("创建 CronJob 失败: %v", err)
		}
		l.Infof("CronJob %s 创建成功", cronJob.Name)

	default:
		return fmt.Errorf("不支持的资源类型: %s", resourceType)
	}

	return nil
}
