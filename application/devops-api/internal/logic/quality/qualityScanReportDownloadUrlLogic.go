// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package quality

import (
	"context"
	"net/url"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-quality-rpc/client/qualityservice"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type QualityScanReportDownloadUrlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 扫描报告下载地址
func NewQualityScanReportDownloadUrlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityScanReportDownloadUrlLogic {
	return &QualityScanReportDownloadUrlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityScanReportDownloadUrlLogic) QualityScanReportDownloadUrl(req *types.DefaultStringIdRequest) (resp *types.QualityReportDownloadUrlResponse, err error) {
	projectIDs, restricted, err := resolveProjectScope(l.ctx, l.svcCtx, "")
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.QualityRpc.ScanReportGet(l.ctx, &qualityservice.GetScanReportReq{
		Id:                req.Id,
		CurrentUserId:     currentUserID(l.ctx),
		CurrentRoles:      currentRoles(l.ctx),
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	})
	if err != nil {
		return nil, err
	}
	report := result.Data
	if report == nil || strings.TrimSpace(report.ObjectKey) == "" {
		return nil, errorx.Msg("扫描报告对象地址不存在")
	}
	return &types.QualityReportDownloadUrlResponse{Url: "/devops/v1/quality/report/" + url.PathEscape(req.Id) + "/download"}, nil
}
