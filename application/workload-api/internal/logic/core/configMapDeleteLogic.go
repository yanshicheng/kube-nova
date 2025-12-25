package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigMapDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 ConfigMap
func NewConfigMapDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigMapDeleteLogic {
	return &ConfigMapDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigMapDeleteLogic) ConfigMapDelete(req *types.DefaultNameRequest) (resp string, err error) {

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

	// 初始化 ConfigMap 客户端
	configMapClient := client.ConfigMaps()

	// 删除 ConfigMap
	deleteErr := configMapClient.Delete(workloadInfo.Data.Namespace, req.Name)
	if deleteErr != nil {
		l.Errorf("删除 ConfigMap 失败: %v", deleteErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  req.WorkloadId,
			Title:        "删除 ConfigMap",
			ActionDetail: fmt.Sprintf(" 在命名空间 %s 删除 ConfigMap %s 失败, 错误原因: %v", workloadInfo.Data.Namespace, req.Name, deleteErr),
			Status:       0,
		})
		return "", fmt.Errorf("删除 ConfigMap 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  req.WorkloadId,
		Title:        "删除 ConfigMap",
		ActionDetail: fmt.Sprintf(" 在命名空间 %s 成功删除 ConfigMap %s", workloadInfo.Data.Namespace, req.Name),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("成功删除 ConfigMap: %s", req.Name)
	return "删除成功", nil
}
