package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchAlertGroupsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchAlertGroupsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchAlertGroupsLogic {
	return &SearchAlertGroupsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchAlertGroupsLogic) SearchAlertGroups(req *types.SearchAlertGroupsRequest) (resp *types.SearchAlertGroupsResponse, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("搜索告警组请求: operator=%s, page=%d, pageSize=%d", username, req.Page, req.PageSize)

	// 调用 RPC 服务搜索告警组
	result, err := l.svcCtx.AlertPortalRpc.AlertGroupsSearch(l.ctx, &pb.SearchAlertGroupsReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		OrderField: req.OrderField,
		IsAsc:      req.IsAsc,
		Uuid:       req.Uuid,
		GroupName:  req.GroupName,
		GroupType:  req.GroupType,
	})
	if err != nil {
		l.Errorf("搜索告警组失败: operator=%s, error=%v", username, err)
		return nil, err
	}

	// 转换数据
	items := make([]types.AlertGroups, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, types.AlertGroups{
			Id:           item.Id,
			Uuid:         item.Uuid,
			GroupName:    item.GroupName,
			GroupType:    item.GroupType,
			Description:  item.Description,
			FilterRules:  item.FilterRules,
			DutySchedule: item.DutySchedule,
			SortOrder:    item.SortOrder,
			CreatedBy:    item.CreatedBy,
			UpdatedBy:    item.UpdatedBy,
			CreatedAt:    item.CreatedAt,
			UpdatedAt:    item.UpdatedAt,
		})
	}

	resp = &types.SearchAlertGroupsResponse{
		Items: items,
		Total: result.Total,
	}

	l.Infof("搜索告警组成功: operator=%s, total=%d", username, result.Total)
	return resp, nil
}
