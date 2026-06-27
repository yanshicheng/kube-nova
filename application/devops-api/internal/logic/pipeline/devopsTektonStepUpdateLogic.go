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

type DevopsTektonStepUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 Tekton 步骤
func NewDevopsTektonStepUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonStepUpdateLogic {
	return &DevopsTektonStepUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonStepUpdateLogic) DevopsTektonStepUpdate(req *types.UpdateDevopsStepTemplateRequest) error {
	_, err := l.svcCtx.PipelineRpc.TektonStepUpdate(l.ctx, &pipelineconfigservice.UpdateStepTemplateReq{
		Id:             req.Id,
		Name:           req.Name,
		Icon:           req.Icon,
		IconColor:      req.IconColor,
		Description:    req.Description,
		Type:           req.Type,
		CategoryId:     req.CategoryId,
		StageContent:   req.StageContent,
		Params:         stepParamsToRpc(req.Params),
		TaskParams:     tektonTaskParamsToRpc(req.TaskParams),
		TaskResults:    tektonTaskResultsToRpc(req.TaskResults),
		TaskWorkspaces: tektonWorkspacesToRpc(req.TaskWorkspaces),
		ArtifactConfig: artifactConfigToRpc(req.ArtifactConfig),
		Status:         req.Status,
		UpdatedBy:      currentUsername(l.ctx),
	})
	return err
}
