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

type UpdateInspectionItemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新巡检项
func NewUpdateInspectionItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateInspectionItemLogic {
	return &UpdateInspectionItemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateInspectionItemLogic) UpdateInspectionItem(req *types.InspectionItemUpdateRequest) (resp *types.InspectionItem, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionItemUpdate(l.ctx, &pb.InspectionItemUpdateReq{
		Id:         req.Id,
		TemplateId: req.TemplateId,
		GroupId:    req.GroupId,
		ItemCode:   req.ItemCode,
		ItemName:   req.ItemName,
		Category:   req.Category,
		CheckType:  req.CheckType,
		TargetType: req.TargetType,
		Severity:   req.Severity,
		Weight:     req.Weight,
		Enabled:    req.Enabled,
		TimeoutSec: req.TimeoutSec,
		OrderNum:   req.OrderNum,
		Promql:     req.Promql,
		Operator:   req.Operator,
		Threshold:  req.Threshold,
		Unit:       req.Unit,
		ConfigJson: req.ConfigJson,
		Advice:     req.Advice,
		UpdatedBy:  currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return convertItem(rpcResp.Data), nil
}
