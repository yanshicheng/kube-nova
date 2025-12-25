package managerservicelogic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertInstancesSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertInstancesSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertInstancesSearchLogic {
	return &AlertInstancesSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertInstancesSearchLogic) AlertInstancesSearch(in *pb.SearchAlertInstancesReq) (*pb.SearchAlertInstancesResp, error) {
	// 构建查询条件
	var conditions []string
	var args []interface{}

	if in.Instance != "" {
		conditions = append(conditions, "`instance` LIKE ?")
		args = append(args, "%"+in.Instance+"%")
	}
	if in.Fingerprint != "" {
		conditions = append(conditions, "`fingerprint` = ?")
		args = append(args, in.Fingerprint)
	}
	if in.ClusterUuid != "" {
		conditions = append(conditions, "`cluster_uuid` = ?")
		args = append(args, in.ClusterUuid)
	}
	if in.ClusterName != "" {
		conditions = append(conditions, "`cluster_name` LIKE ?")
		args = append(args, "%"+in.ClusterName+"%")
	}
	if in.ProjectId > 0 {
		conditions = append(conditions, "`project_id` = ?")
		args = append(args, in.ProjectId)
	}
	if in.ProjectName != "" {
		conditions = append(conditions, "`project_name` LIKE ?")
		args = append(args, "%"+in.ProjectName+"%")
	}
	if in.WorkspaceId > 0 {
		conditions = append(conditions, "`workspace_id` = ?")
		args = append(args, in.WorkspaceId)
	}
	if in.WorkspaceName != "" {
		conditions = append(conditions, "`workspace_name` LIKE ?")
		args = append(args, "%"+in.WorkspaceName+"%")
	}
	if in.AlertName != "" {
		conditions = append(conditions, "`alert_name` LIKE ?")
		args = append(args, "%"+in.AlertName+"%")
	}
	if in.Severity != "" {
		conditions = append(conditions, "`severity` = ?")
		args = append(args, in.Severity)
	}
	if in.Status != "" {
		conditions = append(conditions, "`status` = ?")
		args = append(args, in.Status)
	}
	if in.RepeatCount > 0 {
		conditions = append(conditions, "`repeat_count` >= ?")
		args = append(args, in.RepeatCount)
	}

	queryStr := strings.Join(conditions, " AND ")

	// 设置默认分页参数
	page := in.Page
	if page < 1 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	// 设置排序字段
	orderField := in.OrderField
	if orderField == "" {
		orderField = "id"
	}

	// 执行搜索
	alertInstances, total, err := l.svcCtx.AlertInstancesModel.Search(
		l.ctx,
		orderField,
		in.IsAsc,
		page,
		pageSize,
		queryStr,
		args...,
	)

	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return &pb.SearchAlertInstancesResp{
				Data:  []*pb.AlertInstances{},
				Total: 0,
			}, nil
		}
		return nil, errorx.Msg(fmt.Sprintf("搜索告警实例失败: %v", err))
	}

	// 转换为 pb 格式
	var pbAlertInstances []*pb.AlertInstances
	for _, item := range alertInstances {
		pbAlertInstances = append(pbAlertInstances, &pb.AlertInstances{
			Id:                uint64(item.Id),
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
			StartsAt:          item.StartsAt.Unix(),
			EndsAt:            getTimeValue(item.EndsAt),
			ResolvedAt:        getTimeValue(item.ResolvedAt),
			Duration:          int64(item.Duration),
			RepeatCount:       int64(item.RepeatCount),
			NotifiedGroups:    item.NotifiedGroups,
			NotificationCount: int64(item.NotificationCount),
			LastNotifiedAt:    getTimeValue(item.LastNotifiedAt),
			CreatedBy:         item.CreatedBy,
			UpdatedBy:         item.UpdatedBy,
			CreatedAt:         item.CreatedAt.Unix(),
			UpdatedAt:         item.UpdatedAt.Unix(),
		})
	}

	return &pb.SearchAlertInstancesResp{
		Data:  pbAlertInstances,
		Total: total,
	}, nil
}
