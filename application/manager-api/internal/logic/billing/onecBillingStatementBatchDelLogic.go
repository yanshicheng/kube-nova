package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingStatementBatchDelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingStatementBatchDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingStatementBatchDelLogic {
	return &OnecBillingStatementBatchDelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingStatementBatchDelLogic) OnecBillingStatementBatchDel(req *types.OnecBillingStatementBatchDelRequest) (resp *types.OnecBillingStatementBatchDelResponse, err error) {
	// 调用RPC服务批量删除账单
	result, err := l.svcCtx.ManagerRpc.OnecBillingStatementBatchDel(l.ctx, &pb.OnecBillingStatementBatchDelReq{
		BeforeTime:  req.BeforeTime,
		ClusterUuid: req.ClusterUuid,
		ProjectId:   req.ProjectId,
	})
	if err != nil {
		l.Errorf("批量删除账单失败: %v", err)
		return nil, fmt.Errorf("批量删除账单失败: %v", err)
	}

	return &types.OnecBillingStatementBatchDelResponse{
		DeletedCount: result.DeletedCount,
	}, nil
}
