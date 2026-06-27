package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionTemplateUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionTemplateUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionTemplateUpdateLogic {
	return &InspectionTemplateUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionTemplateUpdateLogic) InspectionTemplateUpdate(in *pb.InspectionTemplateUpdateReq) (*pb.InspectionTemplateResp, error) {
	if in.Id == 0 {
		return nil, errorx.Msg("模板ID不能为空")
	}
	item, err := l.svcCtx.OnecInspectionTemplateModel.FindOne(l.ctx, in.Id)
	if err != nil || item.IsDeleted == 1 {
		return nil, errorx.Msg("巡检模板不存在")
	}
	if item.IsBuiltin == 1 {
		return nil, errorx.Msg("内置模板不允许修改")
	}
	item.Name = in.Name
	item.Code = in.Code
	item.Description = in.Description
	item.ScopeType = inspectionString(in.ScopeType, "cluster")
	item.Enabled = inspectionBool(in.Enabled)
	item.ConfigJson = inspectionNullString(in.ConfigJson)
	item.Version++
	item.UpdatedBy = inspectionString(in.UpdatedBy, "system")
	if err := l.svcCtx.OnecInspectionTemplateModel.Update(l.ctx, item); err != nil {
		return nil, errorx.Msg("更新巡检模板失败")
	}

	return &pb.InspectionTemplateResp{Data: inspectionTemplateToPB(item)}, nil
}
