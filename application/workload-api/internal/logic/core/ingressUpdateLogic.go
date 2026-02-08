package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type IngressUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 Ingress
func NewIngressUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngressUpdateLogic {
	return &IngressUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngressUpdateLogic) IngressUpdate(req *types.IngressRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取集群以及命名空间
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return "", fmt.Errorf("获取项目工作空间详情失败")
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 初始化 Ingress 客户端
	ingressClient := client.Ingresses()

	// 获取现有的 Ingress
	existing, err := ingressClient.Get(workloadInfo.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取现有 Ingress 失败: %v", err)
		return "", fmt.Errorf("获取现有 Ingress 失败: %v", err)
	}

	// 解析新的 YAML
	var newIngress networkingv1.Ingress
	if err := yaml.Unmarshal([]byte(req.IngressYamlStr), &newIngress); err != nil {
		l.Errorf("解析 Ingress YAML 失败: %v", err)
		return "", fmt.Errorf("解析 Ingress YAML 失败: %v", err)
	}

	// 保留重要的元数据
	newIngress.ResourceVersion = existing.ResourceVersion
	newIngress.UID = existing.UID
	if newIngress.Namespace == "" {
		newIngress.Namespace = workloadInfo.Data.Namespace
	}

	// 对比变更
	ruleDiff := CompareIngressRules(existing, &newIngress)
	ruleChangeDetail := BuildIngressDiffDetail(ruleDiff)

	// Labels 变更
	labelsDiff := CompareStringMaps(existing.Labels, newIngress.Labels)
	labelsChangeDetail := BuildMapDiffDetail(labelsDiff, false)

	// Annotations 变更
	annotationsDiff := CompareStringMaps(existing.Annotations, newIngress.Annotations)
	annotationsChangeDetail := BuildMapDiffDetail(annotationsDiff, false)

	// 构建变更详情
	var changeDetails []string
	if ruleChangeDetail != "规则无变更" && ruleChangeDetail != "" {
		changeDetails = append(changeDetails, ruleChangeDetail)
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
		ClusterUuid: workloadInfo.Data.ClusterUuid,
		Namespace:   workloadInfo.Data.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	} else {
		// 注入注解
		utils.AddAnnotations(&newIngress.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   newIngress.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}

	// 更新 Ingress
	updated, updateErr := ingressClient.Update(&newIngress)
	if updateErr != nil {
		l.Errorf("更新 Ingress 失败: %v", updateErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  req.WorkloadId,
			Title:        "更新 Ingress",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 更新 Ingress %s 失败, 错误原因: %v, 变更内容: %s", username, workloadInfo.Data.Namespace, req.Name, updateErr, changeDetailStr),
			Status:       0,
		})
		return "", fmt.Errorf("更新 Ingress 失败: %v", updateErr)
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  req.WorkloadId,
		Title:        "更新 Ingress",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功更新 Ingress %s, %s", username, updated.Namespace, updated.Name, changeDetailStr),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("成功更新 Ingress: %s/%s", updated.Namespace, updated.Name)
	return fmt.Sprintf("成功更新 Ingress: %s", updated.Name), nil
}
