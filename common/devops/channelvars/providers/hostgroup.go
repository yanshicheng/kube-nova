package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

// HostGroupProvider 主机组Provider实现
type HostGroupProvider struct {
	channelManager   channelvars.ChannelManager
	hostGroupManager HostGroupManager
}

// NewHostGroupProvider 创建主机组Provider
func NewHostGroupProvider(channelManager channelvars.ChannelManager, hostGroupManager HostGroupManager) *HostGroupProvider {
	return &HostGroupProvider{
		channelManager:   channelManager,
		hostGroupManager: hostGroupManager,
	}
}

// Capabilities 声明能力
func (p *HostGroupProvider) Capabilities() channelvars.ProviderCapabilities {
	return channelvars.ProviderCapabilities{
		SupportedQueries: []string{
			"hostgroup.group",
			"hostgroup.targets",
			"hostGroup.groupConfig",
		},
		SupportedAddressFormats: []channelvars.AddressFormat{
			{Field: channelvars.FieldAddressGroupConfig, Pattern: `^\[.*\]$`, Example: `[{"host":"192.168.1.1","port":22,"username":"root","password":"password"}]`},
		},
		SupportsAddressResolve: true,
		RequiresCredential:     false,
	}
}

// QueryOptions 查询选项
func (p *HostGroupProvider) QueryOptions(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	switch req.ProviderKey {
	case "hostgroup.group":
		return p.queryHostGroups(ctx, req)
	case "hostgroup.targets", "hostGroup.groupConfig":
		return &channelvars.QueryOptionsResponse{Options: []channelvars.Option{}}, nil
	default:
		return nil, fmt.Errorf("unsupported provider key: %s", req.ProviderKey)
	}
}

// queryHostGroups 查询主机组列表
func (p *HostGroupProvider) queryHostGroups(ctx context.Context, req *channelvars.QueryOptionsRequest) (*channelvars.QueryOptionsResponse, error) {
	// 获取渠道实例
	_, err := p.channelManager.GetInstance(ctx, req.ChannelInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get channel instance: %w", err)
	}

	// 查询主机组列表
	groups, err := p.hostGroupManager.ListHostGroups(ctx, req.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("list host groups: %w", err)
	}

	options := make([]channelvars.Option, len(groups))
	for i, group := range groups {
		options[i] = channelvars.Option{
			Label: group.Name,
			Value: fmt.Sprintf("%d", group.ID),
		}
	}

	return &channelvars.QueryOptionsResponse{
		Options: options,
	}, nil
}

// ResolveAddress 解析地址
func (p *HostGroupProvider) ResolveAddress(ctx context.Context, req *channelvars.ResolveAddressRequest) (*channelvars.ResolveAddressResponse, error) {
	fields := map[string]string{channelvars.FieldAddressGroupConfig: strings.TrimSpace(req.Address)}
	return resolveHostAddressByCandidates(ctx, p.channelManager, req, fields)
}

// RenderOutput 渲染输出
func (p *HostGroupProvider) RenderOutput(ctx context.Context, req *channelvars.RenderOutputRequest) (*channelvars.RenderOutputResponse, error) {
	if value := strings.TrimSpace(firstValue(req.Values, channelvars.FieldAddressGroupConfig, "groupConfig")); value != "" {
		hosts, err := parseHostConfigs(value)
		if err != nil {
			return nil, err
		}
		output, err := renderHostConfigs(hosts, firstValue(req.Values, channelvars.FieldOptionOutputFormat, "outputFormat"))
		if err != nil {
			return nil, err
		}
		return &channelvars.RenderOutputResponse{Output: output}, nil
	}
	if p.hostGroupManager == nil {
		return nil, fmt.Errorf("%s is required", channelvars.FieldAddressGroupConfig)
	}

	// 获取主机组ID
	hostGroupID := req.Values["dynamic.hostGroup"]
	if hostGroupID == "" {
		return nil, fmt.Errorf("dynamic.hostGroup is required")
	}

	// 获取输出格式
	outputFormat := req.Values["option.outputFormat"]
	if outputFormat == "" {
		return nil, fmt.Errorf("option.outputFormat is required")
	}

	// 查询主机列表
	hosts, err := p.hostGroupManager.GetHosts(ctx, hostGroupID)
	if err != nil {
		return nil, fmt.Errorf("get hosts: %w", err)
	}

	output, err := renderHostConfigs(hostsToAccessConfigs(hosts), outputFormat)
	if err != nil {
		return nil, err
	}

	return &channelvars.RenderOutputResponse{
		Output: output,
	}, nil
}

func RenderHostConfigsForFormat(hosts []Host, outputFormat string) (string, error) {
	return renderHostConfigs(hostsToAccessConfigs(hosts), outputFormat)
}

func hostsToAccessConfigs(hosts []Host) []hostAccessConfig {
	result := make([]hostAccessConfig, 0, len(hosts))
	for _, host := range hosts {
		result = append(result, hostAccessConfig{
			Host:     host.IP,
			Port:     int64(host.Port),
			Username: host.Username,
			Password: host.Password,
		})
	}
	return result
}

// ValidateValue 校验值
func (p *HostGroupProvider) ValidateValue(ctx context.Context, req *channelvars.ValidateValueRequest) error {
	// TODO: 实现值校验逻辑
	return nil
}

// HostGroupManager 主机组管理器接口
type HostGroupManager interface {
	// ListHostGroups 查询主机组列表
	ListHostGroups(ctx context.Context, projectID int64) ([]HostGroup, error)

	// GetHosts 获取主机列表
	GetHosts(ctx context.Context, hostGroupID string) ([]Host, error)
}

// HostGroup 主机组
type HostGroup struct {
	ID   int64
	Name string
	Code string
}

// Host 主机
type Host struct {
	ID       int64
	IP       string
	Port     int
	Username string
	Password string
}
