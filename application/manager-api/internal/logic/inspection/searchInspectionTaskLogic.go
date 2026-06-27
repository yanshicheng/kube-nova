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

type SearchInspectionTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询巡检任务
func NewSearchInspectionTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchInspectionTaskLogic {
	return &SearchInspectionTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchInspectionTaskLogic) SearchInspectionTask(req *types.InspectionTaskSearchRequest) (resp *types.InspectionTaskSearchResponse, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionTaskSearch(l.ctx, &pb.InspectionTaskSearchReq{
		Page:         req.Page,
		PageSize:     req.PageSize,
		OrderField:   req.OrderField,
		IsAsc:        req.IsAsc,
		Name:         req.Name,
		ScopeType:    req.ScopeType,
		ClusterUuid:  req.ClusterUuid,
		ScheduleType: req.ScheduleType,
		Enabled:      enabledFilter(req.Enabled),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.InspectionTaskSearchResponse{Data: make([]types.InspectionTask, 0, len(rpcResp.Data)), Total: rpcResp.Total}
	for _, item := range rpcResp.Data {
		if converted := convertTask(item); converted != nil {
			resp.Data = append(resp.Data, *converted)
		}
	}

	return resp, nil
}
