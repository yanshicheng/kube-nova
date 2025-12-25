package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExecuteRetentionPolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExecuteRetentionPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExecuteRetentionPolicyLogic {
	return &ExecuteRetentionPolicyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ExecuteRetentionPolicyLogic) ExecuteRetentionPolicy(in *pb.ExecuteRetentionPolicyReq) (*pb.ExecuteRetentionPolicyResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	executionID, err := client.Retention().Execute(in.PolicyId, in.DryRun)
	if err != nil {
		return nil, errorx.Msg("执行保留策略失败")
	}

	message := "保留策略执行成功"
	if in.DryRun {
		message = "保留策略试运行成功"
	}

	return &pb.ExecuteRetentionPolicyResp{
		ExecutionId: executionID,
		Message:     message,
	}, nil
}
