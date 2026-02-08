package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterRoleAddRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 添加 ClusterRole 规则
func NewClusterRoleAddRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleAddRuleLogic {
	return &ClusterRoleAddRuleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleAddRuleLogic) ClusterRoleAddRule(req *types.AddClusterRoleRuleRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	crOp := client.ClusterRoles()

	// 获取当前规则数量用于审计
	existingCR, _ := crOp.Get(req.Name)
	var oldRuleCount int
	if existingCR != nil {
		oldRuleCount = len(existingCR.Rules)
	}

	rule := rbacv1.PolicyRule{
		Verbs:           req.Rule.Verbs,
		APIGroups:       req.Rule.APIGroups,
		Resources:       req.Rule.Resources,
		ResourceNames:   req.Rule.ResourceNames,
		NonResourceURLs: req.Rule.NonResourceURLs,
	}

	// 格式化新规则用于审计
	ruleDetail := FormatPolicyRule(rule)

	err = crOp.AddRule(req.Name, rule)
	if err != nil {
		l.Errorf("添加 ClusterRole 规则失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "ClusterRole 添加规则",
			ActionDetail: fmt.Sprintf("用户 %s 为 ClusterRole %s 添加规则失败, 规则: %s, 错误: %v", username, req.Name, ruleDetail, err),
			Status:       0,
		})
		return "", fmt.Errorf("添加 ClusterRole 规则失败")
	}

	l.Infof("用户: %s, 成功为 ClusterRole %s 添加规则", username, req.Name)

	// 构建详细审计信息
	auditDetail := fmt.Sprintf("用户 %s 成功为 ClusterRole %s 添加规则, 规则数量: %d → %d, 新增规则: %s",
		username, req.Name, oldRuleCount, oldRuleCount+1, ruleDetail)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "ClusterRole 添加规则",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "添加 ClusterRole 规则成功", nil
}
