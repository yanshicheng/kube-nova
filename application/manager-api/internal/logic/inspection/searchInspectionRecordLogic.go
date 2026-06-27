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

type SearchInspectionRecordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询巡检记录
func NewSearchInspectionRecordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchInspectionRecordLogic {
	return &SearchInspectionRecordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchInspectionRecordLogic) SearchInspectionRecord(req *types.InspectionRecordSearchRequest) (resp *types.InspectionRecordSearchResponse, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionRecordSearch(l.ctx, &pb.InspectionRecordSearchReq{
		Page:        req.Page,
		PageSize:    req.PageSize,
		OrderField:  req.OrderField,
		IsAsc:       req.IsAsc,
		TaskId:      req.TaskId,
		ClusterUuid: req.ClusterUuid,
		Status:      req.Status,
		HealthLevel: req.HealthLevel,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
	})
	if err != nil {
		return nil, err
	}
	resp = &types.InspectionRecordSearchResponse{Data: make([]types.InspectionRecord, 0, len(rpcResp.Data)), Total: rpcResp.Total}
	for _, item := range rpcResp.Data {
		if converted := convertRecord(item); converted != nil {
			resp.Data = append(resp.Data, *converted)
		}
	}

	return resp, nil
}
