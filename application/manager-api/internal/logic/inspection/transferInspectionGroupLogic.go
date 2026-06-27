// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package inspection

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type TransferInspectionGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 复制或迁移巡检分组
func NewTransferInspectionGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransferInspectionGroupLogic {
	return &TransferInspectionGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TransferInspectionGroupLogic) TransferInspectionGroup(req *types.InspectionGroupTransferRequest) (resp *types.InspectionGroupTransferResponse, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionGroupTransfer(l.ctx, &pb.InspectionGroupTransferReq{
		Id:               req.Id,
		TargetTemplateId: req.TargetTemplateId,
		GroupCode:        req.GroupCode,
		GroupName:        req.GroupName,
		ItemCodePrefix:   req.ItemCodePrefix,
		Operation:        req.Operation,
		Operator:         currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	group := convertGroup(rpcResp.Group)
	out := &types.InspectionGroupTransferResponse{ItemCount: rpcResp.ItemCount}
	if group != nil {
		out.Group = *group
	}
	return out, nil
}
