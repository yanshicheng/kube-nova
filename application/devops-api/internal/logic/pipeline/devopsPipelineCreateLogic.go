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

type DevopsPipelineCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建流水线
func NewDevopsPipelineCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineCreateLogic {
	return &DevopsPipelineCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineCreateLogic) DevopsPipelineCreate(req *types.SaveDevopsPipelineRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.PipelineExecRpc.PipelineCreate(l.ctx, &executionservice.CreatePipelineReq{
		ProjectId:               req.ProjectId,
		SystemId:                req.SystemId,
		EnvironmentId:           req.EnvironmentId,
		Name:                    req.Name,
		Code:                    req.Code,
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
		CreatedBy:               currentUsername(l.ctx),
		CurrentUserId:           currentUserID(l.ctx),
		CurrentRoles:            currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.IdResponse{Id: result.Id}, nil
}
