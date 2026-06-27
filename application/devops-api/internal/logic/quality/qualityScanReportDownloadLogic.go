// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package quality

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type QualityScanReportDownloadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 扫描报告下载
func NewQualityScanReportDownloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityScanReportDownloadLogic {
	return &QualityScanReportDownloadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityScanReportDownloadLogic) QualityScanReportDownload(req *types.DefaultStringIdRequest) (resp string, err error) {
	return "", nil
}

func (l *QualityScanReportDownloadLogic) Open(req *types.DefaultStringIdRequest) (*ReportObjectStream, error) {
	return openScanReportObject(l.ctx, l.svcCtx, req.Id, false)
}
