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

type DevopsJenkinsAgentCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 Jenkins Agent
func NewDevopsJenkinsAgentCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsJenkinsAgentCreateLogic {
	return &DevopsJenkinsAgentCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsJenkinsAgentCreateLogic) DevopsJenkinsAgentCreate(req *types.CreateDevopsJenkinsAgentRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.JenkinsAgentCreate(l.ctx, &pipelineconfigservice.CreateJenkinsAgentReq{
		ChannelId:  req.ChannelId,
		Name:       req.Name,
		Code:       req.Code,
		AgentType:  req.AgentType,
		MatchMode:  req.MatchMode,
		MatchValue: req.MatchValue,
		Cloud:      req.Cloud,
		PodYaml:    req.PodYaml,
		Containers: jenkinsAgentContainersToRpc(req.Containers),
		Status:     req.Status,
		CreatedBy:  currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.IdResponse{Id: result.Id}, nil
}
