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

type InspectionGroupSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionGroupSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionGroupSearchLogic {
	return &InspectionGroupSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionGroupSearchLogic) InspectionGroupSearch(in *pb.InspectionGroupSearchReq) (*pb.InspectionGroupSearchResp, error) {
	conditions := make([]string, 0)
	args := make([]any, 0)
	if in.TemplateId > 0 {
		conditions = append(conditions, "`template_id` = ?")
		args = append(args, in.TemplateId)
	}
	if strings.TrimSpace(in.GroupName) != "" {
		conditions = append(conditions, "`group_name` LIKE ?")
		args = append(args, "%"+strings.TrimSpace(in.GroupName)+"%")
	}
	if in.Enabled >= 0 {
		conditions = append(conditions, "`enabled` = ?")
		args = append(args, in.Enabled)
	}
	items, total, err := l.svcCtx.OnecInspectionGroupModel.Search(l.ctx, inspectionString(in.OrderField, "order_num"), in.IsAsc, in.Page, in.PageSize, strings.Join(conditions, " AND "), args...)
	if err != nil && err != model.ErrNotFound {
		return nil, errorx.Msg("查询巡检分组失败")
	}
	resp := &pb.InspectionGroupSearchResp{Data: make([]*pb.InspectionGroup, 0, len(items)), Total: total}
	for _, item := range items {
		resp.Data = append(resp.Data, inspectionGroupToPB(item))
	}

	return resp, nil
}
