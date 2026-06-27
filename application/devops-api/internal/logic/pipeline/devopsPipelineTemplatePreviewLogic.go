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

type DevopsPipelineTemplatePreviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 预览 Jenkins Pipeline
func NewDevopsPipelineTemplatePreviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineTemplatePreviewLogic {
	return &DevopsPipelineTemplatePreviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineTemplatePreviewLogic) DevopsPipelineTemplatePreview(req *types.DefaultStringIdRequest) (resp *types.PipelinePreviewResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.PipelineTemplatePreview(l.ctx, &pipelineconfigservice.GetByIdReq{
		Id:            req.Id,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.PipelinePreviewResponse{Pipeline: result.Pipeline}, nil
}
