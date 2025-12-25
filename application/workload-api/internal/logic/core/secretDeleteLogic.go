// secretDeleteLogic.go
package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type SecretDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 Secret
func NewSecretDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SecretDeleteLogic {
	return &SecretDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SecretDeleteLogic) SecretDelete(req *types.DefaultNameRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取集群以及命名空间
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return "", fmt.Errorf("获取项目工作空间详情失败")
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 初始化 Secret 客户端
	secretClient := client.Secrets()

	// 删除 Secret
	deleteErr := secretClient.Delete(workloadInfo.Data.Namespace, req.Name)
	if deleteErr != nil {
		l.Errorf("删除 Secret 失败: %v", deleteErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  req.WorkloadId,
			Title:        "删除 Secret",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 删除 Secret %s 失败, 错误原因: %v", username, workloadInfo.Data.Namespace, req.Name, deleteErr),
			Status:       0,
		})
		return "", fmt.Errorf("删除 Secret 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  req.WorkloadId,
		Title:        "删除 Secret",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功删除 Secret %s", username, workloadInfo.Data.Namespace, req.Name),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("成功删除 Secret: %s", req.Name)
	return "删除成功", nil
}
