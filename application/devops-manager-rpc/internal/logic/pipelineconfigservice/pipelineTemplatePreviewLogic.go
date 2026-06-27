package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineTemplatePreviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineTemplatePreviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineTemplatePreviewLogic {
	return &PipelineTemplatePreviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineTemplatePreviewLogic) PipelineTemplatePreview(in *pb.GetByIdReq) (*pb.PipelinePreviewResp, error) {
	data, err := l.svcCtx.PipelineTemplateModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("流水线模板预览失败: %v", err)
		return nil, err
	}
	if err := ensureTemplateReadAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线模板预览失败: %v", err)
		return nil, err
	}
	if normalizeTemplateEngineType(data.EngineType) != jenkinsEngineType {
		l.Errorf("Tekton 模板预览暂未实现")
		return nil, errorx.Msg("Tekton 模板预览暂未实现")
	}
	pipeline, err := renderJenkinsPipeline(l.ctx, l.svcCtx, data)
	if err != nil {
		l.Errorf("流水线模板预览失败: %v", err)
		return nil, err
	}

	return &pb.PipelinePreviewResp{Pipeline: pipeline}, nil
}
