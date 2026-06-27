// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package quality

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-quality-rpc/client/qualityservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type QualityScanReportListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询扫描报告
func NewQualityScanReportListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityScanReportListLogic {
	return &QualityScanReportListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityScanReportListLogic) QualityScanReportList(req *types.ListQualityScanReportRequest) (resp *types.ListQualityScanReportResponse, err error) {
	projectIDs, restricted, err := resolveProjectScope(l.ctx, l.svcCtx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.QualityRpc.ScanReportList(l.ctx, &qualityservice.ListScanReportReq{
		Page:              req.Page,
		PageSize:          req.PageSize,
		ProjectId:         req.ProjectId,
		SystemId:          req.SystemId,
		PipelineId:        req.PipelineId,
		RunId:             req.RunId,
		Tool:              req.Tool,
		TargetType:        req.TargetType,
		Status:            req.Status,
		GateStatus:        req.GateStatus,
		CurrentUserId:     currentUserID(l.ctx),
		CurrentRoles:      currentRoles(l.ctx),
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.QualityScanReport, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, scanReportToType(item))
	}

	return &types.ListQualityScanReportResponse{Items: items, Total: result.Total}, nil
}
