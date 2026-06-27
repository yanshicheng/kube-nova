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

type DevopsJenkinsAgentGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// Jenkins Agent 详情
func NewDevopsJenkinsAgentGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsJenkinsAgentGetLogic {
	return &DevopsJenkinsAgentGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsJenkinsAgentGetLogic) DevopsJenkinsAgentGet(req *types.DefaultStringIdRequest) (resp *types.DevopsJenkinsAgent, err error) {
	result, err := l.svcCtx.PipelineRpc.JenkinsAgentGet(l.ctx, &pipelineconfigservice.GetByIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	data := devopsJenkinsAgentToType(result.Data)

	return &data, nil
}
