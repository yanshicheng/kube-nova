package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterRoleBindingDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 ClusterRoleBinding
func NewClusterRoleBindingDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleBindingDeleteLogic {
	return &ClusterRoleBindingDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleBindingDeleteLogic) ClusterRoleBindingDelete(req *types.ClusterResourceDeleteRequest) (resp string, err error) {
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

	// 获取 ClusterRoleBinding 详情用于审计
	existingCRB, _ := crbOp.Get(req.Name)
	var roleRef string
	var subjectCount int
	var subjectsList string
	if existingCRB != nil {
		roleRef = fmt.Sprintf("%s/%s", existingCRB.RoleRef.Kind, existingCRB.RoleRef.Name)
		subjectCount = len(existingCRB.Subjects)
		subjectsList = FormatSubjectsList(existingCRB.Subjects)
	}

	err = crbOp.Delete(req.Name)
	if err != nil {
		l.Errorf("删除 ClusterRoleBinding 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "删除 ClusterRoleBinding",
			ActionDetail: fmt.Sprintf("用户 %s 删除 ClusterRoleBinding %s 失败, 错误: %v", username, req.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("删除 ClusterRoleBinding 失败")
	}

	l.Infof("用户: %s, 成功删除 ClusterRoleBinding: %s", username, req.Name)

	// 构建详细审计信息
	auditDetail := fmt.Sprintf("用户 %s 成功删除 ClusterRoleBinding %s, 绑定角色: %s, 主体数量: %d, 主体详情: %s",
		username, req.Name, roleRef, subjectCount, subjectsList)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "删除 ClusterRoleBinding",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "删除 ClusterRoleBinding 成功", nil
}
