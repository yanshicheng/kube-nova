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

type DevopsJenkinsStepGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Jenkins 步骤详情
func NewDevopsJenkinsStepGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsJenkinsStepGetLogic {
	return &DevopsJenkinsStepGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsJenkinsStepGetLogic) DevopsJenkinsStepGet(req *types.DefaultStringIdRequest) (resp *types.DevopsStepTemplate, err error) {
	result, err := l.svcCtx.PipelineRpc.JenkinsStepGet(l.ctx, &pipelineconfigservice.GetByIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	data := stepTemplateToType(result.Data)

	return &data, nil
}
