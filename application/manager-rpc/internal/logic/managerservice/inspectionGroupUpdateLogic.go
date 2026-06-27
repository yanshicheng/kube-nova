package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionGroupUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionGroupUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionGroupUpdateLogic {
	return &InspectionGroupUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionGroupUpdateLogic) InspectionGroupUpdate(in *pb.InspectionGroupUpdateReq) (*pb.InspectionGroupResp, error) {
	if in.Id == 0 || in.TemplateId == 0 || in.GroupCode == "" || in.GroupName == "" {
		return nil, errorx.Msg("巡检分组ID、模板、编码和名称不能为空")
	}
	item, err := l.svcCtx.OnecInspectionGroupModel.FindOne(l.ctx, in.Id)
	if err != nil || item.IsDeleted == 1 {
		return nil, errorx.Msg("巡检分组不存在")
	}
	oldName := item.GroupName
	item.TemplateId = in.TemplateId
	item.GroupCode = in.GroupCode
	item.GroupName = in.GroupName
	item.Description = in.Description
	item.Enabled = inspectionBool(in.Enabled)
	item.OrderNum = in.OrderNum
	item.ConfigJson = inspectionNullString(in.ConfigJson)
	item.UpdatedBy = inspectionString(in.UpdatedBy, "system")
	if item.OrderNum <= 0 {
		item.OrderNum = 100
	}
	if err := l.svcCtx.OnecInspectionGroupModel.Update(l.ctx, item); err != nil {
		return nil, errorx.Msg("更新巡检分组失败")
	}
	if oldName != item.GroupName {
		_ = l.syncGroupItemCategory(item.Id, item.GroupName, item.UpdatedBy)
	}

	return &pb.InspectionGroupResp{Data: inspectionGroupToPB(item)}, nil
}

func (l *InspectionGroupUpdateLogic) syncGroupItemCategory(groupID uint64, category, operator string) error {
	items, err := l.svcCtx.OnecInspectionItemModel.SearchNoPage(l.ctx, "id", true, "`group_id` = ?", groupID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item == nil || item.IsDeleted == 1 {
			continue
		}
		item.Category = category
		item.UpdatedBy = operator
		if err := l.svcCtx.OnecInspectionItemModel.Update(l.ctx, item); err != nil {
			return err
		}
	}
	return nil
}
