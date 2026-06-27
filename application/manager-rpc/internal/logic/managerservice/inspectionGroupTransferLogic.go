package managerservicelogic

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type InspectionGroupTransferLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionGroupTransferLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionGroupTransferLogic {
	return &InspectionGroupTransferLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionGroupTransferLogic) InspectionGroupTransfer(in *pb.InspectionGroupTransferReq) (*pb.InspectionGroupTransferResp, error) {
	if in.Id == 0 || in.TargetTemplateId == 0 {
		return nil, errorx.Msg("源分组和目标规则库不能为空")
	}
	operation := strings.ToLower(strings.TrimSpace(in.Operation))
	if operation != "copy" && operation != "move" {
		return nil, errorx.Msg("操作类型仅支持 copy 或 move")
	}
	groupCode := strings.TrimSpace(in.GroupCode)
	groupName := strings.TrimSpace(in.GroupName)
	if groupCode == "" || groupName == "" {
		return nil, errorx.Msg("目标分组编码和名称不能为空")
	}
	operator := inspectionString(in.Operator, "system")

	source, err := l.svcCtx.OnecInspectionGroupModel.FindOne(l.ctx, in.Id)
	if err != nil || source.IsDeleted == 1 {
		return nil, errorx.Msg("源巡检分组不存在")
	}
	targetTemplate, err := l.svcCtx.OnecInspectionTemplateModel.FindOne(l.ctx, in.TargetTemplateId)
	if err != nil || targetTemplate.IsDeleted == 1 {
		return nil, errorx.Msg("目标规则库不存在")
	}
	sourceTemplate, err := l.svcCtx.OnecInspectionTemplateModel.FindOne(l.ctx, source.TemplateId)
	if err != nil || sourceTemplate.IsDeleted == 1 {
		return nil, errorx.Msg("源规则库不存在")
	}
	if operation == "move" && sourceTemplate.IsBuiltin == 1 {
		return nil, errorx.Msg("内置规则库分组不允许迁移")
	}

	items, err := l.svcCtx.OnecInspectionItemModel.SearchNoPage(l.ctx, "order_num", true, "`group_id` = ?", source.Id)
	if err != nil && err != model.ErrNotFound {
		return nil, errorx.Msg("查询源分组规则失败")
	}
	if err := l.validateGroupTransferConflicts(source, items, in.TargetTemplateId, groupCode, groupName, strings.TrimSpace(in.ItemCodePrefix), operation); err != nil {
		return nil, err
	}

	var groupID uint64
	if operation == "copy" {
		groupID, err = l.copyInspectionGroup(source, items, in.TargetTemplateId, groupCode, groupName, strings.TrimSpace(in.ItemCodePrefix), operator)
	} else {
		groupID, err = l.moveInspectionGroup(source, items, in.TargetTemplateId, groupCode, groupName, operator)
	}
	if err != nil {
		return nil, err
	}
	group, err := l.svcCtx.OnecInspectionGroupModel.FindOne(l.ctx, groupID)
	if err != nil {
		return nil, errorx.Msg("查询处理后的分组失败")
	}
	return &pb.InspectionGroupTransferResp{Group: inspectionGroupToPB(group), ItemCount: int64(len(items))}, nil
}

func (l *InspectionGroupTransferLogic) validateGroupTransferConflicts(source *model.OnecInspectionGroup, items []*model.OnecInspectionItem, targetTemplateID uint64, groupCode, groupName, itemCodePrefix, operation string) error {
	groups, err := l.svcCtx.OnecInspectionGroupModel.SearchNoPage(l.ctx, "id", true, "`template_id` = ? AND (`group_code` = ? OR `group_name` = ?)", targetTemplateID, groupCode, groupName)
	if err != nil && err != model.ErrNotFound {
		return errorx.Msg("检查目标分组冲突失败")
	}
	for _, group := range groups {
		if group == nil || group.IsDeleted == 1 {
			continue
		}
		if operation == "move" && group.Id == source.Id {
			continue
		}
		return errorx.Msg("目标规则库已存在相同编码或名称的分组")
	}

	sourceItemIDs := make(map[uint64]struct{}, len(items))
	for _, item := range items {
		if item == nil || item.IsDeleted == 1 {
			continue
		}
		sourceItemIDs[item.Id] = struct{}{}
		itemCode := item.ItemCode
		if operation == "copy" && itemCodePrefix != "" {
			itemCode = itemCodePrefix + itemCode
		}
		exists, err := l.svcCtx.OnecInspectionItemModel.SearchNoPage(l.ctx, "id", true, "`template_id` = ? AND `item_code` = ?", targetTemplateID, itemCode)
		if err != nil && err != model.ErrNotFound {
			return errorx.Msg("检查目标规则冲突失败")
		}
		for _, exist := range exists {
			if exist == nil || exist.IsDeleted == 1 {
				continue
			}
			if operation == "move" {
				if _, ok := sourceItemIDs[exist.Id]; ok {
					continue
				}
			}
			return errorx.Msg(fmt.Sprintf("目标规则库已存在规则编码 %s", itemCode))
		}
	}
	return nil
}

func (l *InspectionGroupTransferLogic) copyInspectionGroup(source *model.OnecInspectionGroup, items []*model.OnecInspectionItem, targetTemplateID uint64, groupCode, groupName, itemCodePrefix, operator string) (uint64, error) {
	var newGroupID uint64
	err := l.svcCtx.OnecInspectionGroupModel.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		result, err := session.ExecCtx(ctx, `INSERT INTO onec_inspection_group
			(template_id, group_code, group_name, description, enabled, order_num, config_json, created_by, updated_by, is_deleted)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
			targetTemplateID, groupCode, groupName, source.Description, source.Enabled, source.OrderNum, source.ConfigJson, operator, operator)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		newGroupID = uint64(id)
		for _, item := range items {
			if item == nil || item.IsDeleted == 1 {
				continue
			}
			itemCode := item.ItemCode
			if itemCodePrefix != "" {
				itemCode = itemCodePrefix + itemCode
			}
			if _, err := session.ExecCtx(ctx, `INSERT INTO onec_inspection_item
				(template_id, group_id, item_code, item_name, category, check_type, target_type, severity, weight, enabled, timeout_sec, order_num, promql, operator, threshold, unit, config_json, advice, created_by, updated_by, is_deleted)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
				targetTemplateID, newGroupID, itemCode, item.ItemName, groupName, item.CheckType, item.TargetType, item.Severity, item.Weight, item.Enabled, item.TimeoutSec, item.OrderNum, item.Promql, item.Operator, item.Threshold, item.Unit, item.ConfigJson, item.Advice, operator, operator); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, errorx.Msg("复制巡检分组失败")
	}
	return newGroupID, nil
}

func (l *InspectionGroupTransferLogic) moveInspectionGroup(source *model.OnecInspectionGroup, items []*model.OnecInspectionItem, targetTemplateID uint64, groupCode, groupName, operator string) (uint64, error) {
	err := l.svcCtx.OnecInspectionGroupModel.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := l.svcCtx.OnecInspectionGroupModel.TransOnSql(ctx, session, source.Id,
			"UPDATE onec_inspection_group SET template_id = ?, group_code = ?, group_name = ?, updated_by = ? WHERE id = ?",
			targetTemplateID, groupCode, groupName, operator, source.Id); err != nil {
			return err
		}
		for _, item := range items {
			if item == nil || item.IsDeleted == 1 {
				continue
			}
			if _, err := l.svcCtx.OnecInspectionItemModel.TransOnSql(ctx, session, item.Id,
				"UPDATE onec_inspection_item SET template_id = ?, category = ?, updated_by = ? WHERE id = ?",
				targetTemplateID, groupName, operator, item.Id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, errorx.Msg("迁移巡检分组失败")
	}
	return source.Id, nil
}
