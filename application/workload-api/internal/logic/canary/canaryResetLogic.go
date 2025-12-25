package canary

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CanaryResetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重置 Canary 状态
func NewCanaryResetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CanaryResetLogic {
	return &CanaryResetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CanaryResetLogic) CanaryReset(req *types.CanaryControlRequest) (resp string, err error) {
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

	// 获取现有的 Canary 资源
	canary, err := canaryOperator.Get(workload.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Canary 资源失败: %v", err)
		return "", fmt.Errorf("获取 Canary 资源失败: %v", err)
	}

	// 重置状态的方式是通过添加特殊注解触发 Flagger 控制器重置
	if canary.Annotations == nil {
		canary.Annotations = make(map[string]string)
	}

	// 添加重置注解（Flagger 会监听这个注解）
	canary.Annotations["flagger.app/reset-status"] = "true"

	// 更新 Canary
	_, err = canaryOperator.Update(canary)
	if err != nil {
		l.Errorf("重置 Canary 状态失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  workload.Data.Id,
			Title:        "重置 Canary 状态",
			ActionDetail: fmt.Sprintf("重置 Canary 状态失败: %s, 错误: %v", req.Name, err),
			Status:       0,
		})

		return "", fmt.Errorf("重置 Canary 状态失败: %v", err)
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  workload.Data.Id,
		Title:        "重置 Canary 状态",
		ActionDetail: fmt.Sprintf("重置 Canary 状态成功: %s", req.Name),
		Status:       1,
	})

	return fmt.Sprintf("Canary %s 重置成功，Flagger 将重新开始金丝雀发布流程", req.Name), nil
}
