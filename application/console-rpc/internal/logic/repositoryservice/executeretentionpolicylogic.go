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
	l.Infof("执行保留策略: registryUuid=%s, policyId=%d, dryRun=%v",
		in.RegistryUuid, in.PolicyId, in.DryRun)

	if in.PolicyId <= 0 {
		l.Errorf("执行保留策略失败: PolicyId 无效 (policyId=%d)", in.PolicyId)
		return nil, errorx.Msg("策略ID无效，请先配置保留策略")
	}

	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		l.Errorf("获取仓库客户端失败: %v", err)
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	executionID, err := client.Retention().Execute(in.PolicyId, in.DryRun)
	if err != nil {
		l.Errorf("执行保留策略失败: %v", err)
		return nil, errorx.Msg("执行保留策略失败")
	}

	message := "保留策略执行成功"
	if in.DryRun {
		message = "保留策略试运行成功"
	}

	l.Infof("执行保留策略成功: executionId=%d, dryRun=%v", executionID, in.DryRun)
	return &pb.ExecuteRetentionPolicyResp{
		ExecutionId: executionID,
		Message:     message,
	}, nil
}
