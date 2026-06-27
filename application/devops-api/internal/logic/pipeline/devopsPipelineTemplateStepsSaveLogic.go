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

type DevopsPipelineTemplateStepsSaveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 保存流水线模板步骤编排
func NewDevopsPipelineTemplateStepsSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineTemplateStepsSaveLogic {
	return &DevopsPipelineTemplateStepsSaveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineTemplateStepsSaveLogic) DevopsPipelineTemplateStepsSave(req *types.SaveDevopsPipelineTemplateStepsRequest) error {
	_, err := l.svcCtx.PipelineRpc.PipelineTemplateStepsSave(l.ctx, &pipelineconfigservice.SavePipelineTemplateStepsReq{
		Id:            req.Id,
		Steps:         pipelineStepsToRpc(req.Steps),
		UpdatedBy:     currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
