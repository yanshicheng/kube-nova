package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterRoleRemoveRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 移除 ClusterRole 规则
func NewClusterRoleRemoveRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleRemoveRuleLogic {
	return &ClusterRoleRemoveRuleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleRemoveRuleLogic) ClusterRoleRemoveRule(req *types.RemoveClusterRoleRuleRequest) (resp string, err error) {
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

	// 获取当前 ClusterRole 用于审计
	existingCR, err := crOp.Get(req.Name)
	if err != nil {
		l.Errorf("获取 ClusterRole 失败: %v", err)
		return "", fmt.Errorf("获取 ClusterRole 失败")
	}

	oldRuleCount := len(existingCR.Rules)

	// 获取被移除规则的详情
	var removedRuleDetail string
	if req.RuleIndex >= 0 && req.RuleIndex < len(existingCR.Rules) {
		removedRuleDetail = FormatPolicyRule(existingCR.Rules[req.RuleIndex])
	} else {
		removedRuleDetail = fmt.Sprintf("索引 %d (无效)", req.RuleIndex)
	}

	err = crOp.RemoveRule(req.Name, req.RuleIndex)
	if err != nil {
		l.Errorf("移除 ClusterRole 规则失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "ClusterRole 移除规则",
			ActionDetail: fmt.Sprintf("用户 %s 移除 ClusterRole %s 的规则失败, 规则索引: %d, 规则: %s, 错误: %v", username, req.Name, req.RuleIndex, removedRuleDetail, err),
			Status:       0,
		})
		return "", fmt.Errorf("移除 ClusterRole 规则失败")
	}

	l.Infof("用户: %s, 成功移除 ClusterRole %s 的规则 (索引: %d)", username, req.Name, req.RuleIndex)

	// 构建详细审计信息
	auditDetail := fmt.Sprintf("用户 %s 成功移除 ClusterRole %s 的规则, 规则数量: %d → %d, 移除规则(索引 %d): %s",
		username, req.Name, oldRuleCount, oldRuleCount-1, req.RuleIndex, removedRuleDetail)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "ClusterRole 移除规则",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "移除 ClusterRole 规则成功", nil
}
