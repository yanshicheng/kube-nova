package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchAlertRuleGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 搜索告警规则分组列表
func NewSearchAlertRuleGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchAlertRuleGroupLogic {
	return &SearchAlertRuleGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchAlertRuleGroupLogic) SearchAlertRuleGroup(req *types.SearchAlertRuleGroupRequest) (resp *types.SearchAlertRuleGroupResponse, err error) {
	// 调用RPC服务搜索告警规则分组
	result, err := l.svcCtx.ManagerRpc.AlertRuleGroupSearch(l.ctx, &pb.SearchAlertRuleGroupReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		OrderField: req.OrderField,
		IsAsc:      req.IsAsc,
		FileId:     req.FileId,
		GroupCode:  req.GroupCode,
		GroupName:  req.GroupName,
	})

	if err != nil {
		l.Errorf("搜索告警规则分组失败: %v", err)
		return nil, fmt.Errorf("搜索告警规则分组失败: %v", err)
	}

	// 转换为API响应类型
	items := make([]types.AlertRuleGroup, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, types.AlertRuleGroup{
			Id:          item.Id,
			FileId:      item.FileId,
			GroupCode:   item.GroupCode,
			GroupName:   item.GroupName,
			Description: item.Description,
			Interval:    item.Interval,
			IsEnabled:   item.IsEnabled,
			SortOrder:   item.SortOrder,
			RuleCount:   item.RuleCount,
			FileCode:    item.FileCode,
			FileName:    item.FileName,
			CreatedBy:   item.CreatedBy,
			UpdatedBy:   item.UpdatedBy,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	resp = &types.SearchAlertRuleGroupResponse{
		Items: items,
		Total: result.Total,
	}

	return resp, nil
}
