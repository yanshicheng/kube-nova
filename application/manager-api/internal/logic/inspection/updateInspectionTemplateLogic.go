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

type UpdateInspectionTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新巡检模板
func NewUpdateInspectionTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateInspectionTemplateLogic {
	return &UpdateInspectionTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateInspectionTemplateLogic) UpdateInspectionTemplate(req *types.InspectionTemplateUpdateRequest) (resp *types.InspectionTemplate, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionTemplateUpdate(l.ctx, &pb.InspectionTemplateUpdateReq{
		Id:          req.Id,
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		ScopeType:   req.ScopeType,
		Enabled:     req.Enabled,
		ConfigJson:  req.ConfigJson,
		UpdatedBy:   currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return convertTemplate(rpcResp.Data), nil
}
