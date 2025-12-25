package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchAlertGroupMembersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchAlertGroupMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchAlertGroupMembersLogic {
	return &SearchAlertGroupMembersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchAlertGroupMembersLogic) SearchAlertGroupMembers(req *types.SearchAlertGroupMembersRequest) (resp *types.SearchAlertGroupMembersResponse, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("搜索告警组成员请求: operator=%s, page=%d, pageSize=%d", username, req.Page, req.PageSize)

	// 调用 RPC 服务搜索告警组成员（RPC 已关联用户表）
	result, err := l.svcCtx.AlertPortalRpc.AlertGroupMembersSearch(l.ctx, &pb.SearchAlertGroupMembersReq{
		Page:     req.Page,
		PageSize: req.PageSize,
		OrderStr: req.OrderStr,
		IsAsc:    req.IsAsc,
		GroupId:  req.GroupId,
		UserId:   req.UserId,
		Role:     req.Role,
	})
	if err != nil {
		l.Errorf("搜索告警组成员失败: operator=%s, error=%v", username, err)
		return nil, err
	}

	// 转换数据，包含用户名和账号信息
	items := make([]types.AlertGroupMembers, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, types.AlertGroupMembers{
			Id:          item.Id,
			GroupId:     item.GroupId,
			UserId:      item.UserId,
			UserName:    item.UserName,    // RPC 返回的用户名
			UserAccount: item.UserAccount, // RPC 返回的用户账号
			Role:        item.Role,
			CreatedBy:   item.CreatedBy,
			UpdatedBy:   item.UpdatedBy,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	resp = &types.SearchAlertGroupMembersResponse{
		Items: items,
		Total: result.Total,
	}

	l.Infof("搜索告警组成员成功: operator=%s, total=%d", username, result.Total)
	return resp, nil
}
