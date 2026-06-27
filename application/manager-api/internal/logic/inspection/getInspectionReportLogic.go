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

type GetInspectionReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询巡检报告
func NewGetInspectionReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetInspectionReportLogic {
	return &GetInspectionReportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetInspectionReportLogic) GetInspectionReport(req *types.InspectionReportRequest) (resp *types.InspectionReportResponse, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionReportGet(l.ctx, &pb.InspectionReportReq{
		RecordId: req.RecordId,
		Page:     req.Page,
		PageSize: req.PageSize,
		Keyword:  req.Keyword,
		Status:   req.Status,
		Severity: req.Severity,
	})
	if err != nil {
		return nil, err
	}
	resp = &types.InspectionReportResponse{
		Results:      make([]types.InspectionResult, 0, len(rpcResp.Results)),
		ResultsTotal: rpcResp.ResultsTotal,
	}
	if record := convertRecord(rpcResp.Record); record != nil {
		resp.Record = *record
	}
	for _, item := range rpcResp.Results {
		if converted := convertResult(item); converted != nil {
			resp.Results = append(resp.Results, *converted)
		}
	}

	return resp, nil
}
