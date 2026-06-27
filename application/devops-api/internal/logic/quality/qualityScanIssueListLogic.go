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

type QualityScanIssueListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询扫描问题
func NewQualityScanIssueListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityScanIssueListLogic {
	return &QualityScanIssueListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityScanIssueListLogic) QualityScanIssueList(req *types.ListQualityScanIssueRequest) (resp *types.ListQualityScanIssueResponse, err error) {
	projectIDs, restricted, err := resolveProjectScope(l.ctx, l.svcCtx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.QualityRpc.ScanIssueList(l.ctx, &qualityservice.ListScanIssueReq{
		Page:              req.Page,
		PageSize:          req.PageSize,
		ReportId:          req.ReportId,
		ProjectId:         req.ProjectId,
		SystemId:          req.SystemId,
		RunId:             req.RunId,
		Tool:              req.Tool,
		TargetType:        req.TargetType,
		IssueType:         req.IssueType,
		Severity:          req.Severity,
		Status:            req.Status,
		CurrentUserId:     currentUserID(l.ctx),
		CurrentRoles:      currentRoles(l.ctx),
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.QualityScanIssue, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, scanIssueToType(item))
	}

	return &types.ListQualityScanIssueResponse{Items: items, Total: result.Total}, nil
}
