package canary

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CanaryGetStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Canary 状态信息
func NewCanaryGetStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CanaryGetStatusLogic {
	return &CanaryGetStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CanaryGetStatusLogic) CanaryGetStatus(req *types.CanaryNameRequest) (resp *types.CanaryStatusResponse, err error) {
	workload, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{
		Id: req.WorkloadId,
	})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败: %v", err)
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workload.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败: %v", err)
	}

	canaryOperator := client.Flagger()
	canary, err := canaryOperator.Get(workload.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Canary 状态失败: %v", err)
		return nil, fmt.Errorf("获取 Canary 状态失败: %v", err)
	}

	// 转换状态信息
	status := &types.CanaryStatusResponse{
		Name:            canary.Name,
		Namespace:       canary.Namespace,
		Phase:           string(canary.Status.Phase),
		CanaryWeight:    canary.Status.CanaryWeight,
		FailedChecks:    canary.Status.FailedChecks,
		Iterations:      canary.Status.Iterations,
		LastAppliedSpec: canary.Status.LastAppliedSpec,
	}

	if canary.Status.TrackedConfigs != nil {
		status.TrackedConfigs = *canary.Status.TrackedConfigs
	} else {
		status.TrackedConfigs = make(map[string]string)
	}

	// 转换 Conditions
	if len(canary.Status.Conditions) > 0 {
		conditions := make([]types.CanaryStatusCondition, 0, len(canary.Status.Conditions))
		for _, cond := range canary.Status.Conditions {
			conditions = append(conditions, types.CanaryStatusCondition{
				Type:               string(cond.Type),
				Status:             string(cond.Status),
				LastUpdateTime:     cond.LastUpdateTime.Format("2006-01-02 15:04:05"),
				LastTransitionTime: cond.LastTransitionTime.Format("2006-01-02 15:04:05"),
				Reason:             cond.Reason,
				Message:            cond.Message,
			})
		}
		status.Conditions = conditions
	} else {
		status.Conditions = make([]types.CanaryStatusCondition, 0)
	}
	return status, nil
}
