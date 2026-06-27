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

type QualityGateEvaluateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 扫描门禁评估
func NewQualityGateEvaluateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QualityGateEvaluateLogic {
	return &QualityGateEvaluateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QualityGateEvaluateLogic) QualityGateEvaluate(req *types.QualityGateEvaluateRequest) (resp *types.QualityGateEvaluateResponse, err error) {
	projectIDs, restricted, err := resolveProjectScope(l.ctx, l.svcCtx, "")
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.QualityRpc.ScanGateEvaluate(l.ctx, &qualityservice.ScanGateEvaluateReq{
		RunId:             req.RunId,
		Operator:          currentUsername(l.ctx),
		CurrentUserId:     currentUserID(l.ctx),
		CurrentRoles:      currentRoles(l.ctx),
		ProjectIds:        projectIDs,
		ProjectRestricted: restricted,
	})
	if err != nil {
		return nil, err
	}
	missing := make([]types.QualityGateMissingReport, 0, len(result.MissingReports))
	for _, item := range result.MissingReports {
		if item == nil {
			continue
		}
		missing = append(missing, types.QualityGateMissingReport{
			Tool:         item.Tool,
			StepId:       item.StepId,
			ReportFormat: item.ReportFormat,
			ReportPath:   item.ReportPath,
		})
	}

	return &types.QualityGateEvaluateResponse{
		RunId:          result.RunId,
		GateStatus:     result.GateStatus,
		Message:        result.Message,
		MissingReports: missing,
	}, nil
}
