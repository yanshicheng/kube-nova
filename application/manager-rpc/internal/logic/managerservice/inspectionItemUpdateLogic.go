package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionItemUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionItemUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionItemUpdateLogic {
	return &InspectionItemUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionItemUpdateLogic) InspectionItemUpdate(in *pb.InspectionItemUpdateReq) (*pb.InspectionItemResp, error) {
	if in.Id == 0 {
		return nil, errorx.Msg("巡检项ID不能为空")
	}
	item, err := l.svcCtx.OnecInspectionItemModel.FindOne(l.ctx, in.Id)
	if err != nil || item.IsDeleted == 1 {
		return nil, errorx.Msg("巡检项不存在")
	}
	item.TemplateId = in.TemplateId
	item.GroupId = in.GroupId
	item.ItemCode = in.ItemCode
	item.ItemName = in.ItemName
	item.Category = inspectionString(in.Category, "默认分类")
	item.CheckType = inspectionString(in.CheckType, "builtin")
	item.TargetType = inspectionString(in.TargetType, "cluster")
	item.Severity = inspectionString(in.Severity, "warning")
	item.Weight = in.Weight
	item.Enabled = inspectionBool(in.Enabled)
	item.TimeoutSec = in.TimeoutSec
	item.OrderNum = in.OrderNum
	item.Promql = inspectionNullString(in.Promql)
	item.Operator = in.Operator
	item.Threshold = in.Threshold
	item.Unit = in.Unit
	item.ConfigJson = inspectionNullString(in.ConfigJson)
	item.Advice = in.Advice
	item.UpdatedBy = inspectionString(in.UpdatedBy, "system")
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
	if err := l.svcCtx.OnecInspectionItemModel.Update(l.ctx, item); err != nil {
		return nil, errorx.Msg("更新巡检项失败")
	}

	return &pb.InspectionItemResp{Data: inspectionItemToPB(item)}, nil
}
