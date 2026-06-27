package pipelineconfigservicelogic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveChannelCredentialValueLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

var objectIDPattern = regexp.MustCompile(`^[a-fA-F0-9]{24}$`)

func NewResolveChannelCredentialValueLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveChannelCredentialValueLogic {
	return &ResolveChannelCredentialValueLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResolveChannelCredentialValueLogic) ResolveChannelCredentialValue(in *pb.ResolveChannelCredentialValueReq) (*pb.ResolveChannelCredentialValueResp, error) {
	if err := ensureProjectAccess(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("解析渠道凭证失败: %v", err)
		return nil, err
	}
	binding, channel, err := l.resolveCredentialSourceBinding(in)
	if err != nil {
		l.Errorf("解析渠道凭证来源失败: %v", err)
		return nil, err
	}
	credential, inlineSecret, err := l.resolveChannelCredential(in.ProjectId, binding, channel)
	if err != nil {
		l.Errorf("解析渠道绑定凭证失败: %v", err)
		return nil, err
	}
	credentialMode := normalizeChannelCredentialMode(in.CredentialMode, in.MappingField)
	if credentialMode == credentialModeJenkinsID {
		if credential == nil {
			l.Errorf("解析渠道凭证失败：生成 Jenkins credentialId 必须使用平台凭证")
			return nil, errorx.Msg("生成 Jenkins credentialId 必须使用平台凭证")
		}
		return l.resolveChannelJenkinsCredentialID(in, credential)
	}
	field := strings.TrimSpace(in.MappingField)
	if credentialMode == credentialModeJSONData || credentialMode == credentialModeJSONBase64 {
		field = credentialMode
	} else if !isChannelCredentialFieldMappingField(field) {
		l.Errorf("解析渠道凭证失败：渠道凭证映射字段不支持，mappingField: %s", field)
		return nil, errorx.Msg("渠道凭证映射字段不支持")
	}
	if credential != nil {
		secret, err := credential.Secret()
		if err != nil {
			l.Errorf("解密渠道凭证失败: %v", err)
			return nil, errorx.Msg("解密渠道凭证失败")
		}
		if !credentialSupportsField(credential.CredentialType, field) {
			l.Errorf("解析渠道凭证失败：渠道凭证不支持字段，credentialType: %s, mappingField: %s", credential.CredentialType, field)
			return nil, errorx.Msg("渠道凭证不支持字段 " + field)
		}
		value, ok := channelCredentialSecretField(secret, credential.CredentialType, field)
		if !ok {
			l.Errorf("解析渠道凭证失败：渠道凭证不支持字段，credentialType: %s, mappingField: %s", credential.CredentialType, field)
			return nil, errorx.Msg("渠道凭证不支持字段 " + field)
		}
		return &pb.ResolveChannelCredentialValueResp{Value: value, CredentialId: credential.ID.Hex()}, nil
	}
	value, ok := channelCredentialSecretField(inlineSecret, "", field)
	if !ok || strings.TrimSpace(value) == "" {
		l.Errorf("解析渠道凭证失败：渠道内联凭证不支持字段，mappingField: %s", field)
		return nil, errorx.Msg("渠道凭证不支持字段 " + field)
	}
	return &pb.ResolveChannelCredentialValueResp{Value: value}, nil
}

func credentialSupportsField(credentialType, field string) bool {
	switch strings.TrimSpace(field) {
	case "jsonData", "jsonBase64":
		return true
	}
	return isCredentialMappingField(credentialType, field)
}

func channelCredentialSecretField(secret model.CredentialSecret, credentialType, field string) (string, bool) {
	switch strings.TrimSpace(field) {
	case "jsonData":
		return channelCredentialJSON(secret, credentialType)
	case "jsonBase64":
		data, ok := channelCredentialJSON(secret, credentialType)
		if !ok {
			return "", false
		}
		return base64.StdEncoding.EncodeToString([]byte(data)), true
	default:
		return credentialSecretField(secret, field)
	}
}

func channelCredentialJSON(secret model.CredentialSecret, credentialType string) (string, bool) {
	if strings.TrimSpace(credentialType) == "json" {
		return secret.JsonData, strings.TrimSpace(secret.JsonData) != ""
	}
	fields := make(map[string]string)
	add := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			fields[key] = value
		}
	}
	switch strings.TrimSpace(credentialType) {
	case "username_password":
		add("username", secret.Username)
		add("password", secret.Password)
	case "token":
		add("token", secret.Token)
	case "secret_text":
		add("secretText", secret.SecretText)
	case "ssh_key":
		add("username", secret.Username)
		add("privateKey", secret.PrivateKey)
		add("passphrase", secret.Passphrase)
	case "kubeconfig":
		add("kubeconfig", secret.Kubeconfig)
	case "certificate":
		add("certificate", secret.Certificate)
	default:
		add("username", secret.Username)
		add("password", secret.Password)
		add("token", secret.Token)
		add("secretText", secret.SecretText)
		add("privateKey", secret.PrivateKey)
		add("passphrase", secret.Passphrase)
		add("kubeconfig", secret.Kubeconfig)
		add("certificate", secret.Certificate)
	}
	if len(fields) == 0 {
		return "", false
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func isLikelyObjectID(value string) bool {
	return objectIDPattern.MatchString(strings.TrimSpace(value))
}

func (l *ResolveChannelCredentialValueLogic) resolveCredentialSourceBinding(in *pb.ResolveChannelCredentialValueReq) (*model.DevopsProjectChannelBinding, *model.DevopsChannel, error) {
	bindingID := strings.TrimSpace(in.SourceChannelBindingId)
	sourceType := strings.TrimSpace(in.SourceParamType)
	sourceValue := strings.TrimSpace(in.SourceValue)
	if bindingID == "" && isLikelyObjectID(sourceValue) {
		switch sourceType {
		case channelvars.ParamChannelVariable, channelvars.ParamKubeNovaDeployConfig, channelvars.ParamKubernetesDeployConfig:
			bindingID = sourceValue
		}
	}
	if bindingID == "" && isDeployConfigCredentialSource(sourceType) && sourceValue != "" {
		bindingID = bindingIDFromDeployConfigValue(sourceValue)
	}
	if bindingID == "" && channelvars.IsChannelParamType(sourceType) && isLikelyObjectID(sourceValue) {
		bindingID = sourceValue
	}
	if bindingID != "" {
		return l.loadCredentialSourceBinding(in.ProjectId, bindingID)
	}
	if channelvars.IsChannelParamType(sourceType) && sourceValue != "" {
		return l.matchEndpointBinding(in.ProjectId, sourceType, sourceValue)
	}
	if isCodeRepoAddressCredentialSource(sourceType, in.MappingField) && sourceValue != "" {
		return l.matchGitRepositoryBinding(in.ProjectId, sourceValue)
	}
	if channelvars.IsCompositeAddressParamType(sourceType) && sourceValue != "" {
		return l.matchCompositeAddressBinding(in.ProjectId, sourceType, sourceValue)
	}
	l.Errorf("解析渠道凭证失败：渠道凭证参数缺少渠道来源，sourceType: %s", sourceType)
	return nil, nil, errorx.Msg("渠道凭证参数缺少渠道来源")
}

func (l *ResolveChannelCredentialValueLogic) matchEndpointBinding(projectID, sourceType, endpoint string) (*model.DevopsProjectChannelBinding, *model.DevopsChannel, error) {
	groupCode := channelvars.ChannelGroupCode(sourceType)
	channelType := channelvars.ChannelTypeFilter(sourceType)
	bindings, _, err := l.svcCtx.ProjectChannelModel.List(l.ctx, model.DevopsProjectChannelBindingListFilter{
		ProjectID:        projectID,
		ChannelGroupCode: groupCode,
		ChannelType:      channelType,
		Status:           1,
		Page:             1,
		PageSize:         200,
	})
	if err != nil {
		l.Errorf("查询渠道绑定失败: %v", err)
		return nil, nil, errorx.Msg("查询渠道绑定失败")
	}
	sourceEndpoint := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	var fallbackBinding *model.DevopsProjectChannelBinding
	var fallbackChannel *model.DevopsChannel
	fallbackCount := 0
	for _, binding := range bindings {
		if binding == nil {
			continue
		}
		channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				continue
			}
			l.Errorf("查询渠道失败: %v", err)
			return nil, nil, errorx.Msg("查询渠道失败")
		}
		if channel.Status != 1 {
			continue
		}
		if strings.EqualFold(sourceEndpoint, strings.TrimRight(strings.TrimSpace(channel.Endpoint), "/")) ||
			strings.EqualFold(sourceEndpoint, strings.TrimSpace(channel.Code)) ||
			strings.EqualFold(sourceEndpoint, strings.TrimSpace(channel.Name)) {
			return binding, channel, nil
		}
		fallbackBinding = binding
		fallbackChannel = channel
		fallbackCount++
	}
	if fallbackCount == 1 {
		return fallbackBinding, fallbackChannel, nil
	}
	if fallbackCount > 1 {
		l.Errorf("渠道实例无法匹配唯一渠道，sourceType: %s, endpoint: %s", sourceType, endpoint)
		return nil, nil, errorx.Msg("渠道实例无法匹配唯一渠道")
	}
	l.Errorf("当前项目未绑定对应渠道，sourceType: %s, projectId: %s", sourceType, projectID)
	return nil, nil, errorx.Msg("当前项目未绑定对应渠道")
}

func isCodeRepoAddressCredentialSource(sourceType, sourceMappingField string) bool {
	if strings.TrimSpace(sourceType) == channelvars.ParamGitRepositoryURL {
		return true
	}
	return strings.TrimSpace(sourceType) == channelvars.ParamChannelVariable &&
		strings.TrimSpace(sourceMappingField) == channelvars.FieldAddressProjectURL
}

func isDeployConfigCredentialSource(sourceType string) bool {
	switch strings.TrimSpace(sourceType) {
	case channelvars.ParamKubeNovaDeployConfig, channelvars.ParamKubernetesDeployConfig:
		return true
	default:
		return false
	}
}

func bindingIDFromDeployConfigValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(value), &data); err == nil {
		for _, key := range []string{"channelBindingId", "bindingId"} {
			if id := strings.TrimSpace(fmt.Sprint(data[key])); isLikelyObjectID(id) {
				return id
			}
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return ""
	}
	if err := json.Unmarshal(decoded, &data); err != nil {
		return ""
	}
	for _, key := range []string{"channelBindingId", "bindingId"} {
		if id := strings.TrimSpace(fmt.Sprint(data[key])); isLikelyObjectID(id) {
			return id
		}
	}
	return ""
}

func (l *ResolveChannelCredentialValueLogic) loadCredentialSourceBinding(projectID, bindingID string) (*model.DevopsProjectChannelBinding, *model.DevopsChannel, error) {
	binding, err := l.svcCtx.ProjectChannelModel.FindOne(l.ctx, bindingID)
	if err != nil {
		l.Errorf("查询渠道绑定失败: %v", err)
		return nil, nil, errorx.Msg("查询渠道绑定失败")
	}
	if binding.ProjectID != projectID {
		l.Errorf("渠道绑定不属于当前项目，bindingId: %s, projectId: %s", bindingID, projectID)
		return nil, nil, errorx.Msg("渠道绑定不属于当前项目")
	}
	if binding.Status != 1 {
		l.Errorf("渠道绑定已停用，bindingId: %s", bindingID)
		return nil, nil, errorx.Msg("渠道绑定已停用")
	}
	channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
	if err != nil {
		l.Errorf("查询渠道失败: %v", err)
		return nil, nil, errorx.Msg("查询渠道失败")
	}
	if channel.Status != 1 {
		l.Errorf("渠道已停用，channelId: %s", binding.ChannelID)
		return nil, nil, errorx.Msg("渠道已停用")
	}
	return binding, channel, nil
}

func (l *ResolveChannelCredentialValueLogic) matchGitRepositoryBinding(projectID, repositoryURL string) (*model.DevopsProjectChannelBinding, *model.DevopsChannel, error) {
	bindings, _, err := l.svcCtx.ProjectChannelModel.List(l.ctx, model.DevopsProjectChannelBindingListFilter{
		ProjectID:        projectID,
		ChannelGroupCode: channelvars.GroupCodeRepo,
		Status:           1,
		Page:             1,
		PageSize:         200,
	})
	if err != nil {
		l.Errorf("查询代码仓库渠道绑定失败: %v", err)
		return nil, nil, errorx.Msg("查询代码仓库渠道绑定失败")
	}
	var fallbackBinding *model.DevopsProjectChannelBinding
	var fallbackChannel *model.DevopsChannel
	fallbackCount := 0
	for _, binding := range bindings {
		if binding == nil || !isRepositoryChannelType(binding.ChannelType) {
			continue
		}
		channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				continue
			}
			l.Errorf("查询代码仓库渠道失败: %v", err)
			return nil, nil, errorx.Msg("查询代码仓库渠道失败")
		}
		if channel.Status != 1 {
			continue
		}
		if repositoryURLMatchesEndpoint(repositoryURL, channel.Endpoint) {
			return binding, channel, nil
		}
		fallbackBinding = binding
		fallbackChannel = channel
		fallbackCount++
	}
	if fallbackCount == 1 {
		return fallbackBinding, fallbackChannel, nil
	}
	if fallbackCount > 1 {
		l.Errorf("Git 仓库地址无法匹配唯一渠道实例，repositoryUrl: %s", repositoryURL)
		return nil, nil, errorx.Msg("Git 仓库地址无法匹配唯一渠道实例")
	}
	l.Errorf("当前项目未绑定代码仓库渠道，projectId: %s", projectID)
	return nil, nil, errorx.Msg("当前项目未绑定代码仓库渠道")
}

func (l *ResolveChannelCredentialValueLogic) matchCompositeAddressBinding(projectID, sourceType, sourceURL string) (*model.DevopsProjectChannelBinding, *model.DevopsChannel, error) {
	groupCode, channelTypes, ok := credentialAddressSourceGroup(sourceType)
	if !ok {
		l.Errorf("渠道凭证地址来源类型不支持，sourceType: %s", sourceType)
		return nil, nil, errorx.Msg("渠道凭证地址来源类型不支持")
	}
	channelType := ""
	if len(channelTypes) == 1 {
		channelType = channelTypes[0]
	}
	bindings, _, err := l.svcCtx.ProjectChannelModel.List(l.ctx, model.DevopsProjectChannelBindingListFilter{
		ProjectID:        projectID,
		ChannelGroupCode: groupCode,
		ChannelType:      channelType,
		Status:           1,
		Page:             1,
		PageSize:         200,
	})
	if err != nil {
		l.Errorf("查询渠道绑定失败: %v", err)
		return nil, nil, errorx.Msg("查询渠道绑定失败")
	}
	var fallbackBinding *model.DevopsProjectChannelBinding
	var fallbackChannel *model.DevopsChannel
	fallbackCount := 0
	for _, binding := range bindings {
		if binding == nil {
			continue
		}
		if !credentialAddressChannelTypeAllowed(binding.ChannelType, channelTypes) {
			continue
		}
		channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				continue
			}
			l.Errorf("查询渠道失败: %v", err)
			return nil, nil, errorx.Msg("查询渠道失败")
		}
		if channel.Status != 1 {
			continue
		}
		if credentialAddressMatchesEndpoint(groupCode, sourceURL, channel.Endpoint) {
			return binding, channel, nil
		}
		fallbackBinding = binding
		fallbackChannel = channel
		fallbackCount++
	}
	if fallbackCount == 1 {
		return fallbackBinding, fallbackChannel, nil
	}
	if fallbackCount > 1 {
		l.Errorf("渠道凭证地址无法匹配唯一渠道实例，sourceType: %s, sourceUrl: %s", sourceType, sourceURL)
		return nil, nil, errorx.Msg("渠道凭证地址无法匹配唯一渠道实例")
	}
	l.Errorf("当前项目未绑定对应渠道，sourceType: %s, projectId: %s", sourceType, projectID)
	return nil, nil, errorx.Msg("当前项目未绑定对应渠道")
}

func credentialAddressMatchesEndpoint(groupCode, sourceURL, endpoint string) bool {
	if strings.TrimSpace(groupCode) == channelvars.GroupDeployTarget && hostAddressMatchesEndpoint(sourceURL, endpoint) {
		return true
	}
	if strings.TrimSpace(groupCode) == channelvars.GroupImageRepo {
		return imageRegistryURLMatchesEndpoint(sourceURL, endpoint)
	}
	return repositoryURLMatchesEndpoint(sourceURL, endpoint)
}

func hostAddressMatchesEndpoint(sourceURL, endpoint string) bool {
	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" || strings.TrimSpace(endpoint) == "" {
		return false
	}
	var values []string
	decoded, err := base64.StdEncoding.DecodeString(sourceURL)
	if err == nil && strings.TrimSpace(string(decoded)) != "" {
		sourceURL = strings.TrimSpace(string(decoded))
	}
	var single struct {
		Host string `json:"host"`
		Port int64  `json:"port"`
	}
	if err := json.Unmarshal([]byte(sourceURL), &single); err == nil && strings.TrimSpace(single.Host) != "" {
		values = append(values, single.Host)
		if single.Port > 0 {
			values = append(values, fmt.Sprintf("%s:%d", single.Host, single.Port))
		}
	}
	var group []struct {
		Host string `json:"host"`
		Port int64  `json:"port"`
	}
	if err := json.Unmarshal([]byte(sourceURL), &group); err == nil {
		for _, item := range group {
			if strings.TrimSpace(item.Host) == "" {
				continue
			}
			values = append(values, item.Host)
			if item.Port > 0 {
				values = append(values, fmt.Sprintf("%s:%d", item.Host, item.Port))
			}
		}
	}
	if strings.HasPrefix(sourceURL, "[all]") {
		for _, line := range strings.Split(sourceURL, "\n") {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) == 0 || strings.HasPrefix(fields[0], "[") {
				continue
			}
			values = append(values, fields[0])
			for _, field := range fields[1:] {
				if key, value, ok := strings.Cut(field, "="); ok && (key == "ansible_ssh_host" || key == "ansible_host") {
					values = append(values, value)
				}
			}
		}
	}
	for _, value := range values {
		if endpointHostValueMatches(value, endpoint) {
			return true
		}
	}
	return false
}

func endpointHostValueMatches(value, endpoint string) bool {
	sourceHost, sourcePort := endpointHostAndPort(value)
	endpointHost, endpointPort := endpointHostAndPort(endpoint)
	if sourceHost == "" || endpointHost == "" {
		return false
	}
	if !strings.EqualFold(sourceHost, endpointHost) {
		return false
	}
	return sourcePort == "" || endpointPort == "" || sourcePort == endpointPort
}

func endpointHostAndPort(value string) (string, string) {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return "", ""
	}
	if !strings.Contains(value, "://") && !strings.Contains(value, "@") {
		value = "https://" + value
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		return strings.Trim(parsed.Hostname(), "[]"), parsed.Port()
	}
	host, _ := endpointHostAndPath(value)
	return splitAddressHost(host)
}

func credentialAddressSourceGroup(sourceType string) (string, []string, bool) {
	switch strings.TrimSpace(sourceType) {
	case channelvars.ParamGitRepositoryURL:
		return channelvars.GroupCodeRepo, []string{"gitlab", "github", "gitee", "svn"}, true
	case channelvars.ParamHarborProjectURL, channelvars.ParamHarborImageURL, channelvars.ParamHarborImageTagURL:
		return channelvars.GroupImageRepo, []string{"harbor"}, true
	case channelvars.ParamRegistryRepositoryURL, channelvars.ParamRegistryImageURL, channelvars.ParamRegistryImageTagURL:
		return channelvars.GroupImageRepo, []string{"registry", "aliyun_registry"}, true
	case channelvars.ParamNexusRepositoryURL, channelvars.ParamNexusArtifactURL, channelvars.ParamNexusArtifactVersionURL:
		return channelvars.GroupArtifactRepo, []string{"nexus"}, true
	case channelvars.ParamJfrogRepositoryURL, channelvars.ParamJfrogArtifactURL, channelvars.ParamJfrogArtifactVersionURL:
		return channelvars.GroupArtifactRepo, []string{"jfrog"}, true
	case channelvars.ParamSonarAddress, channelvars.ParamSonarProjectURL, channelvars.ParamSonarProjectKeyURL:
		return channelvars.GroupCodeScan, []string{"sonarqube"}, true
	case channelvars.ParamKubeNovaDeployConfig:
		return channelvars.GroupDeployTarget, []string{"kube-nova"}, true
	case channelvars.ParamKubernetesDeployConfig:
		return channelvars.GroupDeployTarget, []string{"kubernetes"}, true
	case channelvars.ParamHostConfig:
		return channelvars.GroupDeployTarget, []string{"host"}, true
	case channelvars.ParamHostGroupConfig:
		return channelvars.GroupDeployTarget, []string{"host_group"}, true
	default:
		return "", nil, false
	}
}

func credentialAddressChannelTypeAllowed(channelType string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	channelType = strings.TrimSpace(channelType)
	for _, item := range allowed {
		if channelType == strings.TrimSpace(item) {
			return true
		}
	}
	return false
}

func (l *ResolveChannelCredentialValueLogic) resolveChannelCredential(projectID string, binding *model.DevopsProjectChannelBinding, channel *model.DevopsChannel) (*model.DevopsCredential, model.CredentialSecret, error) {
	credentialID := strings.TrimSpace(binding.ProjectCredentialID)
	if credentialID == "" && binding.AllowUseGlobalCredential {
		credentialID = strings.TrimSpace(channel.CredentialID)
		if credentialID == "" {
			credentialID = strings.TrimSpace(channel.GlobalCredentialID)
		}
	}
	if credentialID == "" {
		if !binding.AllowUseGlobalCredential {
			l.Errorf("渠道绑定未配置项目级凭据，且不允许使用渠道全局凭据，bindingId: %s, channelId: %s", binding.ID.Hex(), channel.ID.Hex())
			return nil, model.CredentialSecret{}, errorx.Msg("未配置项目级凭据，且当前绑定不允许使用渠道全局凭据")
		}
		if strings.TrimSpace(channel.Username) != "" || strings.TrimSpace(channel.Password) != "" || strings.TrimSpace(channel.Token) != "" {
			return nil, model.CredentialSecret{
				Username: channel.Username,
				Password: channel.Password,
				Token:    channel.Token,
			}, nil
		}
		l.Errorf("渠道未配置凭证，bindingId: %s, channelId: %s", binding.ID.Hex(), channel.ID.Hex())
		return nil, model.CredentialSecret{}, errorx.Msg("渠道未配置凭证")
	}
	credential, err := l.svcCtx.CredentialModel.FindOne(l.ctx, credentialID)
	if err != nil {
		l.Errorf("查询渠道凭证失败: %v", err)
		return nil, model.CredentialSecret{}, errorx.Msg("查询渠道凭证失败")
	}
	if credential.Status != 1 {
		l.Errorf("渠道凭证已停用，credentialId: %s", credentialID)
		return nil, model.CredentialSecret{}, errorx.Msg("渠道凭证已停用")
	}
	scope := normalizeCredentialScope(credential.Scope, credential.ProjectID, credential.IsSystem)
	if scope == "project" && credential.ProjectID != projectID {
		l.Errorf("项目渠道凭据必须属于当前项目，credentialId: %s, projectId: %s", credentialID, projectID)
		return nil, model.CredentialSecret{}, errorx.Msg("项目渠道凭据必须属于当前项目")
	}
	if scope != "project" && !binding.AllowUseGlobalCredential {
		l.Errorf("渠道绑定不允许使用全局凭据，credentialId: %s", credentialID)
		return nil, model.CredentialSecret{}, errorx.Msg("渠道绑定不允许使用全局凭据")
	}
	if strings.TrimSpace(binding.ProjectCredentialID) != "" {
		if credential.ProjectID != projectID || normalizeCredentialScope(credential.Scope, credential.ProjectID, credential.IsSystem) != "project" {
			l.Errorf("项目渠道凭据必须属于当前项目，credentialId: %s, projectId: %s", credentialID, projectID)
			return nil, model.CredentialSecret{}, errorx.Msg("项目渠道凭据必须属于当前项目")
		}
	}
	return credential, model.CredentialSecret{}, nil
}

func (l *ResolveChannelCredentialValueLogic) resolveChannelJenkinsCredentialID(in *pb.ResolveChannelCredentialValueReq, credential *model.DevopsCredential) (*pb.ResolveChannelCredentialValueResp, error) {
	if !isJenkinsCredentialIDSupportedType(credential.CredentialType) {
		l.Errorf("渠道凭证不支持 Jenkins credentialId 模式，credentialType: %s", credential.CredentialType)
		return nil, errorx.Msg("该凭证类型不支持 Jenkins credentialId 模式")
	}
	buildChannelBindingID := strings.TrimSpace(in.BuildChannelBindingId)
	if buildChannelBindingID == "" {
		l.Errorf("生成 Jenkins credentialId 必须选择构建渠道")
		return nil, errorx.Msg("生成 Jenkins credentialId 必须选择构建渠道")
	}
	credentialID := credential.ID.Hex()
	if !forceRefreshChannelJenkinsCredential(in) {
		if value, ok, err := latestJenkinsCredentialID(l.ctx, l.svcCtx, in.ProjectId, buildChannelBindingID, credentialID); err != nil {
			l.Errorf("查询 Jenkins 凭证同步记录失败: %v", err)
			return nil, errorx.Msg("查询 Jenkins 凭证同步记录失败")
		} else if ok {
			return &pb.ResolveChannelCredentialValueResp{
				Value:               value,
				CredentialId:        credentialID,
				JenkinsCredentialId: value,
			}, nil
		}
	}
	runtime, err := buildJenkinsSyncRuntime(l.ctx, l.svcCtx, in.ProjectId, buildChannelBindingID)
	if err != nil {
		l.Errorf("构建 Jenkins 同步运行时失败: %v", err)
		return nil, err
	}
	mapping, err := syncOneResolvedJenkinsCredential(l.ctx, l.svcCtx, runtime, credential, strings.TrimSpace(fmt.Sprint(in.CurrentUserId)))
	if err != nil {
		l.Errorf("同步 Jenkins 渠道凭证失败: %v", err)
		return nil, err
	}
	if mapping == nil || strings.TrimSpace(mapping.JenkinsCredentialId) == "" {
		l.Errorf("Jenkins 渠道凭证同步结果为空，credentialId: %s", credentialID)
		return nil, errorx.Msg("Jenkins 凭证同步结果为空")
	}
	jenkinsCredentialID := strings.TrimSpace(mapping.JenkinsCredentialId)
	return &pb.ResolveChannelCredentialValueResp{
		Value:               jenkinsCredentialID,
		CredentialId:        credentialID,
		JenkinsCredentialId: jenkinsCredentialID,
	}, nil
}

func forceRefreshChannelJenkinsCredential(in *pb.ResolveChannelCredentialValueReq) bool {
	if in == nil {
		return false
	}
	return strings.TrimSpace(in.SourceParamType) == channelvars.ParamKubernetesDeployConfig
}
