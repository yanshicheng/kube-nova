// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pipeline

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsPipelineTemplateCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建流水线模板
func NewDevopsPipelineTemplateCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineTemplateCreateLogic {
	return &DevopsPipelineTemplateCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineTemplateCreateLogic) DevopsPipelineTemplateCreate(req *types.CreateDevopsPipelineTemplateRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.PipelineTemplateCreate(l.ctx, &pipelineconfigservice.CreatePipelineTemplateReq{
		Name:                       req.Name,
		Code:                       req.Code,
		Icon:                       req.Icon,
		IconColor:                  req.IconColor,
		Description:                req.Description,
		Scope:                      req.Scope,
		ProjectId:                  req.ProjectId,
		EngineType:                 req.EngineType,
		Steps:                      pipelineStepsToRpc(req.Steps),
		TektonDagConfig:            req.TektonDagConfig,
		TektonRunPolicy:            req.TektonRunPolicy,
		TektonTriggerConfig:        req.TektonTriggerConfig,
		TektonPrunerPolicyRef:      req.TektonPrunerPolicyRef,
		TektonPipelineYamlSnapshot: req.TektonPipelineYamlSnapshot,
		Status:                     req.Status,
		CreatedBy:                  currentUsername(l.ctx),
		CurrentUserId:              currentUserID(l.ctx),
		CurrentRoles:               currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.IdResponse{Id: result.Id}, nil
}
