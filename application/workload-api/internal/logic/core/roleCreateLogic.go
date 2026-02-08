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

type RoleCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleCreateLogic {
	return &RoleCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleCreateLogic) RoleCreate(req *types.DefaultCoreCreateRequest) (resp string, err error) {
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

	// 解析 YAML
	var role rbacv1.Role
	if err := yaml.Unmarshal([]byte(req.YamlStr), &role); err != nil {
		l.Errorf("解析 Role YAML 失败: %v", err)
		return "", fmt.Errorf("解析 Role YAML 失败")
	}

	// 确保命名空间正确
	if role.Namespace == "" {
		role.Namespace = req.Namespace
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

	// 构建规则摘要用于审计
	rulesSummary := l.buildRulesSummary(role.Rules)

	// 创建 Role
	createErr := roleOp.Create(&role)
	if createErr != nil {
		l.Errorf("创建 Role 失败: %v", createErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "创建 Role",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 创建 Role %s 失败, 包含 %d 条规则 (%s), 错误原因: %v", username, role.Namespace, role.Name, len(role.Rules), rulesSummary, createErr),
			Status:       0,
		})
		return "", fmt.Errorf("创建 Role 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "创建 Role",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功创建 Role %s, 包含 %d 条规则 (%s)", username, role.Namespace, role.Name, len(role.Rules), rulesSummary),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("用户: %s, 成功创建 Role: %s/%s", username, role.Namespace, role.Name)
	return "创建 Role 成功", nil
}

// buildRulesSummary 构建规则摘要用于审计日志
func (l *RoleCreateLogic) buildRulesSummary(rules []rbacv1.PolicyRule) string {
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
	if len(result) > 200 {
		return result[:200] + "..."
	}
	return result
}
