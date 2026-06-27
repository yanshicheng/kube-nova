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

type DevopsPipelineJenkinsAgentListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询 Jenkins Agent
func NewDevopsPipelineJenkinsAgentListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineJenkinsAgentListLogic {
	return &DevopsPipelineJenkinsAgentListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineJenkinsAgentListLogic) DevopsPipelineJenkinsAgentList(req *types.JenkinsAgentListRequest) (resp *types.JenkinsAgentListResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.JenkinsAgentList(l.ctx, &pipelineconfigservice.ListJenkinsAgentReq{
		Page:                  1,
		PageSize:              200,
		BuildChannelBindingId: req.BuildChannelBindingId,
		Status:                1,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.JenkinsAgentOption, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, types.JenkinsAgentOption{
			Id:          item.Id,
			Name:        item.Name,
			DisplayName: item.Name,
			Label:       item.MatchValue,
			Labels:      []string{item.MatchValue},
			AgentType:   item.AgentType,
			MatchMode:   item.MatchMode,
			MatchValue:  item.MatchValue,
			Cloud:       item.Cloud,
			PodYaml:     item.PodYaml,
			Containers:  jenkinsAgentContainersToType(item.Containers),
		})
	}

	return &types.JenkinsAgentListResponse{Items: items}, nil
}
