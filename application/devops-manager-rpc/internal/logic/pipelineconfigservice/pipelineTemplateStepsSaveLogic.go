package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineTemplateStepsSaveLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineTemplateStepsSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineTemplateStepsSaveLogic {
	return &PipelineTemplateStepsSaveLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineTemplateStepsSaveLogic) PipelineTemplateStepsSave(in *pb.SavePipelineTemplateStepsReq) (*pb.EmptyResp, error) {
	exist, err := l.svcCtx.PipelineTemplateModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("保存流水线模板步骤失败: %v", err)
		return nil, err
	}
	if err := ensureTemplateWriteAccess(l.ctx, l.svcCtx, exist, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("保存流水线模板步骤失败: %v", err)
		return nil, err
	}
	steps, err := validateTemplateSteps(l.ctx, l.svcCtx, exist.EngineType, pipelineStepsFromPb(in.Steps))
	if err != nil {
		l.Errorf("保存流水线模板步骤失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.PipelineTemplateModel.UpdateSteps(l.ctx, in.Id, steps, in.UpdatedBy); err != nil {
		l.Errorf("保存流水线模板步骤失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
