package managerservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionTaskSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionTaskSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionTaskSearchLogic {
	return &InspectionTaskSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionTaskSearchLogic) InspectionTaskSearch(in *pb.InspectionTaskSearchReq) (*pb.InspectionTaskSearchResp, error) {
	conditions := make([]string, 0)
	args := make([]any, 0)
	if strings.TrimSpace(in.Name) != "" {
		conditions = append(conditions, "`name` LIKE ?")
		args = append(args, "%"+strings.TrimSpace(in.Name)+"%")
	}
	if strings.TrimSpace(in.ScopeType) != "" {
		conditions = append(conditions, "`scope_type` = ?")
		args = append(args, strings.TrimSpace(in.ScopeType))
	}
	if strings.TrimSpace(in.ClusterUuid) != "" {
		conditions = append(conditions, "`cluster_uuid` = ?")
		args = append(args, strings.TrimSpace(in.ClusterUuid))
	}
	if strings.TrimSpace(in.ScheduleType) != "" {
		conditions = append(conditions, "`schedule_type` = ?")
		args = append(args, strings.TrimSpace(in.ScheduleType))
	}
	if in.Enabled >= 0 {
		conditions = append(conditions, "`enabled` = ?")
		args = append(args, in.Enabled)
	}
	items, total, err := l.svcCtx.OnecInspectionTaskModel.Search(l.ctx, inspectionString(in.OrderField, "id"), in.IsAsc, in.Page, in.PageSize, strings.Join(conditions, " AND "), args...)
	if err != nil && err != model.ErrNotFound {
		return nil, errorx.Msg("查询巡检任务失败")
	}
	resp := &pb.InspectionTaskSearchResp{Data: make([]*pb.InspectionTask, 0, len(items)), Total: total}
	for _, item := range items {
		resp.Data = append(resp.Data, inspectionTaskToPB(item))
	}

	return resp, nil
}
