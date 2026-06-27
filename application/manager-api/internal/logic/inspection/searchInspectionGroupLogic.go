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

type SearchInspectionGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询巡检分组
func NewSearchInspectionGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchInspectionGroupLogic {
	return &SearchInspectionGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchInspectionGroupLogic) SearchInspectionGroup(req *types.InspectionGroupSearchRequest) (resp *types.InspectionGroupSearchResponse, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionGroupSearch(l.ctx, &pb.InspectionGroupSearchReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		OrderField: req.OrderField,
		IsAsc:      req.IsAsc,
		TemplateId: req.TemplateId,
		GroupName:  req.GroupName,
		Enabled:    enabledFilter(req.Enabled),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.InspectionGroupSearchResponse{Data: make([]types.InspectionGroup, 0, len(rpcResp.Data)), Total: rpcResp.Total}
	for _, item := range rpcResp.Data {
		if converted := convertGroup(item); converted != nil {
			resp.Data = append(resp.Data, *converted)
		}
	}

	return resp, nil
}
