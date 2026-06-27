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

type QualityRunSummaryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 流水线运行质量汇总
func NewQualityRunSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityRunSummaryLogic {
	return &QualityRunSummaryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityRunSummaryLogic) QualityRunSummary(req *types.QualityRunSummaryRequest) (resp *types.QualityRunSummaryResponse, err error) {
	projectIDs, restricted, err := resolveProjectScope(l.ctx, l.svcCtx, "")
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.QualityRpc.QualityRunSummary(l.ctx, &qualityservice.QualityRunSummaryReq{
		RunId:             req.RunId,
		CurrentUserId:     currentUserID(l.ctx),
		CurrentRoles:      currentRoles(l.ctx),
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	})
	if err != nil {
		return nil, err
	}

	return &types.QualityRunSummaryResponse{
		RunId:            result.RunId,
		GateStatus:       result.GateStatus,
		Overview:         dashboardOverviewToType(result.Overview),
		StaticCode:       runTypeSummaryToType(result.StaticCode),
		Image:            runTypeSummaryToType(result.Image),
		ScanPlan:         scanPlanItemsToType(result.ScanPlan),
		Reports:          scanReportsToType(result.Reports),
		StaticCodeIssues: scanIssuesToType(result.StaticCodeIssues),
		ImageIssues:      scanIssuesToType(result.ImageIssues),
		MissingReports:   missingReportsToType(result.MissingReports),
		RejectedReports:  scanReportsToType(result.RejectedReports),
		RejectedEntries:  uploadEntriesToType(result.RejectedEntries),
		Images:           imageSummariesToType(result.Images),
	}, nil
}
