package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingStatementDelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingStatementDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingStatementDelLogic {
	return &OnecBillingStatementDelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingStatementDelLogic) OnecBillingStatementDel(req *types.OnecBillingStatementDelRequest) (resp *types.OnecBillingStatementDelResponse, err error) {
	// 调用RPC服务删除账单
	_, err = l.svcCtx.ManagerRpc.OnecBillingStatementDel(l.ctx, &pb.OnecBillingStatementDelReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("删除账单失败: %v", err)
		return nil, fmt.Errorf("删除账单失败: %v", err)
	}

	return &types.OnecBillingStatementDelResponse{}, nil
}
