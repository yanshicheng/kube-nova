package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionItemAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionItemAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionItemAddLogic {
	return &InspectionItemAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionItemAddLogic) InspectionItemAdd(in *pb.InspectionItemAddReq) (*pb.InspectionItemResp, error) {
	if in.ItemCode == "" || in.ItemName == "" {
		return nil, errorx.Msg("巡检项编码和名称不能为空")
	}
	item := &model.OnecInspectionItem{
		TemplateId: in.TemplateId,
		GroupId:    in.GroupId,
		ItemCode:   in.ItemCode,
		ItemName:   in.ItemName,
		Category:   inspectionString(in.Category, "默认分类"),
		CheckType:  inspectionString(in.CheckType, "builtin"),
		TargetType: inspectionString(in.TargetType, "cluster"),
		Severity:   inspectionString(in.Severity, "warning"),
		Weight:     in.Weight,
		Enabled:    inspectionBool(in.Enabled),
		TimeoutSec: in.TimeoutSec,
		OrderNum:   in.OrderNum,
		Promql:     inspectionNullString(in.Promql),
		Operator:   in.Operator,
		Threshold:  in.Threshold,
		Unit:       in.Unit,
		ConfigJson: inspectionNullString(in.ConfigJson),
		Advice:     in.Advice,
		CreatedBy:  inspectionString(in.CreatedBy, "system"),
		UpdatedBy:  inspectionString(in.CreatedBy, "system"),
	}
	if item.GroupId > 0 {
		group, err := l.svcCtx.OnecInspectionGroupModel.FindOne(l.ctx, item.GroupId)
		if err != nil || group.IsDeleted == 1 {
			return nil, errorx.Msg("巡检分组不存在")
		}
		if item.TemplateId == 0 {
			item.TemplateId = group.TemplateId
		}
		if item.TemplateId != group.TemplateId {
			return nil, errorx.Msg("巡检项和巡检分组不属于同一模板")
		}
		item.Category = group.GroupName
	}
	if item.Weight <= 0 {
		item.Weight = 10
	}
	if item.TimeoutSec <= 0 {
		item.TimeoutSec = 30
	}
	if item.OrderNum <= 0 {
		item.OrderNum = 100
	}
	result, err := l.svcCtx.OnecInspectionItemModel.Insert(l.ctx, item)
	if err != nil {
		return nil, errorx.Msg("创建巡检项失败")
	}
	id, _ := result.LastInsertId()
	item.Id = uint64(id)

	return &pb.InspectionItemResp{Data: inspectionItemToPB(item)}, nil
}
