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

type DevopsPipelineTemplateUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新流水线模板
func NewDevopsPipelineTemplateUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineTemplateUpdateLogic {
	return &DevopsPipelineTemplateUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineTemplateUpdateLogic) DevopsPipelineTemplateUpdate(req *types.UpdateDevopsPipelineTemplateRequest) error {
	_, err := l.svcCtx.PipelineRpc.PipelineTemplateUpdate(l.ctx, &pipelineconfigservice.UpdatePipelineTemplateReq{
		Id:                         req.Id,
		Name:                       req.Name,
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
		UpdatedBy:                  currentUsername(l.ctx),
		CurrentUserId:              currentUserID(l.ctx),
		CurrentRoles:               currentRoles(l.ctx),
	})
	return err
}
