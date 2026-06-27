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

type UpdateInspectionGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新巡检分组
func NewUpdateInspectionGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateInspectionGroupLogic {
	return &UpdateInspectionGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateInspectionGroupLogic) UpdateInspectionGroup(req *types.InspectionGroupUpdateRequest) (resp *types.InspectionGroup, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionGroupUpdate(l.ctx, &pb.InspectionGroupUpdateReq{
		Id:          req.Id,
		TemplateId:  req.TemplateId,
		GroupCode:   req.GroupCode,
		GroupName:   req.GroupName,
		Description: req.Description,
		Enabled:     req.Enabled,
		OrderNum:    req.OrderNum,
		ConfigJson:  req.ConfigJson,
		UpdatedBy:   currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return convertGroup(rpcResp.Data), nil
}
