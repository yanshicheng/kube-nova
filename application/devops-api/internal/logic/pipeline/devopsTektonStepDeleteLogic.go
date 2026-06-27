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

type DevopsTektonStepDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 Tekton 步骤
func NewDevopsTektonStepDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonStepDeleteLogic {
	return &DevopsTektonStepDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonStepDeleteLogic) DevopsTektonStepDelete(req *types.DefaultStringIdRequest) error {
	_, err := l.svcCtx.PipelineRpc.TektonStepDelete(l.ctx, &pipelineconfigservice.DeleteByIdReq{
		Id:        req.Id,
		UpdatedBy: currentUsername(l.ctx),
	})
	return err
}
