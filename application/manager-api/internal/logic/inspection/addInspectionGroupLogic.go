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

type AddInspectionGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建巡检分组
func NewAddInspectionGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddInspectionGroupLogic {
	return &AddInspectionGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddInspectionGroupLogic) AddInspectionGroup(req *types.InspectionGroupAddRequest) (resp *types.InspectionGroup, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionGroupAdd(l.ctx, &pb.InspectionGroupAddReq{
		TemplateId:  req.TemplateId,
		GroupCode:   req.GroupCode,
		GroupName:   req.GroupName,
		Description: req.Description,
		Enabled:     req.Enabled,
		OrderNum:    req.OrderNum,
		ConfigJson:  req.ConfigJson,
		CreatedBy:   currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return convertGroup(rpcResp.Data), nil
}
