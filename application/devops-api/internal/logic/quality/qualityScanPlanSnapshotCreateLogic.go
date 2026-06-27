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

type QualityScanPlanSnapshotCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建扫描计划快照
func NewQualityScanPlanSnapshotCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityScanPlanSnapshotCreateLogic {
	return &QualityScanPlanSnapshotCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityScanPlanSnapshotCreateLogic) QualityScanPlanSnapshotCreate(req *types.CreateQualityScanPlanSnapshotRequest) (resp *types.IdResponse, err error) {
	if _, _, err := resolveProjectScope(l.ctx, l.svcCtx, req.ProjectId); err != nil {
		return nil, err
	}
	items := make([]*qualityservice.ScanPlanItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, scanPlanItemToRpc(item))
	}
	result, err := l.svcCtx.QualityRpc.ScanPlanSnapshotCreate(l.ctx, &qualityservice.CreateScanPlanSnapshotReq{
		RunId:                   req.RunId,
		PipelineId:              req.PipelineId,
		ProjectId:               req.ProjectId,
		ProjectName:             req.ProjectName,
		ProjectCode:             req.ProjectCode,
		SystemId:                req.SystemId,
		SystemName:              req.SystemName,
		SystemCode:              req.SystemCode,
		EnvironmentId:           req.EnvironmentId,
		EnvironmentName:         req.EnvironmentName,
		EnvironmentCode:         req.EnvironmentCode,
		ScanMode:                req.ScanMode,
		ScanEnabled:             req.ScanEnabled,
		RejectUnexpectedReports: req.RejectUnexpectedReports,
		Enforce:                 req.Enforce,
		Items:                   items,
		CreatedBy:               currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.IdResponse{Id: result.Id}, nil
}
