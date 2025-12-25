package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListAlertRuleGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列表查询告警规则分组(无分页)
func NewListAlertRuleGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAlertRuleGroupLogic {
	return &ListAlertRuleGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListAlertRuleGroupLogic) ListAlertRuleGroup(req *types.ListAlertRuleGroupRequest) (resp []types.AlertRuleGroup, err error) {
	// 调用RPC服务列表查询告警规则分组
	result, err := l.svcCtx.ManagerRpc.AlertRuleGroupList(l.ctx, &pb.ListAlertRuleGroupReq{
		FileId: req.FileId,
	})

	if err != nil {
		l.Errorf("列表查询告警规则分组失败: %v", err)
		return nil, fmt.Errorf("列表查询告警规则分组失败: %v", err)
	}

	// 转换为API响应类型
	resp = make([]types.AlertRuleGroup, 0, len(result.Data))
	for _, item := range result.Data {
		resp = append(resp, types.AlertRuleGroup{
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

	return resp, nil
}
