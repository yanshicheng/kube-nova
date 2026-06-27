package managerservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/inspection"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionReportGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionReportGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionReportGetLogic {
	return &InspectionReportGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionReportGetLogic) InspectionReportGet(in *pb.InspectionReportReq) (*pb.InspectionReportResp, error) {
	if in.RecordId == 0 {
		return nil, errorx.Msg("巡检记录ID不能为空")
	}
	record, err := l.svcCtx.OnecInspectionRecordModel.FindOne(l.ctx, in.RecordId)
	if err != nil || record.IsDeleted == 1 {
		return nil, errorx.Msg("巡检记录不存在")
	}
	var (
		items []*model.OnecInspectionResult
		total uint64
	)
	query, args := inspectionReportResultQuery(in)
	if in.PageSize > 0 {
		page := in.Page
		if page == 0 {
			page = 1
		}
		pageSize := in.PageSize
		if pageSize > 1000 {
			pageSize = 1000
		}
		items, total, err = l.svcCtx.OnecInspectionResultModel.Search(l.ctx, "id", true, page, pageSize, query, args...)
	} else {
		items, err = l.svcCtx.OnecInspectionResultModel.SearchNoPage(l.ctx, "id", true, query, args...)
		total = uint64(len(items))
	}
	if err != nil && err != model.ErrNotFound {
		return nil, errorx.Msg("查询巡检结果失败")
	}
	resp := &pb.InspectionReportResp{
		Record:       inspection.ToPBRecord(record),
		Results:      make([]*pb.InspectionResult, 0, len(items)),
		ResultsTotal: total,
	}
	for _, item := range items {
		resp.Results = append(resp.Results, inspection.ToPBResult(item))
	}

	return resp, nil
}

func inspectionReportResultQuery(in *pb.InspectionReportReq) (string, []any) {
	conditions := []string{"`record_id` = ?"}
	args := []any{in.RecordId}
	if status := strings.TrimSpace(in.Status); status != "" {
		conditions = append(conditions, "`status` = ?")
		args = append(args, status)
	}
	if severity := strings.TrimSpace(in.Severity); severity != "" {
		conditions = append(conditions, "`severity` = ?")
		args = append(args, severity)
	}
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		conditions = append(conditions, "(`item_name` LIKE ? OR `item_code` LIKE ? OR `category` LIKE ? OR `target_type` LIKE ? OR `target_name` LIKE ? OR `expected` LIKE ? OR `actual` LIKE ? OR `value` LIKE ? OR `unit` LIKE ? OR `message` LIKE ? OR `suggestion` LIKE ? OR `detail_json` LIKE ?)")
		for i := 0; i < 12; i++ {
			args = append(args, like)
		}
	}
	return strings.Join(conditions, " AND "), args
}
