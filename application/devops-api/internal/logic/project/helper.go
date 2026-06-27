package project

import (
	"context"
	"strconv"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/projectservice"
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

func projectToType(in *projectservice.DevopsProject) types.DevopsProject {
	if in == nil {
		return types.DevopsProject{}
	}
	return types.DevopsProject{
		Id:                     in.Id,
		Name:                   in.Name,
		Code:                   in.Code,
		PortalProjectUuid:      in.PortalProjectUuid,
		Description:            in.Description,
		PipelineEngineType:     in.PipelineEngineType,
		DefaultEngineChannelId: in.DefaultEngineChannelId,
		Status:                 in.Status,
		ExtraConfig:            in.ExtraConfig,
		CreatedBy:              in.CreatedBy,
		UpdatedBy:              in.UpdatedBy,
		CreatedAt:              in.CreatedAt,
		UpdatedAt:              in.UpdatedAt,
	}
}

func projectMemberToType(in *projectservice.DevopsProjectMember) types.DevopsProjectMember {
	if in == nil {
		return types.DevopsProjectMember{}
	}
	return types.DevopsProjectMember{
		Id:        in.Id,
		ProjectId: in.ProjectId,
		UserId:    in.UserId,
		Username:  in.Username,
		Nickname:  in.Nickname,
		Role:      in.Role,
		Status:    in.Status,
		CreatedBy: in.CreatedBy,
		UpdatedBy: in.UpdatedBy,
		CreatedAt: in.CreatedAt,
		UpdatedAt: in.UpdatedAt,
	}
}

func projectMavenConfigToType(in *projectservice.DevopsProjectMavenConfig) types.DevopsProjectMavenConfig {
	if in == nil {
		return types.DevopsProjectMavenConfig{}
	}
	return types.DevopsProjectMavenConfig{
		Id:          in.Id,
		ProjectId:   in.ProjectId,
		Name:        in.Name,
		Code:        in.Code,
		Content:     in.Content,
		Description: in.Description,
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
		CreatedAt:   in.CreatedAt,
		UpdatedAt:   in.UpdatedAt,
	}
}

func configTypeToType(in *projectservice.DevopsConfigType) types.DevopsConfigType {
	if in == nil {
		return types.DevopsConfigType{}
	}
	children := make([]types.DevopsConfigType, 0, len(in.Children))
	for _, child := range in.Children {
		children = append(children, configTypeToType(child))
	}
	return types.DevopsConfigType{
		Id:          in.Id,
		ParentId:    in.ParentId,
		Name:        in.Name,
		Code:        in.Code,
		StorageType: in.StorageType,
		Description: in.Description,
		SortOrder:   in.SortOrder,
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
		CreatedAt:   in.CreatedAt,
		UpdatedAt:   in.UpdatedAt,
		Children:    children,
	}
}

func projectConfigToType(in *projectservice.DevopsProjectConfig) types.DevopsProjectConfig {
	if in == nil {
		return types.DevopsProjectConfig{}
	}
	return types.DevopsProjectConfig{
		Id:          in.Id,
		ProjectId:   in.ProjectId,
		TypeId:      in.TypeId,
		TypeCode:    in.TypeCode,
		TypeName:    in.TypeName,
		Name:        in.Name,
		Code:        in.Code,
		Content:     in.Content,
		Description: in.Description,
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
		CreatedAt:   in.CreatedAt,
		UpdatedAt:   in.UpdatedAt,
	}
}

func projectChannelBindingToType(in *projectservice.DevopsProjectChannelBinding) types.DevopsProjectChannelBinding {
	if in == nil {
		return types.DevopsProjectChannelBinding{}
	}
	return types.DevopsProjectChannelBinding{
		Id:                       in.Id,
		ProjectId:                in.ProjectId,
		ChannelId:                in.ChannelId,
		ChannelGroupCode:         in.ChannelGroupCode,
		ChannelName:              in.ChannelName,
		ChannelCode:              in.ChannelCode,
		ChannelType:              in.ChannelType,
		UsageScope:               in.UsageScope,
		IsDefault:                in.IsDefault,
		AllowUseGlobalCredential: in.AllowUseGlobalCredential,
		ProjectCredentialId:      in.ProjectCredentialId,
		BindingConfig:            in.BindingConfig,
		Status:                   in.Status,
		HealthStatus:             in.HealthStatus,
		LastCheckAt:              in.LastCheckAt,
		LastCheckMessage:         in.LastCheckMessage,
		Metadata:                 in.Metadata,
		ChannelConfig:            in.ChannelConfig,
		CreatedBy:                in.CreatedBy,
		UpdatedBy:                in.UpdatedBy,
		CreatedAt:                in.CreatedAt,
		UpdatedAt:                in.UpdatedAt,
	}
}

func tektonSecretBindingToType(in *projectservice.TektonSecretBindingOption) types.TektonSecretBindingOption {
	if in == nil {
		return types.TektonSecretBindingOption{}
	}
	return types.TektonSecretBindingOption{
		Id:                 in.Id,
		ProjectId:          in.ProjectId,
		ChannelId:          in.ChannelId,
		ChannelName:        in.ChannelName,
		ChannelCode:        in.ChannelCode,
		Namespace:          in.Namespace,
		TaskNamespace:      in.TaskNamespace,
		ServiceAccountName: in.ServiceAccountName,
		Status:             in.Status,
		HealthStatus:       in.HealthStatus,
	}
}

func tektonSecretToType(in *projectservice.TektonSecret) types.TektonSecret {
	if in == nil {
		return types.TektonSecret{}
	}
	return types.TektonSecret{
		Name:              in.Name,
		Type:              in.Type,
		Namespace:         in.Namespace,
		BindingId:         in.BindingId,
		ChannelName:       in.ChannelName,
		Keys:              in.Keys,
		Data:              in.Data,
		ManagedByKubeNova: in.ManagedByKubeNova,
		CreatedAt:         in.CreatedAt,
	}
}

func tektonPVCToType(in *projectservice.TektonPVC) types.TektonPVC {
	if in == nil {
		return types.TektonPVC{}
	}
	return types.TektonPVC{
		Name:             in.Name,
		Namespace:        in.Namespace,
		BindingId:        in.BindingId,
		ChannelName:      in.ChannelName,
		StorageClassName: in.StorageClassName,
		Storage:          in.Storage,
		AccessModes:      in.AccessModes,
		VolumeMode:       in.VolumeMode,
		Status:           in.Status,
		CreatedAt:        in.CreatedAt,
	}
}

func tektonResourceToType(in *projectservice.TektonResource) types.TektonResource {
	if in == nil {
		return types.TektonResource{}
	}
	return types.TektonResource{
		Name:             in.Name,
		Namespace:        in.Namespace,
		BindingId:        in.BindingId,
		ChannelName:      in.ChannelName,
		ResourceType:     in.ResourceType,
		Kind:             in.Kind,
		ApiVersion:       in.ApiVersion,
		Status:           in.Status,
		Phase:            in.Phase,
		Reason:           in.Reason,
		Message:          in.Message,
		Keys:             in.Keys,
		StorageClassName: in.StorageClassName,
		Storage:          in.Storage,
		AccessModes:      in.AccessModes,
		VolumeMode:       in.VolumeMode,
		Secrets:          in.Secrets,
		ImagePullSecrets: in.ImagePullSecrets,
		ContainerNames:   in.ContainerNames,
		CreatedAt:        in.CreatedAt,
		Yaml:             in.Yaml,
	}
}
