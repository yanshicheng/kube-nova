package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionGroupAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionGroupAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionGroupAddLogic {
	return &InspectionGroupAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionGroupAddLogic) InspectionGroupAdd(in *pb.InspectionGroupAddReq) (*pb.InspectionGroupResp, error) {
	if in.TemplateId == 0 || in.GroupCode == "" || in.GroupName == "" {
		return nil, errorx.Msg("巡检分组模板、编码和名称不能为空")
	}
	template, err := l.svcCtx.OnecInspectionTemplateModel.FindOne(l.ctx, in.TemplateId)
	if err != nil || template.IsDeleted == 1 {
		return nil, errorx.Msg("巡检模板不存在")
	}
	item := &model.OnecInspectionGroup{
		TemplateId:  in.TemplateId,
		GroupCode:   in.GroupCode,
		GroupName:   in.GroupName,
		Description: in.Description,
		Enabled:     inspectionBool(in.Enabled),
		OrderNum:    in.OrderNum,
		ConfigJson:  inspectionNullString(in.ConfigJson),
		CreatedBy:   inspectionString(in.CreatedBy, "system"),
		UpdatedBy:   inspectionString(in.CreatedBy, "system"),
	}
	if item.OrderNum <= 0 {
		item.OrderNum = 100
	}
	result, err := l.svcCtx.OnecInspectionGroupModel.Insert(l.ctx, item)
	if err != nil {
		return nil, errorx.Msg("创建巡检分组失败")
	}
	id, _ := result.LastInsertId()
	item.Id = uint64(id)

	return &pb.InspectionGroupResp{Data: inspectionGroupToPB(item)}, nil
}
