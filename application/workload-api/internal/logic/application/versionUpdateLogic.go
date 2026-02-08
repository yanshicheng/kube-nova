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

type VersionUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新版本
func NewVersionUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VersionUpdateLogic {
	return &VersionUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *VersionUpdateLogic) VersionUpdate(req *types.UpdateOnecProjectVersionReq) (resp string, err error) {
	username, _ := l.ctx.Value("username").(string)
	if username == "" {
		username = "system"
	}

	// 获取版本详情
	versionDetail, err := l.svcCtx.ManagerRpc.VersionDetail(l.ctx, &managerservice.GetOnecProjectVersionDetailReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取版本详情失败: %v", err)
		return "", err
	}

	// 验证 namespace 是否正常
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, versionDetail.ClusterUuid)
	if err != nil {
		l.Errorf("集群管理器获取失败: %s", err.Error())
		return "", err
	}
	_, err = client.Namespaces().Get(versionDetail.Namespace)
	if err != nil {
		l.Errorf("命名空间获取异常: %s", err.Error())
		return "", err
	}

	// 解析 YAML
	k8sObj, err := utils.ParseAndConvertK8sResource(req.ResourceYamlStr, versionDetail.ResourceType)
	if err != nil {
		l.Errorf("解析并转换 Kubernetes 资源失败: %v", err)
		return "", fmt.Errorf("解析并转换 Kubernetes 资源失败: %v", err)
	}

	// 验证 K8s 资源
	validator := utils.K8sResourceValidator{
		ExpectedNamespace: versionDetail.Namespace,
		ExpectedName:      versionDetail.ResourceName,
	}
	if err := validator.Validate(k8sObj); err != nil {
		l.Errorf("资源验证失败: %v", err)
		return "", fmt.Errorf("资源验证失败: %v", err)
	}

	// 注入注解（资源级别 + Pod 模板级别）
	injectAnnotations(k8sObj, versionDetail.ResourceType, &utils.AnnotationsInfo{
		ServiceName:     versionDetail.ResourceName,
		ProjectName:     versionDetail.ProjectNameCn,
		ApplicationName: versionDetail.ApplicationNameCn,
		ApplicationEn:   versionDetail.ApplicationNameEn,
		Version:         versionDetail.Version,
		Description:     versionDetail.ProjectDetails,
		ProjectUuid:     versionDetail.ProjectNameCn,
		WorkspaceName:   versionDetail.WorkspaceNameCn,
	})

	// 更新 K8s 资源
	if err := l.updateK8sResource(k8sObj, versionDetail); err != nil {
		l.Errorf("更新 K8s 资源失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			VersionId:    versionDetail.VersionId,
			Title:        "版本更新",
			ActionDetail: fmt.Sprintf("更新版本资源失败: %s, 错误: %v", versionDetail.ResourceName, err),
			Status:       0,
		})

		return "", fmt.Errorf("更新 K8s 资源失败: %v", err)
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		VersionId:    versionDetail.VersionId,
		Title:        "版本更新",
		ActionDetail: fmt.Sprintf("更新版本资源: %s", versionDetail.ResourceName),
		Status:       1,
	})

	return "更新成功", nil
}

func (l *VersionUpdateLogic) updateK8sResource(obj interface{}, detail *managerservice.GetOnecProjectVersionDetailResp) error {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, detail.ClusterUuid)
	if err != nil {
		return fmt.Errorf("获取集群客户端失败: %v", err)
	}

	resourceType := strings.ToUpper(detail.ResourceType)

	switch resourceType {
	case "DEPLOYMENT":
		deployment := obj.(*appsv1.Deployment)

		existing, err := client.Deployment().Get(deployment.Namespace, deployment.Name)
		if err != nil {
			return fmt.Errorf("获取现有 Deployment 失败: %v", err)
		}

		deployment.Spec.Selector = existing.Spec.Selector

		if deployment.Spec.Template.Labels == nil {
			deployment.Spec.Template.Labels = make(map[string]string)
		}
		for key, value := range existing.Spec.Selector.MatchLabels {
			deployment.Spec.Template.Labels[key] = value
		}

		if deployment.Labels == nil {
			deployment.Labels = make(map[string]string)
		}
		for key, value := range existing.Spec.Selector.MatchLabels {
			deployment.Labels[key] = value
		}

		deployment.ResourceVersion = existing.ResourceVersion

		_, err = client.Deployment().Update(deployment)
		return err

	case "STATEFULSET":
		statefulSet := obj.(*appsv1.StatefulSet)

		existing, err := client.StatefulSet().Get(statefulSet.Namespace, statefulSet.Name)
		if err != nil {
			return fmt.Errorf("获取现有 StatefulSet 失败: %v", err)
		}

		statefulSet.Spec.Selector = existing.Spec.Selector

		statefulSet.Spec.ServiceName = existing.Spec.ServiceName

		if statefulSet.Spec.Template.Labels == nil {
			statefulSet.Spec.Template.Labels = make(map[string]string)
		}
		for key, value := range existing.Spec.Selector.MatchLabels {
			statefulSet.Spec.Template.Labels[key] = value
		}

		if statefulSet.Labels == nil {
			statefulSet.Labels = make(map[string]string)
		}
		for key, value := range existing.Spec.Selector.MatchLabels {
			statefulSet.Labels[key] = value
		}

		statefulSet.Spec.VolumeClaimTemplates = existing.Spec.VolumeClaimTemplates

		statefulSet.ResourceVersion = existing.ResourceVersion

		_, err = client.StatefulSet().Update(statefulSet)
		return err

	case "DAEMONSET":
		daemonSet := obj.(*appsv1.DaemonSet)

		existing, err := client.DaemonSet().Get(daemonSet.Namespace, daemonSet.Name)
		if err != nil {
			return fmt.Errorf("获取现有 DaemonSet 失败: %v", err)
		}

		daemonSet.Spec.Selector = existing.Spec.Selector

		if daemonSet.Spec.Template.Labels == nil {
			daemonSet.Spec.Template.Labels = make(map[string]string)
		}
		for key, value := range existing.Spec.Selector.MatchLabels {
			daemonSet.Spec.Template.Labels[key] = value
		}

		if daemonSet.Labels == nil {
			daemonSet.Labels = make(map[string]string)
		}
		for key, value := range existing.Spec.Selector.MatchLabels {
			daemonSet.Labels[key] = value
		}

		daemonSet.ResourceVersion = existing.ResourceVersion

		_, err = client.DaemonSet().Update(daemonSet)
		return err

	case "CRONJOB":
		cronJob := obj.(*batchv1.CronJob)

		// 获取现有的 CronJob
		existing, err := client.CronJob().Get(cronJob.Namespace, cronJob.Name)
		if err != nil {
			return fmt.Errorf("获取现有 CronJob 失败: %v", err)
		}

		cronJob.ResourceVersion = existing.ResourceVersion

		_, err = client.CronJob().Update(cronJob)
		return err

	default:
		return fmt.Errorf("不支持的资源类型: %s", resourceType)
	}
}
