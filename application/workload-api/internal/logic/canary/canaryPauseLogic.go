package canary

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CanaryPauseLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 暂停 Canary 金丝雀发布
func NewCanaryPauseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CanaryPauseLogic {
	return &CanaryPauseLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CanaryPauseLogic) CanaryPause(req *types.CanaryControlRequest) (resp string, err error) {
	// 获取工作空间信息
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

	// 暂停 Canary 发布
	canaryOperator := client.Flagger()
	err = canaryOperator.Pause(workload.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("暂停 Canary 失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  workload.Data.Id,
			Title:        "暂停 Canary 发布",
			ActionDetail: fmt.Sprintf("暂停 Canary 发布失败: %s, 错误: %v", req.Name, err),
			Status:       0,
		})

		return "", fmt.Errorf("暂停 Canary 失败: %v", err)
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  workload.Data.Id,
		Title:        "暂停 Canary 发布",
		ActionDetail: fmt.Sprintf("暂停 Canary 发布成功: %s", req.Name),
		Status:       1,
	})

	return fmt.Sprintf("成功暂停 Canary: %s", req.Name), nil
}
