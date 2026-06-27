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

type DevopsTektonStepGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Tekton 步骤详情
func NewDevopsTektonStepGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonStepGetLogic {
	return &DevopsTektonStepGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonStepGetLogic) DevopsTektonStepGet(req *types.DefaultStringIdRequest) (resp *types.DevopsStepTemplate, err error) {
	result, err := l.svcCtx.PipelineRpc.TektonStepGet(l.ctx, &pipelineconfigservice.GetByIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	data := stepTemplateToType(result.Data)

	return &data, nil
}
