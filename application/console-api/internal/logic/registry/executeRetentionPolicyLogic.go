package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
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

func (l *ExecuteRetentionPolicyLogic) ExecuteRetentionPolicy(req *types.ExecuteRetentionPolicyRequest) (resp string, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.ExecuteRetentionPolicy(l.ctx, &pb.ExecuteRetentionPolicyReq{
		RegistryUuid: req.RegistryUuid,
		PolicyId:     req.PolicyId,
		DryRun:       req.DryRun,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("保留策略执行成功: ExecutionId=%d", rpcResp.ExecutionId)
	return "保留策略执行成功", nil
}
