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

type DevopsJenkinsAgentListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询 Jenkins Agent
func NewDevopsJenkinsAgentListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsJenkinsAgentListLogic {
	return &DevopsJenkinsAgentListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsJenkinsAgentListLogic) DevopsJenkinsAgentList(req *types.ListDevopsJenkinsAgentRequest) (resp *types.ListDevopsJenkinsAgentResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.JenkinsAgentList(l.ctx, &pipelineconfigservice.ListJenkinsAgentReq{
		Page:                  req.Page,
		PageSize:              req.PageSize,
		ChannelId:             req.ChannelId,
		BuildChannelBindingId: req.BuildChannelBindingId,
		Name:                  req.Name,
		AgentType:             req.AgentType,
		Status:                req.Status,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsJenkinsAgent, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, devopsJenkinsAgentToType(item))
	}

	return &types.ListDevopsJenkinsAgentResponse{Items: items, Total: result.Total}, nil
}
