package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchAlertRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 搜索告警规则列表，支持分页和条件筛选
func NewSearchAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchAlertRuleLogic {
	return &SearchAlertRuleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchAlertRuleLogic) SearchAlertRule(req *types.SearchAlertRuleRequest) (resp *types.SearchAlertRuleResponse, err error) {
	// 调用RPC服务搜索告警规则
	result, err := l.svcCtx.ManagerRpc.AlertRuleSearch(l.ctx, &pb.SearchAlertRuleReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		OrderField: req.OrderField,
		IsAsc:      req.IsAsc,
		FileId:     req.FileId,
		GroupId:    req.GroupId,
		AlertName:  req.AlertName,
		RuleNameCn: req.RuleNameCn,
		Severity:   req.Severity,
	})

	if err != nil {
		l.Errorf("搜索告警规则失败: %v", err)
		return nil, fmt.Errorf("搜索告警规则失败: %v", err)
	}

	// 转换为API响应类型
	items := make([]types.AlertRule, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, types.AlertRule{
			Id:          item.Id,
			GroupId:     item.GroupId,
			AlertName:   item.AlertName,
			RuleNameCn:  item.RuleNameCn,
			Expr:        item.Expr,
			ForDuration: item.ForDuration,
			Severity:    item.Severity,
			Summary:     item.Summary,
			Description: item.Description,
			Labels:      item.Labels,
			Annotations: item.Annotations,
			IsEnabled:   item.IsEnabled,
			SortOrder:   item.SortOrder,
			GroupCode:   item.GroupCode,
			GroupName:   item.GroupName,
			FileId:      item.FileId,
			FileCode:    item.FileCode,
			CreatedBy:   item.CreatedBy,
			UpdatedBy:   item.UpdatedBy,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	resp = &types.SearchAlertRuleResponse{
		Items: items,
		Total: result.Total,
	}

	return resp, nil
}
