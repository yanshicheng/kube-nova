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

type QualityReportUploadBatchListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询报告上传批次
func NewQualityReportUploadBatchListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityReportUploadBatchListLogic {
	return &QualityReportUploadBatchListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityReportUploadBatchListLogic) QualityReportUploadBatchList(req *types.ListQualityReportUploadBatchRequest) (resp *types.ListQualityReportUploadBatchResponse, err error) {
	projectIDs, restricted, err := resolveProjectScope(l.ctx, l.svcCtx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.QualityRpc.ReportUploadBatchList(l.ctx, &qualityservice.ListReportUploadBatchReq{
		Page:              req.Page,
		PageSize:          req.PageSize,
		ProjectId:         req.ProjectId,
		SystemId:          req.SystemId,
		PipelineId:        req.PipelineId,
		RunId:             req.RunId,
		Tool:              req.Tool,
		Status:            req.Status,
		CurrentUserId:     currentUserID(l.ctx),
		CurrentRoles:      currentRoles(l.ctx),
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	})
	if err != nil {
		return nil, err
	}

	return &types.ListQualityReportUploadBatchResponse{
		Items: uploadBatchesToType(result.Data),
		Total: result.Total,
	}, nil
}
