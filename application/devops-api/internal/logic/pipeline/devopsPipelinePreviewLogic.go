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

type DevopsPipelinePreviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 预览流水线
func NewDevopsPipelinePreviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelinePreviewLogic {
	return &DevopsPipelinePreviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelinePreviewLogic) DevopsPipelinePreview(req *types.DefaultStringIdRequest) (resp *types.PipelinePreviewResponse, err error) {
	result, err := l.svcCtx.PipelineExecRpc.PipelinePreview(l.ctx, &executionservice.GetPipelineReq{
		Id:            req.Id,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.PipelinePreviewResponse{Pipeline: result.Pipeline}, nil
}
