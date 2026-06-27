package managerservicelogic

import (
	"context"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/inspection"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionRecordSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionRecordSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionRecordSearchLogic {
	return &InspectionRecordSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionRecordSearchLogic) InspectionRecordSearch(in *pb.InspectionRecordSearchReq) (*pb.InspectionRecordSearchResp, error) {
	conditions := make([]string, 0)
	args := make([]any, 0)
	if in.TaskId > 0 {
		conditions = append(conditions, "`task_id` = ?")
		args = append(args, in.TaskId)
	}
	if strings.TrimSpace(in.ClusterUuid) != "" {
		conditions = append(conditions, "`cluster_uuid` = ?")
		args = append(args, strings.TrimSpace(in.ClusterUuid))
	}
	if strings.TrimSpace(in.Status) != "" {
		conditions = append(conditions, "`status` = ?")
		args = append(args, strings.TrimSpace(in.Status))
	}
	if strings.TrimSpace(in.HealthLevel) != "" {
		conditions = append(conditions, "`health_level` = ?")
		args = append(args, strings.TrimSpace(in.HealthLevel))
	}
	if in.StartTime > 0 {
		conditions = append(conditions, "`created_at` >= ?")
		args = append(args, time.Unix(in.StartTime, 0))
	}
	if in.EndTime > 0 {
		conditions = append(conditions, "`created_at` <= ?")
		args = append(args, time.Unix(in.EndTime, 0))
	}
	items, total, err := l.svcCtx.OnecInspectionRecordModel.Search(l.ctx, inspectionString(in.OrderField, "id"), in.IsAsc, in.Page, in.PageSize, strings.Join(conditions, " AND "), args...)
	if err != nil && err != model.ErrNotFound {
		return nil, errorx.Msg("查询巡检记录失败")
	}
	resp := &pb.InspectionRecordSearchResp{Data: make([]*pb.InspectionRecord, 0, len(items)), Total: total}
	for _, item := range items {
		resp.Data = append(resp.Data, inspection.ToPBRecord(item))
	}

	return resp, nil
}
