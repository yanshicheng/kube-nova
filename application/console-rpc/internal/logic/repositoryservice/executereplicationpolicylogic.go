package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExecuteReplicationPolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExecuteReplicationPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExecuteReplicationPolicyLogic {
	return &ExecuteReplicationPolicyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ExecuteReplicationPolicyLogic) ExecuteReplicationPolicy(in *pb.ExecuteReplicationPolicyReq) (*pb.ExecuteReplicationPolicyResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	executionID, err := client.Replication().Execute(in.PolicyId)
	if err != nil {
		return nil, errorx.Msg("执行复制策略失败")
	}

	return &pb.ExecuteReplicationPolicyResp{
		ExecutionId: executionID,
		Message:     "复制策略执行成功",
	}, nil
}
