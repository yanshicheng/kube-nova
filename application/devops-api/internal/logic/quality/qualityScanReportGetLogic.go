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

type QualityScanReportGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 扫描报告详情
func NewQualityScanReportGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityScanReportGetLogic {
	return &QualityScanReportGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityScanReportGetLogic) QualityScanReportGet(req *types.DefaultStringIdRequest) (resp *types.QualityScanReport, err error) {
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
	data := scanReportToType(result.Data)

	return &data, nil
}
