package executionservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-quality-rpc/client/qualityservice"
)

func createScanPlanSnapshot(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun, pipeline *model.DevopsPipeline, runParams map[string]string, operator string) error {
	if pipeline == nil || run == nil || !pipeline.ScanEnabled {
		return nil
	}
	paramByCode := make(map[string]model.PipelineParam, len(pipeline.Params))
	for _, item := range pipeline.Params {
		paramByCode[item.Code] = item
	}
	items := make([]*qualityservice.ScanPlanItem, 0, len(pipeline.ScanItems))
	for _, item := range pipeline.ScanItems {
		targetName := item.TargetName
		if targetName == "" && item.TargetParamCode != "" {
			targetName = runParams[item.TargetParamCode]
			if targetName == "" {
				if param, ok := paramByCode[item.TargetParamCode]; ok {
					targetName = pipelineParamValue(param)
				}
			}
		}
		items = append(items, &qualityservice.ScanPlanItem{
			StepId:          item.StepID,
			StageId:         item.StageID,
			Tool:            item.Tool,
			ToolMode:        item.ToolMode,
			ToolBindingId:   item.ToolBindingID,
			TargetType:      item.TargetType,
			TargetName:      targetName,
			TargetParamCode: item.TargetParamCode,
			ReportSource:    item.ReportSource,
			ReportPath:      item.ReportPath,
			ReportFormat:    item.ReportFormat,
			Required:        item.Required,
			Enforce:         item.Enforce,
			Parser:          item.Parser,
		})
	}
	_, err := svcCtx.QualityRpc.ScanPlanSnapshotCreate(ctx, &qualityservice.CreateScanPlanSnapshotReq{
		RunId:                   run.ID.Hex(),
		PipelineId:              pipeline.ID.Hex(),
		ProjectId:               pipeline.ProjectID,
		ProjectName:             pipeline.ProjectName,
		ProjectCode:             pipeline.ProjectCode,
		SystemId:                pipeline.SystemID,
		SystemName:              pipeline.SystemName,
		SystemCode:              pipeline.SystemCode,
		EnvironmentId:           pipeline.EnvironmentID,
		EnvironmentName:         pipeline.EnvironmentName,
		EnvironmentCode:         pipeline.EnvironmentCode,
		ScanMode:                pipeline.ScanMode,
		ScanEnabled:             pipeline.ScanEnabled,
		RejectUnexpectedReports: pipeline.RejectUnexpectedReports,
		Enforce:                 pipeline.Enforce,
		Items:                   items,
		CreatedBy:               operator,
	})
	return err
}
