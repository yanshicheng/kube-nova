package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ExecuteReplicationPolicyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 执行复制策略
func NewExecuteReplicationPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExecuteReplicationPolicyLogic {
	return &ExecuteReplicationPolicyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ExecuteReplicationPolicyLogic) ExecuteReplicationPolicy(req *types.ExecuteReplicationPolicyRequest) (resp string, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.ExecuteReplicationPolicy(l.ctx, &pb.ExecuteReplicationPolicyReq{
		RegistryUuid: req.RegistryUuid,
		PolicyId:     req.PolicyId,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("复制策略执行成功: ExecutionId=%d", rpcResp.ExecutionId)
	return "复制策略执行成功", nil
}
