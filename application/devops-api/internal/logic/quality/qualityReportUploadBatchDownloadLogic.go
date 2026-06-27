// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package quality

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type QualityReportUploadBatchDownloadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 报告上传批次原件下载
func NewQualityReportUploadBatchDownloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityReportUploadBatchDownloadLogic {
	return &QualityReportUploadBatchDownloadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityReportUploadBatchDownloadLogic) QualityReportUploadBatchDownload(req *types.DefaultStringIdRequest) (resp string, err error) {
	return "", nil
}

func (l *QualityReportUploadBatchDownloadLogic) Open(req *types.DefaultStringIdRequest) (*ReportObjectStream, error) {
	return openUploadBatchObject(l.ctx, l.svcCtx, req.Id)
}
