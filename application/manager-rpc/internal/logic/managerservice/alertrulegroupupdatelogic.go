package managerservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRuleGroupUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleGroupUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleGroupUpdateLogic {
	return &AlertRuleGroupUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleGroupUpdate 更新告警规则分组
func (l *AlertRuleGroupUpdateLogic) AlertRuleGroupUpdate(in *pb.UpdateAlertRuleGroupReq) (*pb.UpdateAlertRuleGroupResp, error) {
	// 检查分组是否存在
	existGroup, err := l.svcCtx.AlertRuleGroupsModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("告警规则分组不存在")
		}
		return nil, errorx.Msg("查询告警规则分组失败")
	}

	// 如果修改了 group_code，需要检查新的 group_code 在同一文件下是否已存在
	if in.GroupCode != "" && in.GroupCode != existGroup.GroupCode {
		checkGroup, err := l.svcCtx.AlertRuleGroupsModel.FindOneByFileIdGroupCode(l.ctx, existGroup.FileId, in.GroupCode)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("查询分组代码失败")
		}
		if checkGroup != nil && checkGroup.Id != existGroup.Id {
			return nil, errorx.Msg("该文件下分组代码已存在")
		}
	}

	// 更新数据
	existGroup.GroupCode = in.GroupCode
	existGroup.GroupName = in.GroupName
	existGroup.Description = in.Description
	existGroup.Interval = in.Interval
	existGroup.IsEnabled = boolToInt(in.IsEnabled)
	existGroup.SortOrder = in.SortOrder
	existGroup.UpdatedBy = in.UpdatedBy

	err = l.svcCtx.AlertRuleGroupsModel.Update(l.ctx, existGroup)
	if err != nil {
		return nil, errorx.Msg("更新告警规则分组失败")
	}

	return &pb.UpdateAlertRuleGroupResp{}, nil
}
