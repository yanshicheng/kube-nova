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

type AlertRuleAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRuleAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRuleAddLogic {
	return &AlertRuleAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRuleAdd 新增告警规则
func (l *AlertRuleAddLogic) AlertRuleAdd(in *pb.AddAlertRuleReq) (*pb.AddAlertRuleResp, error) {
	// 检查所属分组是否存在
	_, err := l.svcCtx.AlertRuleGroupsModel.FindOne(l.ctx, in.GroupId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("所属告警规则分组不存在")
		}
		return nil, errorx.Msg("查询告警规则分组失败")
	}

	// 检查 group_id + alert_name 唯一性
	existRule, err := l.svcCtx.AlertRulesModel.FindOneByGroupIdAlertName(l.ctx, in.GroupId, in.AlertName)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, errorx.Msg("查询告警名称失败")
	}
	if existRule != nil {
		return nil, errorx.Msg("该分组下告警名称已存在")
	}

	// 设置默认值
	forDuration := in.ForDuration
	if forDuration == "" {
		forDuration = "5m"
	}

	severity := in.Severity
	if severity == "" {
		severity = "warning"
	}

	// 构建数据
	data := &model.AlertRules{
		GroupId:     in.GroupId,
		AlertName:   in.AlertName,
		RuleNameCn:  in.RuleNameCn,
		Expr:        in.Expr,
		ForDuration: forDuration,
		Severity:    severity,
		Summary:     in.Summary,
		Description: sql.NullString{String: in.Description, Valid: in.Description != ""},
		Labels:      sql.NullString{String: in.Labels, Valid: in.Labels != ""},
		Annotations: sql.NullString{String: in.Annotations, Valid: in.Annotations != ""},
		IsEnabled:   boolToInt(in.IsEnabled),
		SortOrder:   in.SortOrder,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.CreatedBy,
		IsDeleted:   0,
	}

	result, err := l.svcCtx.AlertRulesModel.Insert(l.ctx, data)
	if err != nil {
		return nil, errorx.Msg("新增告警规则失败")
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, errorx.Msg("获取新增ID失败")
	}

	return &pb.AddAlertRuleResp{
		Id: uint64(id),
	}, nil
}
