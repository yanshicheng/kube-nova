// roleUpdateLogic.go
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

type RoleUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleUpdateLogic {
	return &RoleUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleUpdateLogic) RoleUpdate(req *types.ClusterNamespaceResourceUpdateRequest) (resp string, err error) {
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

	// 获取 Role operator
	roleOp := client.Roles()

	// 获取现有 Role 用于对比
	existingRole, err := roleOp.Get(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取现有 Role 失败: %v", err)
		return "", fmt.Errorf("获取现有 Role 失败")
	}

	// 解析新的 YAML
	var role rbacv1.Role
	if err := yaml.Unmarshal([]byte(req.YamlStr), &role); err != nil {
		l.Errorf("解析 Role YAML 失败: %v", err)
		return "", fmt.Errorf("解析 Role YAML 失败")
	}

	// 确保命名空间正确
	if role.Namespace == "" {
		role.Namespace = req.Namespace
	}

	// 对比规则变更
	rulesDiff := l.compareRoles(existingRole.Rules, role.Rules)

	// Labels 变更
	labelsDiff := CompareStringMaps(existingRole.Labels, role.Labels)
	labelsChangeDetail := BuildMapDiffDetail(labelsDiff, false)

	// 构建变更详情
	var changeDetails []string
	if rulesDiff != "" {
		changeDetails = append(changeDetails, fmt.Sprintf("规则变更: %s", rulesDiff))
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
		utils.AddAnnotations(&role.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   role.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}

	// 更新 Role
	updateErr := roleOp.Update(req.Namespace, req.Name, &role)
	if updateErr != nil {
		l.Errorf("更新 Role 失败: %v", updateErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "更新 Role",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 更新 Role %s 失败, 错误原因: %v, 变更内容: %s", username, req.Namespace, req.Name, updateErr, changeDetailStr),
			Status:       0,
		})
		return "", fmt.Errorf("更新 Role 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "更新 Role",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功更新 Role %s, %s", username, req.Namespace, req.Name, changeDetailStr),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("用户: %s, 成功更新 Role: %s/%s", username, req.Namespace, role.Name)
	return "更新 Role 成功", nil
}

// compareRoles 对比 Role 规则变更
func (l *RoleUpdateLogic) compareRoles(oldRules, newRules []rbacv1.PolicyRule) string {
	oldRuleMap := make(map[string]rbacv1.PolicyRule)
	newRuleMap := make(map[string]rbacv1.PolicyRule)

	for _, r := range oldRules {
		key := l.ruleKey(r)
		oldRuleMap[key] = r
	}
	for _, r := range newRules {
		key := l.ruleKey(r)
		newRuleMap[key] = r
	}

	var added, deleted []string

	// 检查新增
	for key := range newRuleMap {
		if _, exists := oldRuleMap[key]; !exists {
			added = append(added, key)
		}
	}

	// 检查删除
	for key := range oldRuleMap {
		if _, exists := newRuleMap[key]; !exists {
			deleted = append(deleted, key)
		}
	}

	var parts []string
	if len(added) > 0 {
		parts = append(parts, fmt.Sprintf("新增规则 [%s]", strings.Join(added, ", ")))
	}
	if len(deleted) > 0 {
		parts = append(parts, fmt.Sprintf("删除规则 [%s]", strings.Join(deleted, ", ")))
	}

	if len(parts) == 0 {
		// 检查是否有修改（资源相同但 verbs 不同）
		for key, newRule := range newRuleMap {
			if oldRule, exists := oldRuleMap[key]; exists {
				if !l.sameVerbs(oldRule.Verbs, newRule.Verbs) {
					parts = append(parts, fmt.Sprintf("修改规则 [%s: verbs %v → %v]", key, oldRule.Verbs, newRule.Verbs))
				}
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

// ruleKey 生成规则的唯一标识
func (l *RoleUpdateLogic) ruleKey(r rbacv1.PolicyRule) string {
	return fmt.Sprintf("%s/%s", strings.Join(r.APIGroups, ","), strings.Join(r.Resources, ","))
}

// sameVerbs 检查两个 verbs 列表是否相同
func (l *RoleUpdateLogic) sameVerbs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]bool)
	for _, v := range a {
		aMap[v] = true
	}
	for _, v := range b {
		if !aMap[v] {
			return false
		}
	}
	return true
}
