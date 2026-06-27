package executionservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelinePreviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelinePreviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelinePreviewLogic {
	return &PipelinePreviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelinePreviewLogic) PipelinePreview(in *pb.GetPipelineReq) (*pb.PipelinePreviewResp, error) {
	data, err := l.svcCtx.PipelineModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("流水线预览失败: %v", err)
		return nil, err
	}
	if err := ensurePipelineAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线预览失败: %v", err)
		return nil, err
	}
	if data.EngineType == engineTekton {
		runtime, err := buildRuntime(l.ctx, l.svcCtx, data.ProjectID, data.SystemID, data.EnvironmentID, data.BuildChannelBindingID, in.CurrentUserId, in.CurrentRoles)
		if err != nil {
			l.Errorf("生成 Tekton Pipeline 失败: %v", err)
			return nil, err
		}
		if runtime == nil || runtime.Binding == nil {
			l.Errorf("生成 Tekton Pipeline 失败: Tekton 运行时配置不完整")
			return nil, errorx.Msg("Tekton 运行时配置不完整")
		}
		cfg, err := parseTektonBindingConfig(runtime.Binding.BindingConfig)
		if err != nil {
			l.Errorf("生成 Tekton Pipeline 失败: %v", err)
			return nil, err
		}
		taskNamespace := ""
		if runtime.Channel != nil {
			taskCfg, err := parseTektonChannelTaskConfig(runtime.Channel.Config)
			if err != nil {
				l.Errorf("生成 Tekton Pipeline 失败: %v", err)
				return nil, err
			}
			taskNamespace = taskCfg.TaskNamespace
		}
		content, err := renderTektonPipelineContent(l.ctx, nil, data, cfg, taskNamespace, nil)
		if err != nil {
			l.Errorf("生成 Tekton Pipeline 失败: %v", err)
			return nil, errorx.Msg("生成 Tekton Pipeline 失败")
		}
		return &pb.PipelinePreviewResp{Pipeline: content}, nil
	}

	rendered, err := renderPipelineWithMetadata(l.ctx, l.svcCtx, data, pipelineRenderContext{UserID: in.CurrentUserId, Roles: in.CurrentRoles})
	if err != nil {
		l.Errorf("生成 Jenkins Pipeline 失败: %v", err)
		return nil, errorx.Msg("生成 Jenkins Pipeline 失败")
	}
	return &pb.PipelinePreviewResp{Pipeline: rendered.Script}, nil
}
