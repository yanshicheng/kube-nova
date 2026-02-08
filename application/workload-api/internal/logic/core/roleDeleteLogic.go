package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	rbacv1 "k8s.io/api/rbac/v1"
)

type RoleDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleDeleteLogic {
	return &RoleDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleDeleteLogic) RoleDelete(req *types.ClusterNamespaceResourceRequest) (resp string, err error) {
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

	// 获取 Role 详情用于审计
	existingRole, _ := roleOp.Get(req.Namespace, req.Name)
	var rulesCount int
	var rulesSummary string
	if existingRole != nil {
		rulesCount = len(existingRole.Rules)
		rulesSummary = l.buildRulesSummary(existingRole.Rules)
	}

	// 删除 Role
	deleteErr := roleOp.Delete(req.Namespace, req.Name)
	if deleteErr != nil {
		l.Errorf("删除 Role 失败: %v", deleteErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "删除 Role",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 删除 Role %s 失败, 错误原因: %v", username, req.Namespace, req.Name, deleteErr),
			Status:       0,
		})
		return "", fmt.Errorf("删除 Role 失败")
	}

	// 记录成功的审计日志
	auditDetail := fmt.Sprintf("用户 %s 在命名空间 %s 成功删除 Role %s", username, req.Namespace, req.Name)
	if rulesCount > 0 {
		auditDetail = fmt.Sprintf("%s, 删除前包含 %d 条规则 (%s)", auditDetail, rulesCount, rulesSummary)
	}

	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "删除 Role",
		ActionDetail: auditDetail,
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("用户: %s, 成功删除 Role: %s/%s", username, req.Namespace, req.Name)
	return "删除 Role 成功", nil
}

// buildRulesSummary 构建规则摘要用于审计日志
func (l *RoleDeleteLogic) buildRulesSummary(rules []rbacv1.PolicyRule) string {
	if len(rules) == 0 {
		return "无规则"
	}

	var summaries []string
	for _, rule := range rules {
		resources := strings.Join(rule.Resources, ",")
		verbs := strings.Join(rule.Verbs, ",")
		if resources == "" {
			resources = "*"
		}
		summaries = append(summaries, fmt.Sprintf("%s[%s]", resources, verbs))
	}

	result := strings.Join(summaries, "; ")
	if len(result) > 150 {
		return result[:150] + "..."
	}
	return result
}
