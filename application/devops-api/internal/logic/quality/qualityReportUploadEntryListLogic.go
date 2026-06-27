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

type QualityReportUploadEntryListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询报告上传批次明细
func NewQualityReportUploadEntryListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityReportUploadEntryListLogic {
	return &QualityReportUploadEntryListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityReportUploadEntryListLogic) QualityReportUploadEntryList(req *types.ListQualityReportUploadEntryRequest) (resp *types.ListQualityReportUploadEntryResponse, err error) {
	projectIDs, restricted, err := resolveProjectScope(l.ctx, l.svcCtx, "")
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.QualityRpc.ReportUploadEntryList(l.ctx, &qualityservice.ListReportUploadEntryReq{
		Page:              req.Page,
		PageSize:          req.PageSize,
		BatchId:           req.BatchId,
		RunId:             req.RunId,
		Status:            req.Status,
		CurrentUserId:     currentUserID(l.ctx),
		CurrentRoles:      currentRoles(l.ctx),
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	})
	if err != nil {
		return nil, err
	}

	return &types.ListQualityReportUploadEntryResponse{
		Items: uploadEntriesToType(result.Data),
		Total: result.Total,
	}, nil
}
