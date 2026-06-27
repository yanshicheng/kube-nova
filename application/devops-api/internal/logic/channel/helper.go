package channel

import (
	"context"
	"strconv"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/channelservice"
)

func currentUsername(ctx context.Context) string {
	username, ok := ctx.Value("username").(string)
	if !ok || username == "" {
		return "system"
	}
	return username
}

func currentUserID(ctx context.Context) uint64 {
	value := ctx.Value("userId")
	switch v := value.(type) {
	case uint64:
		return v
	case uint:
		return uint64(v)
	case int:
		if v > 0 {
			return uint64(v)
		}
	case int64:
		if v > 0 {
			return uint64(v)
		}
	case float64:
		if v > 0 {
			return uint64(v)
		}
	case string:
		id, _ := strconv.ParseUint(v, 10, 64)
		return id
	}
	return 0
}

func currentRoles(ctx context.Context) []string {
	value := ctx.Value("roles")
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		roles := make([]string, 0, len(v))
		for _, item := range v {
			if role, ok := item.(string); ok && role != "" {
				roles = append(roles, role)
			}
		}
		return roles
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return nil
}

func channelGroupToType(in *channelservice.DevopsChannelGroup) types.DevopsChannelGroup {
	if in == nil {
		return types.DevopsChannelGroup{}
	}
	return types.DevopsChannelGroup{
		Id:                  in.Id,
		Name:                in.Name,
		Code:                in.Code,
		Description:         in.Description,
		SortOrder:           in.SortOrder,
		IsSystem:            in.IsSystem,
		Status:              in.Status,
		CreatedBy:           in.CreatedBy,
		UpdatedBy:           in.UpdatedBy,
		CreatedAt:           in.CreatedAt,
		UpdatedAt:           in.UpdatedAt,
		GroupType:           in.GroupType,
		AllowedChannelTypes: in.AllowedChannelTypes,
		Icon:                in.Icon,
		IconColor:           in.IconColor,
	}
}

func channelToType(in *channelservice.DevopsChannel) types.DevopsChannel {
	if in == nil {
		return types.DevopsChannel{}
	}
	return types.DevopsChannel{
		Id:                 in.Id,
		GroupId:            in.GroupId,
		Name:               in.Name,
		Code:               in.Code,
		ChannelType:        in.ChannelType,
		Endpoint:           in.Endpoint,
		Description:        in.Description,
		GlobalCredentialId: in.GlobalCredentialId,
		CredentialId:       in.CredentialId,
		Config:             in.Config,
		Labels:             in.Labels,
		HealthStatus:       in.HealthStatus,
		LastCheckAt:        in.LastCheckAt,
		LastCheckMessage:   in.LastCheckMessage,
		Metadata:           in.Metadata,
		Status:             in.Status,
		IsSystem:           in.IsSystem,
		AuthType:           in.AuthType,
		Username:           in.Username,
		Password:           in.Password,
		Token:              in.Token,
		InsecureSkipTls:    in.InsecureSkipTls,
		Icon:               in.Icon,
		IconColor:          in.IconColor,
		CreatedBy:          in.CreatedBy,
		UpdatedBy:          in.UpdatedBy,
		CreatedAt:          in.CreatedAt,
		UpdatedAt:          in.UpdatedAt,
	}
}

func channelTypeToType(in *channelservice.DevopsChannelType) types.DevopsChannelType {
	if in == nil {
		return types.DevopsChannelType{}
	}
	return types.DevopsChannelType{
		Id:               in.Id,
		Code:             in.Code,
		Name:             in.Name,
		GroupCode:        in.GroupCode,
		CredentialTypes:  in.CredentialTypes,
		ConfigSchema:     in.ConfigSchema,
		MappingFields:    in.MappingFields,
		TestStrategy:     in.TestStrategy,
		Icon:             in.Icon,
		IconColor:        in.IconColor,
		ConnectionMode:   in.ConnectionMode,
		MetadataStrategy: in.MetadataStrategy,
		IsSystem:         in.IsSystem,
		Status:           in.Status,
		CreatedBy:        in.CreatedBy,
		UpdatedBy:        in.UpdatedBy,
		CreatedAt:        in.CreatedAt,
		UpdatedAt:        in.UpdatedAt,
	}
}

func credentialToType(in *channelservice.DevopsCredential) types.DevopsCredential {
	if in == nil {
		return types.DevopsCredential{}
	}
	return types.DevopsCredential{
		Id:                  in.Id,
		Name:                in.Name,
		Code:                in.Code,
		CredentialType:      in.CredentialType,
		Username:            in.Username,
		Password:            in.Password,
		Token:               in.Token,
		PrivateKey:          in.PrivateKey,
		Passphrase:          in.Passphrase,
		Kubeconfig:          in.Kubeconfig,
		SecretText:          in.SecretText,
		Certificate:         in.Certificate,
		JsonData:            in.JsonData,
		Description:         in.Description,
		Status:              in.Status,
		IsSystem:            in.IsSystem,
		Scope:               in.Scope,
		ProjectId:           in.ProjectId,
		ChannelGroupCode:    in.ChannelGroupCode,
		ChannelType:         in.ChannelType,
		JenkinsSyncStatus:   in.JenkinsSyncStatus,
		JenkinsSyncMessage:  in.JenkinsSyncMessage,
		JenkinsCredentialId: in.JenkinsCredentialId,
		JenkinsLastSyncAt:   in.JenkinsLastSyncAt,
		CreatedBy:           in.CreatedBy,
		UpdatedBy:           in.UpdatedBy,
		CreatedAt:           in.CreatedAt,
		UpdatedAt:           in.UpdatedAt,
	}
}

func credentialUsageToType(in *channelservice.CredentialUsage) types.DevopsCredentialUsage {
	if in == nil {
		return types.DevopsCredentialUsage{}
	}
	return types.DevopsCredentialUsage{
		ResourceType: in.ResourceType,
		ResourceName: in.ResourceName,
		ResourceCode: in.ResourceCode,
		ResourceId:   in.ResourceId,
		Relation:     in.Relation,
	}
}

func hostToType(in *channelservice.DevopsHost) types.DevopsHost {
	if in == nil {
		return types.DevopsHost{}
	}
	return types.DevopsHost{
		Id:               in.Id,
		Name:             in.Name,
		Ip:               in.Ip,
		Port:             in.Port,
		CredentialId:     in.CredentialId,
		Labels:           in.Labels,
		Description:      in.Description,
		HealthStatus:     in.HealthStatus,
		LastCheckAt:      in.LastCheckAt,
		LastCheckMessage: in.LastCheckMessage,
		Metadata:         in.Metadata,
		Status:           in.Status,
		CreatedBy:        in.CreatedBy,
		UpdatedBy:        in.UpdatedBy,
		CreatedAt:        in.CreatedAt,
		UpdatedAt:        in.UpdatedAt,
	}
}
