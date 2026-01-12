package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/billing"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingStatementGenerateAllLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingStatementGenerateAllLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingStatementGenerateAllLogic {
	return &OnecBillingStatementGenerateAllLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 生成所有账单
func (l *OnecBillingStatementGenerateAllLogic) OnecBillingStatementGenerateAll(in *pb.OnecBillingStatementGenerateAllReq) (*pb.OnecBillingStatementGenerateAllResp, error) {
	// 生成所有项目订单，定时任务触发
	_, err := l.svcCtx.BillingService.GenerateAll(l.ctx, &billing.GenerateOption{
		StatementType: billing.StatementTypeDaily,
		CreatedBy:     in.UserName,
	})
	if err != nil {
		l.Infof("同步全部账单失败, :%v", err)
		return nil, err
	}
	l.Infof("同步全部账单成功")
	return &pb.OnecBillingStatementGenerateAllResp{}, nil
}
