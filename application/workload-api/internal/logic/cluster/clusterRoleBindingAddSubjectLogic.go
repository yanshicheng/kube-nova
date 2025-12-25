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

type ClusterRoleBindingAddSubjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 添加主体
func NewClusterRoleBindingAddSubjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleBindingAddSubjectLogic {
	return &ClusterRoleBindingAddSubjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleBindingAddSubjectLogic) ClusterRoleBindingAddSubject(req *types.AddSubjectRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	crbOp := client.ClusterRoleBindings()

	subject := rbacv1.Subject{
		Kind:      req.Subject.Kind,
		Name:      req.Subject.Name,
		Namespace: req.Subject.Namespace,
		APIGroup:  req.Subject.APIGroup,
	}

	err = crbOp.AddSubject(req.Name, subject)
	if err != nil {
		l.Errorf("添加主体失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "ClusterRoleBinding 添加主体",
			ActionDetail: fmt.Sprintf(" ClusterRoleBinding %s 添加主体 %s/%s 失败，错误: %v", req.Name, req.Subject.Kind, req.Subject.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("添加主体失败")
	}

	l.Infof("用户: %s, 成功为 ClusterRoleBinding %s 添加主体 %s/%s", username, req.Name, req.Subject.Kind, req.Subject.Name)
	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "ClusterRoleBinding 添加主体",
		ActionDetail: fmt.Sprintf("ClusterRoleBinding %s 添加主体 %s/%s 成功", req.Name, req.Subject.Kind, req.Subject.Name),
		Status:       1,
	})
	return "添加主体成功", nil
}
