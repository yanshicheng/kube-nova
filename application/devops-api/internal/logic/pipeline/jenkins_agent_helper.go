package pipeline

import (
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"
)

func jenkinsAgentContainersToRpc(items []types.JenkinsAgentContainer) []*pipelineconfigservice.JenkinsAgentContainer {
	result := make([]*pipelineconfigservice.JenkinsAgentContainer, 0, len(items))
	for _, item := range items {
		result = append(result, &pipelineconfigservice.JenkinsAgentContainer{
			Name:  item.Name,
			Image: item.Image,
		})
	}
	return result
}

func jenkinsAgentContainersToType(items []*pipelineconfigservice.JenkinsAgentContainer) []types.JenkinsAgentContainer {
	result := make([]types.JenkinsAgentContainer, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, types.JenkinsAgentContainer{
			Name:  item.Name,
			Image: item.Image,
		})
	}
	return result
}

func devopsJenkinsAgentToType(in *pipelineconfigservice.DevopsJenkinsAgent) types.DevopsJenkinsAgent {
	if in == nil {
		return types.DevopsJenkinsAgent{}
	}
	return types.DevopsJenkinsAgent{
		Id:          in.Id,
		ChannelId:   in.ChannelId,
		ChannelName: in.ChannelName,
		Name:        in.Name,
		Code:        in.Code,
		AgentType:   in.AgentType,
		MatchMode:   in.MatchMode,
		MatchValue:  in.MatchValue,
		Cloud:       in.Cloud,
		PodYaml:     in.PodYaml,
		Containers:  jenkinsAgentContainersToType(in.Containers),
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
		CreatedAt:   in.CreatedAt,
		UpdatedAt:   in.UpdatedAt,
	}
}
