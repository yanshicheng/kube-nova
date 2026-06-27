// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package quality

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type QualityReportUploadEntryDownloadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 报告上传明细下载
func NewQualityReportUploadEntryDownloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityReportUploadEntryDownloadLogic {
	return &QualityReportUploadEntryDownloadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityReportUploadEntryDownloadLogic) QualityReportUploadEntryDownload(req *types.DefaultStringIdRequest) (resp string, err error) {
	return "", nil
}

func (l *QualityReportUploadEntryDownloadLogic) Open(req *types.DefaultStringIdRequest) (*ReportObjectStream, error) {
	return openUploadEntryObject(l.ctx, l.svcCtx, req.Id, false)
}
