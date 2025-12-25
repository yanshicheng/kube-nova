package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterRoleBindingRemoveSubjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 移除主体
func NewClusterRoleBindingRemoveSubjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleBindingRemoveSubjectLogic {
	return &ClusterRoleBindingRemoveSubjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleBindingRemoveSubjectLogic) ClusterRoleBindingRemoveSubject(req *types.RemoveSubjectRequest) (resp string, err error) {
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
	err = crbOp.RemoveSubject(req.Name, req.SubjectKind, req.SubjectName, req.SubjectNamespace)
	if err != nil {
		l.Errorf("移除主体失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "ClusterRoleBinding 移除主体",
			ActionDetail: fmt.Sprintf("从 ClusterRoleBinding %s 移除主体 %s/%s 失败，错误: %v", req.Name, req.SubjectKind, req.SubjectName, err),
			Status:       0,
		})
		return "", fmt.Errorf("移除主体失败")
	}

	l.Infof("用户: %s, 成功从 ClusterRoleBinding %s 移除主体 %s/%s", username, req.Name, req.SubjectKind, req.SubjectName)
	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "ClusterRoleBinding 移除主体",
		ActionDetail: fmt.Sprintf("从 ClusterRoleBinding %s 移除主体 %s/%s 成功", req.Name, req.SubjectKind, req.SubjectName),
		Status:       1,
	})
	return "移除主体成功", nil
}
