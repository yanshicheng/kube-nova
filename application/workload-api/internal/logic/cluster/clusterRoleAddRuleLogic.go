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

	rule := rbacv1.PolicyRule{
		Verbs:           req.Rule.Verbs,
		APIGroups:       req.Rule.APIGroups,
		Resources:       req.Rule.Resources,
		ResourceNames:   req.Rule.ResourceNames,
		NonResourceURLs: req.Rule.NonResourceURLs,
	}

	err = crOp.AddRule(req.Name, rule)
	if err != nil {
		l.Errorf("添加 ClusterRole 规则失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "ClusterRole 添加规则",
			ActionDetail: fmt.Sprintf("为 ClusterRole %s 添加规则失败，错误: %v", req.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("添加 ClusterRole 规则失败")
	}

	l.Infof("用户: %s, 新增 ClusterRole %s 添加规则", username, req.Name)
	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "ClusterRole 添加规则",
		ActionDetail: fmt.Sprintf("为 ClusterRole %s 添加规则成功，资源: %v，动作: %v", req.Name, req.Rule.Resources, req.Rule.Verbs),
		Status:       1,
	})
	return "添加 ClusterRole 规则成功", nil
}
