package adapter

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

// ChannelManagerAdapter 渠道管理器适配器
type ChannelManagerAdapter struct {
	channelModel     *model.DevopsChannelModel
	channelTypeModel *model.DevopsChannelTypeModel
}

// NewChannelManagerAdapter 创建渠道管理器适配器
func NewChannelManagerAdapter(channelModel *model.DevopsChannelModel, channelTypeModel *model.DevopsChannelTypeModel) *ChannelManagerAdapter {
	return &ChannelManagerAdapter{
		channelModel:     channelModel,
		channelTypeModel: channelTypeModel,
	}
}

// GetChannelTypeByCode 根据编码获取渠道类型
func (a *ChannelManagerAdapter) GetChannelTypeByCode(ctx context.Context, code string) (*channelvars.ChannelType, error) {
	channelType, err := a.channelTypeModel.FindOneByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("查询渠道类型失败: %w", err)
	}

	return &channelvars.ChannelType{
		ID:           channelType.ID.Hex(),
		Code:         channelType.Code,
		Name:         channelType.Name,
		GroupCode:    channelType.GroupCode,
		SpecProfile:  "", // TODO: DevopsChannelType需要添加SpecProfile字段
		ProviderType: providerTypeByChannelType(channelType.Code),
	}, nil
}

// GetInstance 获取渠道实例
func (a *ChannelManagerAdapter) GetInstance(ctx context.Context, id int64) (*channelvars.ChannelInstance, error) {
	// MongoDB使用string ID
	idStr := fmt.Sprintf("%d", id)
	channel, err := a.channelModel.FindOne(ctx, idStr)
	if err != nil {
		return nil, fmt.Errorf("查询渠道实例失败: %w", err)
	}

	channelType, err := a.channelTypeModel.FindOneByCode(ctx, channel.ChannelType)
	if err != nil {
		return nil, fmt.Errorf("查询渠道类型失败: %w", err)
	}

	return &channelvars.ChannelInstance{
		ID:           id,
		Name:         channel.Name,
		Code:         channel.Code,
		Endpoint:     channel.Endpoint,
		ChannelType:  channelType.Code,
		ProviderType: providerTypeByChannelType(channelType.Code),
		CredentialID: 0, // TODO: 转换CredentialId
		Config:       nil,
	}, nil
}

// FindInstancesByEndpoint 根据endpoint查找渠道实例
func (a *ChannelManagerAdapter) FindInstancesByEndpoint(ctx context.Context, endpoint string, projectID int64) ([]*channelvars.ChannelInstance, error) {
	channels, err := a.channelModel.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询渠道实例失败: %w", err)
	}
	result := make([]*channelvars.ChannelInstance, 0)
	for _, channel := range channels {
		if channel == nil || channel.Status != 1 {
			continue
		}
		if !endpointMatches(endpoint, channel.Endpoint) {
			continue
		}
		result = append(result, &channelvars.ChannelInstance{
			Name:         channel.Name,
			Code:         channel.Code,
			Endpoint:     channel.Endpoint,
			ChannelType:  channel.ChannelType,
			ProviderType: providerTypeByChannelType(channel.ChannelType),
			CredentialID: 0,
			Config:       nil,
		})
	}
	return result, nil
}

func providerTypeByChannelType(channelType string) string {
	switch strings.TrimSpace(channelType) {
	case "gitlab", "github", "gitee":
		return "git"
	case "svn":
		return "svn"
	case "harbor":
		return "harbor"
	case "registry", "aliyun_registry":
		return "registry"
	case "nexus":
		return "nexus"
	case "jfrog":
		return "jfrog"
	case "sonarqube":
		return "sonarqube"
	case "spotbugs":
		return "spotbugs"
	case "trivy":
		return "trivy"
	case "kube_bench":
		return "kube_bench"
	case "jenkins":
		return "jenkins"
	case "tekton":
		return "tekton"
	case "buildkit":
		return "buildkit"
	case "kubernetes":
		return "kubernetes"
	case "host":
		return "host"
	case "host_group":
		return "host_group"
	case "kube-nova":
		return "kube-nova"
	case "argocd":
		return "argocd"
	default:
		return strings.TrimSpace(channelType)
	}
}

func endpointMatches(source, endpoint string) bool {
	source = normalizeEndpoint(source)
	endpoint = normalizeEndpoint(endpoint)
	if source == "" || endpoint == "" {
		return false
	}
	if strings.EqualFold(source, endpoint) {
		return true
	}
	sourceHost, sourcePort, sourceOK := splitEndpointHostPort(source)
	endpointHost, endpointPort, endpointOK := splitEndpointHostPort(endpoint)
	if sourceOK && endpointOK && strings.EqualFold(sourceHost, endpointHost) {
		return sourcePort == "" || endpointPort == "" || sourcePort == endpointPort
	}
	return false
}

func splitEndpointHostPort(value string) (string, string, bool) {
	if value == "" {
		return "", "", false
	}
	host, port, err := net.SplitHostPort(value)
	if err == nil {
		return strings.ToLower(host), port, true
	}
	return strings.ToLower(value), "", true
}

func normalizeEndpoint(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return ""
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		if parsed.Port() != "" {
			return strings.ToLower(parsed.Host)
		}
		return strings.ToLower(parsed.Hostname())
	}
	if host, port, err := net.SplitHostPort(value); err == nil {
		if port != "" {
			return strings.ToLower(net.JoinHostPort(host, port))
		}
		return strings.ToLower(host)
	}
	return strings.ToLower(value)
}
