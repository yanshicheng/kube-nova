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

type DevopsStepCategoryGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取步骤分类详情
func NewDevopsStepCategoryGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsStepCategoryGetLogic {
	return &DevopsStepCategoryGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsStepCategoryGetLogic) DevopsStepCategoryGet(req *types.DefaultStringIdRequest) (resp *types.DevopsStepCategory, err error) {
	result, err := l.svcCtx.PipelineRpc.StepCategoryGet(l.ctx, &pipelineconfigservice.GetByIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	data := stepCategoryToType(result.Data)

	return &data, nil
}
