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

type AddInspectionTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建巡检模板
func NewAddInspectionTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddInspectionTemplateLogic {
	return &AddInspectionTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddInspectionTemplateLogic) AddInspectionTemplate(req *types.InspectionTemplateAddRequest) (resp *types.InspectionTemplate, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionTemplateAdd(l.ctx, &pb.InspectionTemplateAddReq{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		ScopeType:   req.ScopeType,
		Enabled:     req.Enabled,
		ConfigJson:  req.ConfigJson,
		CreatedBy:   currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return convertTemplate(rpcResp.Data), nil
}
