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

type DevopsJenkinsAgentUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 Jenkins Agent
func NewDevopsJenkinsAgentUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsJenkinsAgentUpdateLogic {
	return &DevopsJenkinsAgentUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsJenkinsAgentUpdateLogic) DevopsJenkinsAgentUpdate(req *types.UpdateDevopsJenkinsAgentRequest) error {
	_, err := l.svcCtx.PipelineRpc.JenkinsAgentUpdate(l.ctx, &pipelineconfigservice.UpdateJenkinsAgentReq{
		Id:         req.Id,
		Name:       req.Name,
		Code:       req.Code,
		AgentType:  req.AgentType,
		MatchMode:  req.MatchMode,
		MatchValue: req.MatchValue,
		Cloud:      req.Cloud,
		PodYaml:    req.PodYaml,
		Containers: jenkinsAgentContainersToRpc(req.Containers),
		Status:     req.Status,
		UpdatedBy:  currentUsername(l.ctx),
	})

	return err
}
