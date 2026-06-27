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

type DevopsTektonStepImageBatchUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 批量更新 Tekton 步骤镜像
func NewDevopsTektonStepImageBatchUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonStepImageBatchUpdateLogic {
	return &DevopsTektonStepImageBatchUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonStepImageBatchUpdateLogic) DevopsTektonStepImageBatchUpdate(req *types.BatchUpdateTektonStepImageRequest) (resp *types.BatchUpdateTektonStepImageResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.TektonStepImageBatchUpdate(l.ctx, &pipelineconfigservice.BatchUpdateTektonStepImageReq{
		Items:     tektonStepImageUpdatesToRpc(req.Items),
		UpdatedBy: currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.BatchUpdateTektonStepImageResponse{Total: result.Total, Updated: result.Updated}, nil
}
