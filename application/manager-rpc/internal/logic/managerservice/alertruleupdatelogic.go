package managerservicelogic

import (
	"context"
	"database/sql"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRuleUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleUpdateLogic {
	return &AlertRuleUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleUpdate 更新告警规则
func (l *AlertRuleUpdateLogic) AlertRuleUpdate(in *pb.UpdateAlertRuleReq) (*pb.UpdateAlertRuleResp, error) {
	// 检查规则是否存在
	existRule, err := l.svcCtx.AlertRulesModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("告警规则不存在")
		}
		return nil, errorx.Msg("查询告警规则失败")
	}

	// 如果修改了 alert_name，需要检查新的 alert_name 在同一分组下是否已存在
	if in.AlertName != "" && in.AlertName != existRule.AlertName {
		checkRule, err := l.svcCtx.AlertRulesModel.FindOneByGroupIdAlertName(l.ctx, existRule.GroupId, in.AlertName)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("查询告警名称失败")
		}
		if checkRule != nil && checkRule.Id != existRule.Id {
			return nil, errorx.Msg("该分组下告警名称已存在")
		}
	}

	// 更新数据
	existRule.AlertName = in.AlertName
	existRule.RuleNameCn = in.RuleNameCn
	existRule.Expr = in.Expr
	existRule.ForDuration = in.ForDuration
	existRule.Severity = in.Severity
	existRule.Summary = in.Summary
	existRule.Description = sql.NullString{String: in.Description, Valid: in.Description != ""}
	existRule.Labels = sql.NullString{String: in.Labels, Valid: in.Labels != ""}
	existRule.Annotations = sql.NullString{String: in.Annotations, Valid: in.Annotations != ""}
	existRule.IsEnabled = boolToInt(in.IsEnabled)
	existRule.SortOrder = in.SortOrder
	existRule.UpdatedBy = in.UpdatedBy

	err = l.svcCtx.AlertRulesModel.Update(l.ctx, existRule)
	if err != nil {
		return nil, errorx.Msg("更新告警规则失败")
	}

	return &pb.UpdateAlertRuleResp{}, nil
}
