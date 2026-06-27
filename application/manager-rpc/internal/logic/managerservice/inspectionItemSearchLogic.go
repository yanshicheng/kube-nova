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

type InspectionItemSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionItemSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionItemSearchLogic {
	return &InspectionItemSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionItemSearchLogic) InspectionItemSearch(in *pb.InspectionItemSearchReq) (*pb.InspectionItemSearchResp, error) {
	conditions := make([]string, 0)
	args := make([]any, 0)
	if in.TemplateId > 0 {
		conditions = append(conditions, "`template_id` = ?")
		args = append(args, in.TemplateId)
	}
	if in.GroupId > 0 {
		conditions = append(conditions, "`group_id` = ?")
		args = append(args, in.GroupId)
	}
	if strings.TrimSpace(in.Category) != "" {
		conditions = append(conditions, "`category` = ?")
		args = append(args, strings.TrimSpace(in.Category))
	}
	if strings.TrimSpace(in.CheckType) != "" {
		conditions = append(conditions, "`check_type` = ?")
		args = append(args, strings.TrimSpace(in.CheckType))
	}
	if in.Enabled >= 0 {
		conditions = append(conditions, "`enabled` = ?")
		args = append(args, in.Enabled)
	}
	items, total, err := l.svcCtx.OnecInspectionItemModel.Search(l.ctx, inspectionString(in.OrderField, "order_num"), in.IsAsc, in.Page, in.PageSize, strings.Join(conditions, " AND "), args...)
	if err != nil && err != model.ErrNotFound {
		return nil, errorx.Msg("查询巡检项失败")
	}
	resp := &pb.InspectionItemSearchResp{Data: make([]*pb.InspectionItem, 0, len(items)), Total: total}
	for _, item := range items {
		resp.Data = append(resp.Data, inspectionItemToPB(item))
	}

	return resp, nil
}
