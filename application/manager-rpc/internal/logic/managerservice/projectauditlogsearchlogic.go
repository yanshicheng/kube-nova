package managerservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectAuditLogSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectAuditLogSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectAuditLogSearchLogic {
	return &ProjectAuditLogSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectAuditLogSearchLogic) ProjectAuditLogSearch(in *pb.SearchOnecProjectAuditLogReq) (*pb.SearchOnecProjectAuditLogResp, error) {
	// 参数校验
	if in.Page == 0 {
		in.Page = 1
	}
	if in.PageSize == 0 {
		in.PageSize = 20
	}
	if in.PageSize > 100 {
		in.PageSize = 100
	}

	// 构建查询条件
	var conditions []string
	var args []interface{}
	if in.ProjectId != -1 {
		conditions = append(conditions, "`project_id` = ?")
		args = append(args, uint64(in.ProjectId))
	}
	// clusterUuid
	if in.ClusterUuid != "" {
		conditions = append(conditions, "`cluster_uuid` = ?")
		args = append(args, in.ClusterUuid)
	}

	if in.WorkspaceId != -1 {
		conditions = append(conditions, "`workspace_id` = ?")
		args = append(args, uint64(in.WorkspaceId))
	}
	// 添加状态搜索
	if in.Status != -1 {
		conditions = append(conditions, "`status` = ?")
		args = append(args, in.Status)
	}

	if in.ApplicationId != -1 && in.ApplicationId != 0 {
		conditions = append(conditions, "`application_id` = ?")
		args = append(args, uint64(in.ApplicationId))
	}

	// title 模糊搜索
	if in.Title != "" {
		conditions = append(conditions, "`title` LIKE ?")
		args = append(args, "%"+in.Title+"%")
	}

	// operatorName 模糊搜索
	if in.OperatorName != "" {
		conditions = append(conditions, "`operator_name` LIKE ?")
		args = append(args, "%"+in.OperatorName+"%")
	}

	// 时间范围筛选
	if in.StartAt > 0 {
		conditions = append(conditions, "UNIX_TIMESTAMP(`created_at`) >= ?")
		args = append(args, in.StartAt)
	}

	if in.EndAt > 0 {
		conditions = append(conditions, "UNIX_TIMESTAMP(`created_at`) <= ?")
		args = append(args, in.EndAt)
	}

	// 拼接查询条件
	queryStr := ""
	if len(conditions) > 0 {
		queryStr = strings.Join(conditions, " AND ")
	}

	// 排序字段处理
	orderStr := in.OrderField
	if orderStr == "" {
		orderStr = "id" // 默认按 id 排序
	}

	l.Debugf("查询条件: conditions=%s, args=%v", queryStr, args)

	// 查询数据
	results, total, err := l.svcCtx.OnecProjectAuditLog.Search(
		l.ctx,
		orderStr,
		in.IsAsc,
		in.Page,
		in.PageSize,
		queryStr,
		args...,
	)

	if err != nil {
		// 如果是未找到数据的错误，返回空结果而不是错误
		if errors.Is(err, model.ErrNotFound) {
			l.Infof("未找到匹配的审计日志: queryStr=%s", queryStr)
			return &pb.SearchOnecProjectAuditLogResp{
				Data:  []*pb.OnecProjectAuditLog{},
				Total: 0,
			}, nil
		}
		l.Errorf("搜索审计日志失败: %v, queryStr=%s, args=%v", err, queryStr, args)
		return nil, errorx.Msg("搜索审计日志失败")
	}

	// 转换为 protobuf 格式
	var pbResults []*pb.OnecProjectAuditLog
	for _, item := range results {
		pbResults = append(pbResults, &pb.OnecProjectAuditLog{
			Id:              item.Id,
			ClusterName:     item.ClusterName,
			ClusterUuid:     item.ClusterUuid,
			ProjectId:       item.ProjectId,
			ProjectName:     item.ProjectName,
			WorkspaceId:     item.WorkspaceId,
			WorkspaceName:   item.WorkspaceName,
			ApplicationId:   item.ApplicationId,
			ApplicationName: item.ApplicationName,
			Title:           item.Title,
			ActionDetail:    item.ActionDetail,
			Status:          item.Status,
			OperatorId:      item.OperatorId,
			OperatorName:    item.OperatorName,
			CreatedAt:       item.CreatedAt.Unix(),
			UpdatedAt:       item.UpdatedAt.Unix(),
		})
	}

	l.Infof("搜索审计日志成功: total=%d, page=%d, pageSize=%d, returned=%d",
		total, in.Page, in.PageSize, len(pbResults))

	return &pb.SearchOnecProjectAuditLogResp{
		Data:  pbResults,
		Total: total,
	}, nil
}
