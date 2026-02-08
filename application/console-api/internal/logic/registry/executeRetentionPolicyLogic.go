package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type ExecuteRetentionPolicyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 执行保留策略
func NewExecuteRetentionPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExecuteRetentionPolicyLogic {
	return &ExecuteRetentionPolicyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ExecuteRetentionPolicyLogic) ExecuteRetentionPolicy(req *types.ExecuteRetentionPolicyRequest) (resp *types.ExecuteRetentionPolicyResponse, err error) {
	if req.PolicyId <= 0 {
		l.Errorf("执行保留策略失败: PolicyId 无效 (policyId=%d)", req.PolicyId)
		return nil, errorx.Msg("策略ID无效，请先配置保留策略")
	}

	rpcResp, err := l.svcCtx.RepositoryRpc.ExecuteRetentionPolicy(l.ctx, &pb.ExecuteRetentionPolicyReq{
		RegistryUuid: req.RegistryUuid,
		PolicyId:     req.PolicyId,
		DryRun:       req.DryRun,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	message := "保留策略执行成功"
	if req.DryRun {
		message = "保留策略试运行成功"
	}

	l.Infof("保留策略执行成功: ExecutionId=%d, DryRun=%v", rpcResp.ExecutionId, req.DryRun)
	return &types.ExecuteRetentionPolicyResponse{
		ExecutionId: rpcResp.ExecutionId,
		Message:     message,
	}, nil
}
