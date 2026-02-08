package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterRoleDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 ClusterRole
func NewClusterRoleDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleDeleteLogic {
	return &ClusterRoleDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleDeleteLogic) ClusterRoleDelete(req *types.ClusterResourceDeleteRequest) (resp string, err error) {
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

	// 获取 ClusterRole 详情用于审计
	existingCR, _ := crOp.Get(req.Name)
	var rulesSummary string
	var ruleCount int
	if existingCR != nil {
		ruleCount = len(existingCR.Rules)
		rulesSummary = FormatPolicyRulesShort(existingCR.Rules)
	}

	// 获取使用情况
	usage, _ := crOp.GetUsage(req.Name)
	var usageInfo string
	if usage != nil && (usage.BindingCount > 0 || usage.RoleBindingCount > 0) {
		usageInfo = fmt.Sprintf(", 关联绑定: ClusterRoleBinding %d 个, RoleBinding %d 个",
			usage.BindingCount, usage.RoleBindingCount)
	}

	err = crOp.Delete(req.Name)
	if err != nil {
		l.Errorf("删除 ClusterRole 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "删除 ClusterRole",
			ActionDetail: fmt.Sprintf("用户 %s 删除 ClusterRole %s 失败, 错误: %v", username, req.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("删除 ClusterRole 失败")
	}

	l.Infof("用户: %s, 成功删除 ClusterRole: %s", username, req.Name)

	// 构建详细审计信息
	auditDetail := fmt.Sprintf("用户 %s 成功删除 ClusterRole %s, 规则数量: %d, 规则摘要: %s%s",
		username, req.Name, ruleCount, rulesSummary, usageInfo)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "删除 ClusterRole",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "删除 ClusterRole 成功", nil
}
