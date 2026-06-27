// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package quality

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type QualityReportUploadEntryPreviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 报告上传明细预览
func NewQualityReportUploadEntryPreviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityReportUploadEntryPreviewLogic {
	return &QualityReportUploadEntryPreviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityReportUploadEntryPreviewLogic) QualityReportUploadEntryPreview(req *types.DefaultStringIdRequest) (resp string, err error) {
	return "", nil
}

func (l *QualityReportUploadEntryPreviewLogic) Open(req *types.DefaultStringIdRequest) (*ReportObjectStream, error) {
	return openUploadEntryObject(l.ctx, l.svcCtx, req.Id, true)
}
