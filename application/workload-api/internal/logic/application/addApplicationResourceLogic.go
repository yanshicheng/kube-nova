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
	corev1 "k8s.io/api/core/v1"
)

type AddApplicationResourceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddApplicationResourceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddApplicationResourceLogic {
	return &AddApplicationResourceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddApplicationResourceLogic) AddApplicationResource(req *types.AddApplicationResource) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 获取工作空间资源
	workspace, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{
		Id: req.WorkspaceId,
	})
	if err != nil {
		l.Errorf("获取工作空间失败: %v", err)
		return "", fmt.Errorf("获取工作空间失败: %v", err)
	}

	// 查询项目集群
	projectCluster, err := l.svcCtx.ManagerRpc.ProjectClusterGetById(l.ctx, &managerservice.GetOnecProjectClusterByIdReq{
		Id: workspace.Data.ProjectClusterId,
	})
	if err != nil {
		l.Errorf("获取项目集群失败: %v", err)
		return "", fmt.Errorf("获取项目集群失败: %v", err)
	}

	// 查询项目
	project, err := l.svcCtx.ManagerRpc.ProjectGetById(l.ctx, &managerservice.GetOnecProjectByIdReq{
		Id: projectCluster.Data.ProjectId,
	})
	if err != nil {
		l.Errorf("获取项目失败: %v", err)
		return "", fmt.Errorf("获取项目失败: %v", err)
	}

	// 解析 YAML
	k8sObj, err := utils.ParseAndConvertK8sResource(req.ResourceYamlStr, req.ResourceType)
	if err != nil {
		l.Errorf("解析并转换 Kubernetes 资源失败: %v", err)
		return "", fmt.Errorf("解析并转换 Kubernetes 资源失败: %v", err)
	}
	l.Infof("资源解析和转换成功，类型: %s", req.ResourceType)

	// 验证 K8s 资源
	validator := utils.K8sResourceValidator{
		ExpectedNamespace: workspace.Data.Namespace,
		ExpectedName:      req.ResourceName,
	}
	if err := validator.Validate(k8sObj); err != nil {
		l.Errorf("资源验证失败: %v", err)
		return "", fmt.Errorf("资源验证失败: %v", err)
	}

	// 注入注解，包括资源级别和 Pod 模板级别
	injectAnnotations(k8sObj, req.ResourceType, &utils.AnnotationsInfo{
		ServiceName:     req.ResourceName,
		ApplicationName: req.NameCn,
		ApplicationEn:   req.NameEn,
		ProjectName:     project.Data.Name,
		ProjectUuid:     project.Data.Uuid,
		Version:         req.Version,
		Description:     req.Description,
		WorkspaceName:   workspace.Data.Name,
	})

	// 验证 resourceType
	if !utils.IsResourceType(req.ResourceType) {
		return "", fmt.Errorf("资源类型错误")
	}

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

	// 判断是否为一次性资源
	resourceTypeUpper := strings.ToUpper(req.ResourceType)
	isOneTimeResource := resourceTypeUpper == "POD" || resourceTypeUpper == "JOB"

	if isOneTimeResource {
		l.Infof("资源类型 %s 为一次性任务，直接部署到集群", req.ResourceType)

		// 部署到 K8s 集群
		if err := l.deployToK8sCluster(k8sObj, req.ClusterUuid, workspace.Data.Namespace, req.ResourceType); err != nil {
			l.Errorf("部署到 K8s 集群失败: %v", err)

			// 记录失败的审计日志
			_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
				WorkspaceId:  req.WorkspaceId,
				Title:        "创建资源",
				ActionDetail: fmt.Sprintf("创建 %s 资源失败: %s, 错误: %v", req.ResourceType, req.ResourceName, err),
				Status:       0,
			})

			return "", fmt.Errorf("部署到 K8s 集群失败: %v", err)
		}

		// 记录成功的审计日志
		_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  req.WorkspaceId,
			Title:        "创建资源",
			ActionDetail: fmt.Sprintf("创建 %s 资源: %s", req.ResourceType, req.ResourceName),
			Status:       1,
		})
		if auditErr != nil {
			l.Errorf("记录审计日志失败: %v", auditErr)
		}
	} else {
		// 其他类型：创建应用和版本
		if err := utils.ValidateVersionName(req.NameEn); err != nil {
			return "", fmt.Errorf("服务英文名错误: %v", err)
		}
		if err := utils.ValidateVersionName(req.Version); err != nil {
			return "", fmt.Errorf("版本错误: %v", err)
		}

		// 添加应用
		addAppResp, err := l.svcCtx.ManagerRpc.ApplicationAdd(l.ctx, &managerservice.AddOnecProjectApplicationReq{
			WorkspaceId:  req.WorkspaceId,
			NameCn:       req.NameCn,
			NameEn:       req.NameEn,
			ResourceType: req.ResourceType,
			Description:  req.Description,
			CreatedBy:    username,
			UpdatedBy:    username,
		})
		if err != nil {
			l.Errorf("添加应用失败: %v", err)
			return "", err
		}

		// 添加版本
		addVersionResp, err := l.svcCtx.ManagerRpc.VersionAdd(l.ctx, &managerservice.AddOnecProjectVersionReq{
			ApplicationId: addAppResp.Id,
			Version:       req.Version,
			ResourceName:  req.ResourceName,
			CreatedBy:     username,
			UpdatedBy:     username,
		})
		if err != nil {
			l.Errorf("添加版本失败: %v", err)
			return "", err
		}

		// 部署到 K8s 集群
		if err := l.deployToK8sCluster(k8sObj, req.ClusterUuid, workspace.Data.Namespace, req.ResourceType); err != nil {
			l.Errorf("部署到 K8s 集群失败: %v", err)

			// 记录失败的审计日志
			_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
				VersionId:    addVersionResp.Id,
				Title:        "创建应用和版本",
				ActionDetail: fmt.Sprintf("创建应用和版本失败: %s-%s, 错误: %v", req.NameCn, req.Version, err),
				Status:       0,
			})

			return "", fmt.Errorf("部署到 K8s 集群失败: %v", err)
		}

		// 记录成功的审计日志
		_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			VersionId:    addVersionResp.Id,
			Title:        "创建应用和版本",
			ActionDetail: fmt.Sprintf("创建应用和版本: %s-%s", req.NameCn, req.Version),
			Status:       1,
		})
		if auditErr != nil {
			l.Errorf("记录审计日志失败: %v", auditErr)
		}
	}

	return fmt.Sprintf("资源 %s 创建成功", req.ResourceName), nil
}

// injectAnnotations 注入注解到资源，包括资源级别和 Pod 模板级别
func injectAnnotations(obj interface{}, resourceType string, info *utils.AnnotationsInfo) {

	switch strings.ToUpper(resourceType) {
	case "DEPLOYMENT":
		deployment := obj.(*appsv1.Deployment)
		utils.AddAnnotations(&deployment.ObjectMeta, info)
		utils.AddAnnotations(&deployment.Spec.Template.ObjectMeta, info)

	case "STATEFULSET":
		statefulSet := obj.(*appsv1.StatefulSet)
		utils.AddAnnotations(&statefulSet.ObjectMeta, info)
		utils.AddAnnotations(&statefulSet.Spec.Template.ObjectMeta, info)
	case "DAEMONSET":
		daemonSet := obj.(*appsv1.DaemonSet)
		utils.AddAnnotations(&daemonSet.ObjectMeta, info)
		utils.AddAnnotations(&daemonSet.Spec.Template.ObjectMeta, info)

	case "JOB":
		job := obj.(*batchv1.Job)
		utils.AddAnnotations(&job.ObjectMeta, info)
		utils.AddAnnotations(&job.Spec.Template.ObjectMeta, info)

	case "CRONJOB":
		cronJob := obj.(*batchv1.CronJob)
		utils.AddAnnotations(&cronJob.ObjectMeta, info)
		utils.AddAnnotations(&cronJob.Spec.JobTemplate.Spec.Template.ObjectMeta, info)

	case "POD":
		pod := obj.(*corev1.Pod)
		utils.AddAnnotations(&pod.ObjectMeta, info)
	}
}

// deployToK8sCluster 部署资源到 K8s 集群
func (l *AddApplicationResourceLogic) deployToK8sCluster(
	obj interface{},
	clusterUuid string,
	namespace string,
	resourceType string,
) error {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, clusterUuid)
	if err != nil {
		return fmt.Errorf("获取集群客户端失败: %v", err)
	}

	resourceType = strings.ToUpper(resourceType)

	switch resourceType {
	case "DEPLOYMENT":
		deployment := obj.(*appsv1.Deployment)
		l.Infof("准备部署 Deployment: %s 到集群: %s, namespace: %s", deployment.Name, clusterUuid, namespace)
		_, err := client.Deployment().Create(deployment)
		if err != nil {
			return fmt.Errorf("创建 Deployment 失败: %v", err)
		}
		l.Infof("Deployment %s 创建成功", deployment.Name)

	case "STATEFULSET":
		statefulSet := obj.(*appsv1.StatefulSet)
		l.Infof("准备部署 StatefulSet: %s 到集群: %s, namespace: %s", statefulSet.Name, clusterUuid, namespace)
		_, err := client.StatefulSet().Create(statefulSet)
		if err != nil {
			return fmt.Errorf("创建 StatefulSet 失败: %v", err)
		}
		l.Infof("StatefulSet %s 创建成功", statefulSet.Name)

	case "DAEMONSET":
		daemonSet := obj.(*appsv1.DaemonSet)
		l.Infof("准备部署 DaemonSet: %s 到集群: %s, namespace: %s", daemonSet.Name, clusterUuid, namespace)
		_, err := client.DaemonSet().Create(daemonSet)
		if err != nil {
			return fmt.Errorf("创建 DaemonSet 失败: %v", err)
		}
		l.Infof("DaemonSet %s 创建成功", daemonSet.Name)

	case "JOB":
		job := obj.(*batchv1.Job)
		l.Infof("准备部署 Job: %s 到集群: %s, namespace: %s", job.Name, clusterUuid, namespace)
		_, err := client.Job().Create(job)
		if err != nil {
			return fmt.Errorf("创建 Job 失败: %v", err)
		}
		l.Infof("Job %s 创建成功", job.Name)

	case "CRONJOB":
		cronJob := obj.(*batchv1.CronJob)
		l.Infof("准备部署 CronJob: %s 到集群: %s, namespace: %s", cronJob.Name, clusterUuid, namespace)
		_, err := client.CronJob().Create(cronJob)
		if err != nil {
			return fmt.Errorf("创建 CronJob 失败: %v", err)
		}
		l.Infof("CronJob %s 创建成功", cronJob.Name)

	case "POD":
		pod := obj.(*corev1.Pod)
		l.Infof("准备部署 Pod: %s 到集群: %s, namespace: %s", pod.Name, clusterUuid, namespace)
		_, err := client.Pods().Create(pod)
		if err != nil {
			return fmt.Errorf("创建 Pod 失败: %v", err)
		}
		l.Infof("Pod %s 创建成功", pod.Name)

	default:
		return fmt.Errorf("不支持的资源类型: %s", resourceType)
	}

	return nil
}
