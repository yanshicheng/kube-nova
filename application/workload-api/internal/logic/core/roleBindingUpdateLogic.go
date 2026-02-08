package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleBindingUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleBindingUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleBindingUpdateLogic {
	return &RoleBindingUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleBindingUpdateLogic) RoleBindingUpdate(req *types.ClusterNamespaceResourceUpdateRequest) (resp string, err error) {
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

	// 获取 RoleBinding operator
	rbOp := client.RoleBindings()

	// 获取现有 RoleBinding 用于对比
	existingRB, err := rbOp.Get(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取现有 RoleBinding 失败: %v", err)
		return "", fmt.Errorf("获取现有 RoleBinding 失败")
	}

	// 解析新的 YAML
	var rb rbacv1.RoleBinding
	if err := yaml.Unmarshal([]byte(req.YamlStr), &rb); err != nil {
		l.Errorf("解析 RoleBinding YAML 失败: %v", err)
		return "", fmt.Errorf("解析 RoleBinding YAML 失败")
	}

	// 确保命名空间正确
	if rb.Namespace == "" {
		rb.Namespace = req.Namespace
	}

	// 对比主体变更
	subjectsDiff := l.compareSubjects(existingRB.Subjects, rb.Subjects)

	// 对比 RoleRef 变更（注意：RoleRef 通常是不可变的，但我们仍然记录）
	var roleRefChange string
	if existingRB.RoleRef.Kind != rb.RoleRef.Kind || existingRB.RoleRef.Name != rb.RoleRef.Name {
		roleRefChange = fmt.Sprintf("角色引用变更: %s/%s → %s/%s", existingRB.RoleRef.Kind, existingRB.RoleRef.Name, rb.RoleRef.Kind, rb.RoleRef.Name)
	}

	// Labels 变更
	labelsDiff := CompareStringMaps(existingRB.Labels, rb.Labels)
	labelsChangeDetail := BuildMapDiffDetail(labelsDiff, false)

	// 构建变更详情
	var changeDetails []string
	if subjectsDiff != "" {
		changeDetails = append(changeDetails, subjectsDiff)
	}
	if roleRefChange != "" {
		changeDetails = append(changeDetails, roleRefChange)
	}
	if HasMapChanges(labelsDiff) {
		changeDetails = append(changeDetails, fmt.Sprintf("Labels变更: %s", labelsChangeDetail))
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
		utils.AddAnnotations(&rb.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   rb.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}

	// 更新 RoleBinding
	updateErr := rbOp.Update(req.Namespace, req.Name, &rb)
	if updateErr != nil {
		l.Errorf("更新 RoleBinding 失败: %v", updateErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "更新 RoleBinding",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 更新 RoleBinding %s 失败, 错误原因: %v, 变更内容: %s", username, req.Namespace, req.Name, updateErr, changeDetailStr),
			Status:       0,
		})
		return "", fmt.Errorf("更新 RoleBinding 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "更新 RoleBinding",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功更新 RoleBinding %s, 绑定角色: %s/%s, %s", username, req.Namespace, req.Name, rb.RoleRef.Kind, rb.RoleRef.Name, changeDetailStr),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("用户: %s, 成功更新 RoleBinding: %s/%s", username, req.Namespace, rb.Name)
	return "更新 RoleBinding 成功", nil
}

// compareSubjects 对比主体变更
func (l *RoleBindingUpdateLogic) compareSubjects(oldSubjects, newSubjects []rbacv1.Subject) string {
	oldMap := make(map[string]bool)
	newMap := make(map[string]bool)

	for _, s := range oldSubjects {
		key := l.subjectKey(s)
		oldMap[key] = true
	}
	for _, s := range newSubjects {
		key := l.subjectKey(s)
		newMap[key] = true
	}

	var added, deleted []string

	for key := range newMap {
		if !oldMap[key] {
			added = append(added, key)
		}
	}

	for key := range oldMap {
		if !newMap[key] {
			deleted = append(deleted, key)
		}
	}

	var parts []string
	if len(added) > 0 {
		parts = append(parts, fmt.Sprintf("新增主体 [%s]", strings.Join(added, ", ")))
	}
	if len(deleted) > 0 {
		parts = append(parts, fmt.Sprintf("删除主体 [%s]", strings.Join(deleted, ", ")))
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

// subjectKey 生成主体的唯一标识
func (l *RoleBindingUpdateLogic) subjectKey(s rbacv1.Subject) string {
	if s.Namespace != "" {
		return fmt.Sprintf("%s/%s/%s", s.Kind, s.Namespace, s.Name)
	}
	return fmt.Sprintf("%s/%s", s.Kind, s.Name)
}
