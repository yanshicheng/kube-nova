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

type AddInspectionItemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建巡检项
func NewAddInspectionItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddInspectionItemLogic {
	return &AddInspectionItemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddInspectionItemLogic) AddInspectionItem(req *types.InspectionItemAddRequest) (resp *types.InspectionItem, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionItemAdd(l.ctx, &pb.InspectionItemAddReq{
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
		CreatedBy:  currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return convertItem(rpcResp.Data), nil
}
