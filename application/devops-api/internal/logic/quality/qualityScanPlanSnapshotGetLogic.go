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

type QualityScanPlanSnapshotGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取扫描计划快照
func NewQualityScanPlanSnapshotGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityScanPlanSnapshotGetLogic {
	return &QualityScanPlanSnapshotGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityScanPlanSnapshotGetLogic) QualityScanPlanSnapshotGet(req *types.QualityScanPlanSnapshotRequest) (resp *types.QualityScanPlanSnapshotResponse, err error) {
	projectIDs, restricted, err := resolveProjectScope(l.ctx, l.svcCtx, "")
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.QualityRpc.ScanPlanSnapshotGet(l.ctx, &qualityservice.GetScanPlanSnapshotReq{
		RunId:             req.RunId,
		CurrentUserId:     currentUserID(l.ctx),
		CurrentRoles:      currentRoles(l.ctx),
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.QualityScanPlanItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, scanPlanItemToType(item))
	}

	return &types.QualityScanPlanSnapshotResponse{
		Id:                      result.Id,
		RunId:                   result.RunId,
		PipelineId:              result.PipelineId,
		ProjectId:               result.ProjectId,
		ScanMode:                result.ScanMode,
		ScanEnabled:             result.ScanEnabled,
		RejectUnexpectedReports: result.RejectUnexpectedReports,
		Items:                   items,
		CreatedAt:               result.CreatedAt,
	}, nil
}
