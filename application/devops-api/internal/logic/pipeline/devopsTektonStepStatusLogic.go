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

type DevopsTektonStepStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 Tekton 步骤状态
func NewDevopsTektonStepStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonStepStatusLogic {
	return &DevopsTektonStepStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonStepStatusLogic) DevopsTektonStepStatus(req *types.UpdateStatusRequest) error {
	_, err := l.svcCtx.PipelineRpc.TektonStepStatus(l.ctx, &pipelineconfigservice.UpdateStatusReq{
		Id:        req.Id,
		Status:    req.Status,
		UpdatedBy: currentUsername(l.ctx),
	})
	return err
}
