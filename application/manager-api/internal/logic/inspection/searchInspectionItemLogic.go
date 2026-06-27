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

type SearchInspectionItemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询巡检项
func NewSearchInspectionItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchInspectionItemLogic {
	return &SearchInspectionItemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchInspectionItemLogic) SearchInspectionItem(req *types.InspectionItemSearchRequest) (resp *types.InspectionItemSearchResponse, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionItemSearch(l.ctx, &pb.InspectionItemSearchReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		OrderField: req.OrderField,
		IsAsc:      req.IsAsc,
		TemplateId: req.TemplateId,
		GroupId:    req.GroupId,
		Category:   req.Category,
		CheckType:  req.CheckType,
		Enabled:    enabledFilter(req.Enabled),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.InspectionItemSearchResponse{Data: make([]types.InspectionItem, 0, len(rpcResp.Data)), Total: rpcResp.Total}
	for _, item := range rpcResp.Data {
		if converted := convertItem(item); converted != nil {
			resp.Data = append(resp.Data, *converted)
		}
	}

	return resp, nil
}
