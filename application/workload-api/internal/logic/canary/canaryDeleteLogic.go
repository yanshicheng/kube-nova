package canary

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CanaryDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 Canary 资源
func NewCanaryDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CanaryDeleteLogic {
	return &CanaryDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CanaryDeleteLogic) CanaryDelete(req *types.DeleteCanaryRequest) (resp string, err error) {
	workload, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{
		Id: req.WorkloadId,
	})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return "", fmt.Errorf("获取项目工作空间详情失败: %v", err)
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workload.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败: %v", err)
	}

	canaryOperator := client.Flagger()
	if err := canaryOperator.Delete(workload.Data.Namespace, req.Name); err != nil {
		l.Errorf("删除 Canary 资源失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  workload.Data.Id,
			Title:        "删除 Canary",
			ActionDetail: fmt.Sprintf("删除 Canary 资源失败: %s, 错误: %v", req.Name, err),
			Status:       0,
		})

		return "", fmt.Errorf("删除 Canary 资源失败: %v", err)
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  workload.Data.Id,
		Title:        "删除 Canary",
		ActionDetail: fmt.Sprintf("删除 Canary 资源: %s", req.Name),
		Status:       1,
	})

	return fmt.Sprintf("Canary 资源 %s 删除成功", req.Name), nil
}
