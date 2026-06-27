package pipelineconfigservicelogic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars/providers"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveHostGroupTargetsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type hostGroupBindingConfig struct {
	HostID  string   `json:"hostId"`
	HostIDs []string `json:"hostIds"`
}

type hostGroupTarget struct {
	Host     string `json:"host"`
	Port     int64  `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewResolveHostGroupTargetsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveHostGroupTargetsLogic {
	return &ResolveHostGroupTargetsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResolveHostGroupTargetsLogic) ResolveHostGroupTargets(in *pb.ResolveHostGroupTargetsReq) (*pb.ResolveHostGroupTargetsResp, error) {
	if strings.TrimSpace(in.ProjectId) == "" {
		l.Errorf("解析主机组失败: 项目不能为空")
		return nil, errorx.Msg("项目不能为空")
	}
	if strings.TrimSpace(in.HostGroupBindingId) == "" {
		l.Errorf("解析主机组失败: 主机组不能为空")
		return nil, errorx.Msg("主机组不能为空")
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("解析主机组失败: %v", err)
		return nil, err
	}
	renderMode := normalizeHostGroupRenderMode(in.RenderMode)
	binding, channel, err := l.loadHostGroupBinding(in.ProjectId, in.HostGroupBindingId)
	if err != nil {
		l.Errorf("解析主机组失败: %v", err)
		return nil, err
	}
	hostIDs, err := hostIDsFromConfig(binding.BindingConfig, channel.Config)
	if err != nil {
		l.Errorf("解析主机组失败: %v", err)
		return nil, err
	}
	targets, order, err := l.resolveHostTargets(hostIDs)
	if err != nil {
		l.Errorf("解析主机组失败: %v", err)
		return nil, err
	}
	value, err := renderHostGroupTargets(targets, order, renderMode)
	if err != nil {
		l.Errorf("解析主机组失败: %v", err)
		return nil, err
	}

	return &pb.ResolveHostGroupTargetsResp{Value: value, RenderMode: renderMode}, nil
}

func normalizeHostGroupRenderMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "normal":
		return "normal"
	case "json":
		return "json"
	case "inventory", "ansible":
		return "inventory"
	case "jsonbase64":
		return "jsonBase64"
	case "ansiblebase64":
		return "ansibleBase64"
	default:
		return strings.TrimSpace(value)
	}
}

func isSupportedHostGroupRenderMode(value string) bool {
	switch normalizeHostGroupRenderMode(value) {
	case "normal", "json", "jsonBase64", "inventory", "ansibleBase64":
		return true
	default:
		return false
	}
}

func (l *ResolveHostGroupTargetsLogic) loadHostGroupBinding(projectID, bindingID string) (*model.DevopsProjectChannelBinding, *model.DevopsChannel, error) {
	binding, err := l.svcCtx.ProjectChannelModel.FindOne(l.ctx, bindingID)
	if err != nil {
		return nil, nil, err
	}
	if binding.ProjectID != projectID {
		return nil, nil, errorx.Msg("主机组不属于当前项目")
	}
	if binding.Status != 1 {
		return nil, nil, errorx.Msg("主机组已停用")
	}
	if binding.ChannelGroupCode != channelvars.GroupDeployTarget && binding.UsageScope != channelvars.GroupDeployTarget {
		return nil, nil, errorx.Msg("请选择部署渠道主机组")
	}
	if binding.ChannelType != "host_group" {
		return nil, nil, errorx.Msg("请选择主机组渠道")
	}
	channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
	if err != nil {
		return nil, nil, err
	}
	if channel.Status != 1 {
		return nil, nil, errorx.Msg("主机组渠道已停用")
	}
	if channel.ChannelType != "host_group" {
		return nil, nil, errorx.Msg("请选择主机组渠道")
	}
	return binding, channel, nil
}

func hostIDsFromConfig(bindingConfig, channelConfig string) ([]string, error) {
	configText := strings.TrimSpace(bindingConfig)
	if configText == "" || configText == "{}" {
		configText = strings.TrimSpace(channelConfig)
	}
	var cfg hostGroupBindingConfig
	if err := json.Unmarshal([]byte(configText), &cfg); err != nil {
		return nil, errorx.Msg("主机组配置不是有效 JSON")
	}
	if len(cfg.HostIDs) == 0 && strings.TrimSpace(cfg.HostID) != "" {
		cfg.HostIDs = []string{strings.TrimSpace(cfg.HostID)}
	}
	result := make([]string, 0, len(cfg.HostIDs))
	seen := make(map[string]struct{}, len(cfg.HostIDs))
	for _, item := range cfg.HostIDs {
		hostID := strings.TrimSpace(item)
		if hostID == "" {
			continue
		}
		if _, ok := seen[hostID]; ok {
			continue
		}
		seen[hostID] = struct{}{}
		result = append(result, hostID)
	}
	if len(result) == 0 {
		return nil, errorx.Msg("主机组未选择主机资产")
	}
	return result, nil
}

func (l *ResolveHostGroupTargetsLogic) resolveHostTargets(hostIDs []string) (map[string]hostGroupTarget, []string, error) {
	targets := make(map[string]hostGroupTarget, len(hostIDs))
	order := make([]string, 0, len(hostIDs))
	for _, hostID := range hostIDs {
		host, err := l.svcCtx.HostModel.FindOne(l.ctx, hostID)
		if err != nil {
			return nil, nil, err
		}
		if host.Status != 1 {
			return nil, nil, errorx.Msg("主机资产已停用：" + host.Name)
		}
		credential, err := l.svcCtx.CredentialModel.FindOne(l.ctx, host.CredentialID)
		if err != nil {
			return nil, nil, err
		}
		if credential.Status != 1 {
			return nil, nil, errorx.Msg("主机凭据已停用：" + host.Name)
		}
		if credential.CredentialType != "username_password" {
			return nil, nil, errorx.Msg("主机组仅支持用户名密码凭据：" + host.Name)
		}
		secret, err := credential.Secret()
		if err != nil {
			return nil, nil, err
		}
		if strings.TrimSpace(secret.Username) == "" || strings.TrimSpace(secret.Password) == "" {
			return nil, nil, errorx.Msg("主机用户名或密码为空：" + host.Name)
		}
		name := strings.TrimSpace(host.Name)
		if name == "" {
			name = strings.TrimSpace(host.IP)
		}
		port := host.Port
		if port == 0 {
			port = 22
		}
		targets[name] = hostGroupTarget{
			Host:     host.IP,
			Port:     port,
			Username: secret.Username,
			Password: secret.Password,
		}
		order = append(order, name)
	}
	return targets, order, nil
}

func renderHostGroupTargets(targets map[string]hostGroupTarget, order []string, renderMode string) (string, error) {
	hosts := make([]providers.Host, 0, len(order))
	for _, name := range order {
		target := targets[name]
		hosts = append(hosts, providers.Host{
			IP:       target.Host,
			Port:     int(target.Port),
			Username: target.Username,
			Password: target.Password,
		})
	}
	switch normalizeHostGroupRenderMode(renderMode) {
	case "normal", "json":
		return providers.RenderHostConfigsForFormat(hosts, "json")
	case "jsonBase64":
		return providers.RenderHostConfigsForFormat(hosts, "jsonBase64")
	case "inventory", "ansible":
		return providers.RenderHostConfigsForFormat(hosts, "ansible")
	case "ansibleBase64":
		return providers.RenderHostConfigsForFormat(hosts, "ansibleBase64")
	default:
		return "", fmt.Errorf("unsupported render mode: %s", renderMode)
	}
}
