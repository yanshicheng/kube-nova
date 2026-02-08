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

type RoleBindingDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleBindingDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleBindingDeleteLogic {
	return &RoleBindingDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleBindingDeleteLogic) RoleBindingDelete(req *types.ClusterNamespaceResourceRequest) (resp string, err error) {
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

	// 获取 RoleBinding 详情用于审计
	existingRB, _ := rbOp.Get(req.Namespace, req.Name)
	var roleRef string
	var subjectsSummary string
	var subjectsCount int
	if existingRB != nil {
		roleRef = fmt.Sprintf("%s/%s", existingRB.RoleRef.Kind, existingRB.RoleRef.Name)
		subjectsCount = len(existingRB.Subjects)
		subjectsSummary = l.buildSubjectsSummary(existingRB.Subjects)
	}

	// 删除 RoleBinding
	deleteErr := rbOp.Delete(req.Namespace, req.Name)
	if deleteErr != nil {
		l.Errorf("删除 RoleBinding 失败: %v", deleteErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "删除 RoleBinding",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 删除 RoleBinding %s 失败, 绑定角色: %s, 错误原因: %v", username, req.Namespace, req.Name, roleRef, deleteErr),
			Status:       0,
		})
		return "", fmt.Errorf("删除 RoleBinding 失败")
	}

	// 记录成功的审计日志
	auditDetail := fmt.Sprintf("用户 %s 在命名空间 %s 成功删除 RoleBinding %s, 绑定角色: %s, 包含 %d 个主体 (%s)", username, req.Namespace, req.Name, roleRef, subjectsCount, subjectsSummary)

	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "删除 RoleBinding",
		ActionDetail: auditDetail,
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("用户: %s, 成功删除 RoleBinding: %s/%s", username, req.Namespace, req.Name)
	return "删除 RoleBinding 成功", nil
}

// buildSubjectsSummary 构建主体摘要用于审计日志
func (l *RoleBindingDeleteLogic) buildSubjectsSummary(subjects []rbacv1.Subject) string {
	if len(subjects) == 0 {
		return "无"
	}

	var summaries []string
	for _, s := range subjects {
		if s.Namespace != "" {
			summaries = append(summaries, fmt.Sprintf("%s/%s/%s", s.Kind, s.Namespace, s.Name))
		} else {
			summaries = append(summaries, fmt.Sprintf("%s/%s", s.Kind, s.Name))
		}
	}

	result := strings.Join(summaries, ", ")
	if len(result) > 100 {
		return result[:100] + "..."
	}
	return result
}
