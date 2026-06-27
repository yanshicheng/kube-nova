// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pipeline

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/client/executionservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsPipelineUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新流水线
func NewDevopsPipelineUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineUpdateLogic {
	return &DevopsPipelineUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineUpdateLogic) DevopsPipelineUpdate(req *types.UpdateDevopsPipelineRequest) error {
	_, err := l.svcCtx.PipelineExecRpc.PipelineUpdate(l.ctx, &executionservice.UpdatePipelineReq{
		Id:                      req.Id,
		SystemId:                req.SystemId,
		EnvironmentId:           req.EnvironmentId,
		Name:                    req.Name,
		EngineType:              req.EngineType,
		TemplateId:              req.TemplateId,
		BuildChannelBindingId:   req.BuildChannelBindingId,
		Steps:                   execStepsToRpc(req.Steps),
		Params:                  pipelineParamsToRpc(req.Params),
		Agent:                   agentToRpc(req.Agent),
		Tools:                   toolsToRpc(req.Tools),
		Options:                 execOptionsToRpc(req.Options),
		Triggers:                triggersToRpc(req.Triggers),
		TriggerMode:             req.TriggerMode,
		ScanEnabled:             req.ScanEnabled,
		ScanMode:                req.ScanMode,
		RejectUnexpectedReports: req.RejectUnexpectedReports,
		Enforce:                 req.Enforce,
		ScanItems:               pipelineScanItemsToRpc(req.ScanItems),
		TektonDagConfig:         req.TektonDagConfig,
		TektonRunPolicy:         req.TektonRunPolicy,
		TektonTriggerConfig:     req.TektonTriggerConfig,
		TektonPrunerPolicyRef:   req.TektonPrunerPolicyRef,
		TektonTemplateId:        req.TektonTemplateId,
		TektonTemplateSnapshot:  req.TektonTemplateSnapshot,
		TektonParamBindings:     req.TektonParamBindings,
		TektonWorkspaceBindings: req.TektonWorkspaceBindings,
		TektonResourceBindings:  req.TektonResourceBindings,
		Status:                  req.Status,
		UpdatedBy:               currentUsername(l.ctx),
		CurrentUserId:           currentUserID(l.ctx),
		CurrentRoles:            currentRoles(l.ctx),
	})
	return err
}
