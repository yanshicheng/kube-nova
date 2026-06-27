// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package quality

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type QualityScanReportPreviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 扫描报告预览
func NewQualityScanReportPreviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityScanReportPreviewLogic {
	return &QualityScanReportPreviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityScanReportPreviewLogic) QualityScanReportPreview(req *types.DefaultStringIdRequest) (resp string, err error) {
	return "", nil
}

func (l *QualityScanReportPreviewLogic) Open(req *types.DefaultStringIdRequest) (*ReportObjectStream, error) {
	return openScanReportObject(l.ctx, l.svcCtx, req.Id, true)
}
