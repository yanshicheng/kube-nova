package pipelineconfigservicelogic

import (
	"context"
	"errors"
	"github.com/zeromicro/go-zero/core/logx"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

func jenkinsAgentContainersFromPb(items []*pb.JenkinsAgentContainer) []model.JenkinsAgentContainer {
	result := make([]model.JenkinsAgentContainer, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, model.JenkinsAgentContainer{
			Name:  strings.TrimSpace(item.Name),
			Image: strings.TrimSpace(item.Image),
		})
	}
	return model.NormalizeJenkinsAgentContainers(result)
}

func jenkinsAgentContainersToPb(items []model.JenkinsAgentContainer) []*pb.JenkinsAgentContainer {
	result := make([]*pb.JenkinsAgentContainer, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		result = append(result, &pb.JenkinsAgentContainer{
			Name:  item.Name,
			Image: item.Image,
		})
	}
	return result
}

func jenkinsAgentToPb(ctx context.Context, svcCtx *svc.ServiceContext, in *model.DevopsJenkinsAgent) *pb.DevopsJenkinsAgent {
	if in == nil {
		return nil
	}
	channelName := ""
	if in.ChannelID != "" {
		if channel, err := svcCtx.ChannelModel.FindOne(ctx, in.ChannelID); err == nil && channel != nil {
			channelName = channel.Name
		}
	}
	return &pb.DevopsJenkinsAgent{
		Id:          in.ID.Hex(),
		ChannelId:   in.ChannelID,
		ChannelName: channelName,
		Name:        in.Name,
		Code:        in.Code,
		AgentType:   normalizeJenkinsAgentType(in.AgentType),
		MatchMode:   in.MatchMode,
		MatchValue:  in.MatchValue,
		Cloud:       in.Cloud,
		PodYaml:     in.PodYaml,
		Containers:  jenkinsAgentContainersToPb(in.Containers),
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
		CreatedAt:   in.CreateAt.Unix(),
		UpdatedAt:   in.UpdateAt.Unix(),
	}
}

func normalizeJenkinsAgentType(agentType string) string {
	agentType = strings.TrimSpace(agentType)
	if agentType == "pod" {
		return "dynamic"
	}
	if agentType == "" {
		return "static"
	}
	return agentType
}

func normalizeJenkinsAgentInput(channelID, name, code, agentType, matchMode, matchValue, cloud, podYaml string, containers []model.JenkinsAgentContainer) (*model.DevopsJenkinsAgent, error) {
	data := &model.DevopsJenkinsAgent{
		ChannelID:  strings.TrimSpace(channelID),
		Name:       strings.TrimSpace(name),
		Code:       strings.TrimSpace(code),
		AgentType:  normalizeJenkinsAgentType(agentType),
		MatchMode:  strings.TrimSpace(matchMode),
		MatchValue: strings.TrimSpace(matchValue),
		Cloud:      strings.TrimSpace(cloud),
		PodYaml:    strings.TrimSpace(podYaml),
		Containers: model.NormalizeJenkinsAgentContainers(containers),
	}
	if data.Name == "" {
		logx.Errorf("Agent 名称不能为空")
		return nil, errorx.Msg("Agent 名称不能为空")
	}
	if data.Code == "" {
		data.Code = data.Name
	}
	if data.AgentType != "static" && data.AgentType != "dynamic" {
		logx.Errorf("Agent 类型必须是 static 或 dynamic")
		return nil, errorx.Msg("Agent 类型必须是 static 或 dynamic")
	}
	if data.MatchMode == "" {
		data.MatchMode = "label"
	}
	if data.MatchMode != "name" && data.MatchMode != "label" {
		logx.Errorf("Agent 匹配模式必须是 name 或 label")
		return nil, errorx.Msg("Agent 匹配模式必须是 name 或 label")
	}
	if data.AgentType == "static" && data.MatchValue == "" {
		logx.Errorf("静态 Agent 必须配置匹配值")
		return nil, errorx.Msg("静态 Agent 必须配置匹配值")
	}
	if data.AgentType == "dynamic" && len(data.Containers) == 0 {
		data.Containers = model.DefaultJenkinsAgentContainers()
	}
	if data.AgentType == "dynamic" {
		hasJnlp := false
		for _, item := range data.Containers {
			if item.Name == "jnlp" {
				hasJnlp = true
				break
			}
		}
		if !hasJnlp {
			data.Containers = append([]model.JenkinsAgentContainer{{Name: "jnlp"}}, data.Containers...)
		}
	}
	return data, nil
}

func ensureJenkinsChannel(ctx context.Context, svcCtx *svc.ServiceContext, channelID string) error {
	if strings.TrimSpace(channelID) == "" {
		logx.Errorf("请选择 Jenkins 渠道")
		return errorx.Msg("请选择 Jenkins 渠道")
	}
	channel, err := svcCtx.ChannelModel.FindOne(ctx, channelID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			logx.Errorf("Jenkins 渠道不存在")
			return errorx.Msg("Jenkins 渠道不存在")
		}
		logx.Errorf("处理 Jenkins Agent 配置失败: %v", err)
		return err
	}
	if channel.ChannelType != "jenkins" {
		logx.Errorf("只能为 Jenkins 渠道配置 Agent")
		return errorx.Msg("只能为 Jenkins 渠道配置 Agent")
	}
	return nil
}

func resolveJenkinsAgentChannelID(ctx context.Context, svcCtx *svc.ServiceContext, channelID, bindingID string) (string, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID != "" {
		return channelID, ensureJenkinsChannel(ctx, svcCtx, channelID)
	}
	bindingID = strings.TrimSpace(bindingID)
	if bindingID == "" {
		return "", nil
	}
	binding, err := svcCtx.ProjectChannelModel.FindOne(ctx, bindingID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			logx.Errorf("构建渠道绑定不存在")
			return "", errorx.Msg("构建渠道绑定不存在")
		}
		logx.Errorf("处理 Jenkins Agent 配置失败: %v", err)
		return "", err
	}
	if binding.ChannelType != "jenkins" {
		logx.Errorf("构建渠道不是 Jenkins 类型")
		return "", errorx.Msg("构建渠道不是 Jenkins 类型")
	}
	return binding.ChannelID, nil
}
