package managerservicelogic

import (
	"context"

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
	// todo: add your logic here and delete this line

	return &pb.OnecBillingStatementGenerateResp{}, nil
}
