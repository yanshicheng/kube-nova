// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package inspection

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchInspectionTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询巡检模板
func NewSearchInspectionTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchInspectionTemplateLogic {
	return &SearchInspectionTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchInspectionTemplateLogic) SearchInspectionTemplate(req *types.InspectionTemplateSearchRequest) (resp *types.InspectionTemplateSearchResponse, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionTemplateSearch(l.ctx, &pb.InspectionTemplateSearchReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		OrderField: req.OrderField,
		IsAsc:      req.IsAsc,
		Name:       req.Name,
		ScopeType:  req.ScopeType,
		Enabled:    enabledFilter(req.Enabled),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.InspectionTemplateSearchResponse{Data: make([]types.InspectionTemplate, 0, len(rpcResp.Data)), Total: rpcResp.Total}
	for _, item := range rpcResp.Data {
		if converted := convertTemplate(item); converted != nil {
			resp.Data = append(resp.Data, *converted)
		}
	}

	return resp, nil
}
