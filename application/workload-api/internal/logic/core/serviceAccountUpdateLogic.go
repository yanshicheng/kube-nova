package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceAccountUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceAccountUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceAccountUpdateLogic {
	return &ServiceAccountUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceAccountUpdateLogic) ServiceAccountUpdate(req *types.ClusterNamespaceResourceUpdateRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 获取 ServiceAccount operator
	saOp := client.ServiceAccounts()

	// 获取现有 ServiceAccount 用于对比
	existingSA, err := saOp.Get(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取现有 ServiceAccount 失败: %v", err)
		return "", fmt.Errorf("获取现有 ServiceAccount 失败")
	}

	// 解析 YAML 以获取资源名称
	var sa corev1.ServiceAccount
	if err := yaml.Unmarshal([]byte(req.YamlStr), &sa); err != nil {
		l.Errorf("解析 ServiceAccount YAML 失败: %v", err)
		return "", fmt.Errorf("解析 ServiceAccount YAML 失败")
	}

	// 确保命名空间正确
	if sa.Namespace == "" {
		sa.Namespace = req.Namespace
	}

	// 对比 Secrets 变更
	oldSecrets := l.extractSecretNames(existingSA.Secrets)
	newSecrets := l.extractSecretNames(sa.Secrets)
	secretsAdded, secretsDeleted := CompareStringSlices(oldSecrets, newSecrets)
	secretsChangeDetail := BuildSliceDiffDetail("Secrets", secretsAdded, secretsDeleted)

	// 对比 ImagePullSecrets 变更
	oldImagePullSecrets := l.extractImagePullSecretNames(existingSA.ImagePullSecrets)
	newImagePullSecrets := l.extractImagePullSecretNames(sa.ImagePullSecrets)
	imagePullSecretsAdded, imagePullSecretsDeleted := CompareStringSlices(oldImagePullSecrets, newImagePullSecrets)
	imagePullSecretsChangeDetail := BuildSliceDiffDetail("ImagePullSecrets", imagePullSecretsAdded, imagePullSecretsDeleted)

	// Labels 变更
	labelsDiff := CompareStringMaps(existingSA.Labels, sa.Labels)
	labelsChangeDetail := BuildMapDiffDetail(labelsDiff, false)

	// Annotations 变更
	annotationsDiff := CompareStringMaps(existingSA.Annotations, sa.Annotations)
	annotationsChangeDetail := BuildMapDiffDetail(annotationsDiff, false)

	// 构建变更详情
	var changeDetails []string
	if secretsChangeDetail != "" {
		changeDetails = append(changeDetails, secretsChangeDetail)
	}
	if imagePullSecretsChangeDetail != "" {
		changeDetails = append(changeDetails, imagePullSecretsChangeDetail)
	}
	if HasMapChanges(labelsDiff) {
		changeDetails = append(changeDetails, fmt.Sprintf("Labels变更: %s", labelsChangeDetail))
	}
	if HasMapChanges(annotationsDiff) {
		changeDetails = append(changeDetails, fmt.Sprintf("Annotations变更: %s", annotationsChangeDetail))
	}

	changeDetailStr := "无变更"
	if len(changeDetails) > 0 {
		changeDetailStr = strings.Join(changeDetails, "; ")
	}

	// 获取项目详情
	projectDetail, err := l.svcCtx.ManagerRpc.GetClusterNsDetail(l.ctx, &managerservice.GetClusterNsDetailReq{
		ClusterUuid: req.ClusterUuid,
		Namespace:   req.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	} else {
		// 注入注解
		utils.AddAnnotations(&sa.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   sa.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}

	// 更新 ServiceAccount
	updateErr := saOp.Update(req.Namespace, req.Name, &sa)
	if updateErr != nil {
		l.Errorf("更新 ServiceAccount 失败: %v", updateErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "更新 ServiceAccount",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 更新 ServiceAccount %s 失败, 错误原因: %v, 变更内容: %s", username, req.Namespace, req.Name, updateErr, changeDetailStr),
			Status:       0,
		})
		return "", fmt.Errorf("更新 ServiceAccount 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "更新 ServiceAccount",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功更新 ServiceAccount %s, %s", username, req.Namespace, req.Name, changeDetailStr),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("用户: %s, 成功更新 ServiceAccount: %s/%s", username, req.Namespace, sa.Name)
	return "更新 ServiceAccount 成功", nil
}

// extractSecretNames 提取 Secret 名称列表
func (l *ServiceAccountUpdateLogic) extractSecretNames(secrets []corev1.ObjectReference) []string {
	var names []string
	for _, s := range secrets {
		names = append(names, s.Name)
	}
	return names
}

// extractImagePullSecretNames 提取 ImagePullSecret 名称列表
func (l *ServiceAccountUpdateLogic) extractImagePullSecretNames(secrets []corev1.LocalObjectReference) []string {
	var names []string
	for _, s := range secrets {
		names = append(names, s.Name)
	}
	return names
}
