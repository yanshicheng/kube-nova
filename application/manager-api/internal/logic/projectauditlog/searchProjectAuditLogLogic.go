package projectauditlog

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchProjectAuditLogLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchProjectAuditLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchProjectAuditLogLogic {
	return &SearchProjectAuditLogLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchProjectAuditLogLogic) SearchProjectAuditLog(req *types.SearchProjectAuditLogRequest) (resp *types.SearchProjectAuditLogResponse, err error) {
	// 设置默认值
	page := req.Page
	if page == 0 {
		page = 1
	}

	pageSize := req.PageSize
	if pageSize == 0 {
		pageSize = 20
	}

	orderField := req.OrderField
	if orderField == "" {
		orderField = "id"
	}

	// 调用 RPC 服务
	rpcResp, err := l.svcCtx.ManagerRpc.ProjectAuditLogSearch(l.ctx, &pb.SearchOnecProjectAuditLogReq{
		Page:          page,
		PageSize:      pageSize,
		OrderField:    orderField,
		IsAsc:         req.IsAsc,
		ClusterUuid:   req.ClusterUuid,
		WorkspaceId:   req.WorkspaceId,
		ApplicationId: req.ApplicationId,
		Title:         req.Title,
		OperatorName:  req.OperatorName,
		StartAt:       req.StartAt,
		EndAt:         req.EndAt,
		Status:        req.Status,
		ProjectId:     req.ProjectId,
	})

	if err != nil {
		l.Errorf("调用 RPC 服务失败: %v, page=%d, pageSize=%d", err, page, pageSize)
		return nil, err
	}

	// 转换响应数据
	items := make([]types.ProjectAuditLog, 0, len(rpcResp.Data))
	for _, item := range rpcResp.Data {
		items = append(items, types.ProjectAuditLog{
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
			CreatedAt:       item.CreatedAt,
			UpdatedAt:       item.UpdatedAt,
		})
	}

	l.Infof("搜索审计日志成功: total=%d, page=%d, pageSize=%d", rpcResp.Total, page, pageSize)

	return &types.SearchProjectAuditLogResponse{
		Items: items,
		Total: rpcResp.Total,
	}, nil
}
