package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchAlertInstanceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 搜索告警实例列表，支持分页和条件筛选
func NewSearchAlertInstanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchAlertInstanceLogic {
	return &SearchAlertInstanceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchAlertInstanceLogic) SearchAlertInstance(req *types.SearchAlertInstanceRequest) (resp *types.SearchAlertInstanceResponse, err error) {
	// 调用RPC服务搜索告警实例
	result, err := l.svcCtx.ManagerRpc.AlertInstancesSearch(l.ctx, &pb.SearchAlertInstancesReq{
		Page:          req.Page,
		PageSize:      req.PageSize,
		OrderField:    req.OrderField,
		IsAsc:         req.IsAsc,
		Instance:      req.Instance,
		Fingerprint:   req.Fingerprint,
		ClusterUuid:   req.ClusterUuid,
		ClusterName:   req.ClusterName,
		ProjectId:     req.ProjectId,
		ProjectName:   req.ProjectName,
		WorkspaceId:   req.WorkspaceId,
		WorkspaceName: req.WorkspaceName,
		AlertName:     req.AlertName,
		Severity:      req.Severity,
		Status:        req.Status,
		RepeatCount:   req.RepeatCount,
	})

	if err != nil {
		l.Errorf("搜索告警实例失败: %v", err)
		return nil, fmt.Errorf("搜索告警实例失败: %v", err)
	}

	// 转换为 API 类型
	var items []types.AlertInstance
	for _, item := range result.Data {
		items = append(items, types.AlertInstance{
			Id:                item.Id,
			Instance:          item.Instance,
			Fingerprint:       item.Fingerprint,
			ClusterUuid:       item.ClusterUuid,
			ClusterName:       item.ClusterName,
			ProjectId:         item.ProjectId,
			ProjectName:       item.ProjectName,
			WorkspaceId:       item.WorkspaceId,
			WorkspaceName:     item.WorkspaceName,
			AlertName:         item.AlertName,
			Severity:          item.Severity,
			Status:            item.Status,
			Labels:            item.Labels,
			Annotations:       item.Annotations,
			GeneratorUrl:      item.GeneratorUrl,
			StartsAt:          item.StartsAt,
			EndsAt:            item.EndsAt,
			ResolvedAt:        item.ResolvedAt,
			Duration:          item.Duration,
			RepeatCount:       item.RepeatCount,
			NotifiedGroups:    item.NotifiedGroups,
			NotificationCount: item.NotificationCount,
			LastNotifiedAt:    item.LastNotifiedAt,
			CreatedBy:         item.CreatedBy,
			UpdatedBy:         item.UpdatedBy,
			CreatedAt:         item.CreatedAt,
			UpdatedAt:         item.UpdatedAt,
		})
	}

	return &types.SearchAlertInstanceResponse{
		Items: items,
		Total: result.Total,
	}, nil
}
