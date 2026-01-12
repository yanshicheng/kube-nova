package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/billing"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingStatementGenerateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingStatementGenerateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingStatementGenerateLogic {
	return &OnecBillingStatementGenerateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 立即生成账单
func (l *OnecBillingStatementGenerateLogic) OnecBillingStatementGenerate(in *pb.OnecBillingStatementGenerateReq) (*pb.OnecBillingStatementGenerateResp, error) {
	// 立即触发账单生成
	err := l.svcCtx.BillingService.GenerateByClusterAndProject(
		l.ctx, in.ClusterUuid,
		in.ProjectId,
		&billing.GenerateOption{
			CreatedBy:     in.UserName,
			StatementType: billing.StatementTypeConfigChange,
		})
	if err != nil {
		l.Errorf("生成账单失败, :%v", err)
		return nil, err
	}
	l.Infof("生成账单成功")
	return &pb.OnecBillingStatementGenerateResp{}, nil
}
