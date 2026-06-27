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

type InspectionTemplateSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionTemplateSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionTemplateSearchLogic {
	return &InspectionTemplateSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionTemplateSearchLogic) InspectionTemplateSearch(in *pb.InspectionTemplateSearchReq) (*pb.InspectionTemplateSearchResp, error) {
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
	if in.Enabled >= 0 {
		conditions = append(conditions, "`enabled` = ?")
		args = append(args, in.Enabled)
	}
	items, total, err := l.svcCtx.OnecInspectionTemplateModel.Search(l.ctx, in.OrderField, in.IsAsc, in.Page, in.PageSize, strings.Join(conditions, " AND "), args...)
	if err != nil && err != model.ErrNotFound {
		return nil, errorx.Msg("查询巡检模板失败")
	}
	resp := &pb.InspectionTemplateSearchResp{Data: make([]*pb.InspectionTemplate, 0, len(items)), Total: total}
	for _, item := range items {
		resp.Data = append(resp.Data, inspectionTemplateToPB(item))
	}

	return resp, nil
}
