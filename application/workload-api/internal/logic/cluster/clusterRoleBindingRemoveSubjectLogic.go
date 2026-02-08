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

	// 获取当前 ClusterRoleBinding 用于审计
	existingCRB, _ := crbOp.Get(req.Name)
	var oldSubjectCount int
	var roleRef string
	if existingCRB != nil {
		oldSubjectCount = len(existingCRB.Subjects)
		roleRef = fmt.Sprintf("%s/%s", existingCRB.RoleRef.Kind, existingCRB.RoleRef.Name)
	}

	// 格式化被移除主体用于审计
	var subjectDetail string
	if req.SubjectNamespace != "" {
		subjectDetail = fmt.Sprintf("%s/%s/%s", req.SubjectKind, req.SubjectNamespace, req.SubjectName)
	} else {
		subjectDetail = fmt.Sprintf("%s/%s", req.SubjectKind, req.SubjectName)
	}

	err = crbOp.RemoveSubject(req.Name, req.SubjectKind, req.SubjectName, req.SubjectNamespace)
	if err != nil {
		l.Errorf("移除主体失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "ClusterRoleBinding 移除主体",
			ActionDetail: fmt.Sprintf("用户 %s 从 ClusterRoleBinding %s 移除主体 %s 失败, 错误: %v", username, req.Name, subjectDetail, err),
			Status:       0,
		})
		return "", fmt.Errorf("移除主体失败")
	}

	l.Infof("用户: %s, 成功从 ClusterRoleBinding %s 移除主体 %s/%s", username, req.Name, req.SubjectKind, req.SubjectName)

	// 构建详细审计信息
	auditDetail := fmt.Sprintf("用户 %s 成功从 ClusterRoleBinding %s 移除主体, 绑定角色: %s, 主体数量: %d → %d, 移除主体: %s",
		username, req.Name, roleRef, oldSubjectCount, oldSubjectCount-1, subjectDetail)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "ClusterRoleBinding 移除主体",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "移除主体成功", nil
}
