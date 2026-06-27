package projectservicelogic

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/tektonsync"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

var baseProjectChannelCredentialTypes = []string{"username_password", "token"}
var projectMavenConfigCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func projectToPb(in *model.DevopsProject) *pb.DevopsProject {
	if in == nil {
		return nil
	}
	return &pb.DevopsProject{
		Id:                     in.ID.Hex(),
		Name:                   in.Name,
		Code:                   in.Code,
		Description:            in.Description,
		PipelineEngineType:     in.PipelineEngineType,
		DefaultEngineChannelId: in.DefaultEngineChannelID,
		Status:                 in.Status,
		ExtraConfig:            in.ExtraConfig,
		CreatedBy:              in.CreatedBy,
		UpdatedBy:              in.UpdatedBy,
		CreatedAt:              in.CreateAt.Unix(),
		UpdatedAt:              in.UpdateAt.Unix(),
	}
}

func projectChannelBindingToPb(in *model.DevopsProjectChannelBinding) *pb.DevopsProjectChannelBinding {
	if in == nil {
		return nil
	}
	return &pb.DevopsProjectChannelBinding{
		Id:                       in.ID.Hex(),
		ProjectId:                in.ProjectID,
		ChannelId:                in.ChannelID,
		ChannelGroupCode:         in.ChannelGroupCode,
		ChannelName:              in.ChannelName,
		ChannelCode:              in.ChannelCode,
		ChannelType:              in.ChannelType,
		UsageScope:               in.UsageScope,
		IsDefault:                in.IsDefault,
		AllowUseGlobalCredential: in.AllowUseGlobalCredential,
		ProjectCredentialId:      in.ProjectCredentialID,
		BindingConfig:            in.BindingConfig,
		Status:                   in.Status,
		HealthStatus:             in.HealthStatus,
		LastCheckAt:              in.LastCheckAt,
		LastCheckMessage:         in.LastCheckMessage,
		Metadata:                 normalizeProjectMetadataJSON(in.Metadata),
		CreatedBy:                in.CreatedBy,
		UpdatedBy:                in.UpdatedBy,
		CreatedAt:                in.CreateAt.Unix(),
		UpdatedAt:                in.UpdateAt.Unix(),
	}
}

func normalizeProjectMetadataJSON(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return raw
	}
	if version, ok := data["version"].(string); ok && strings.TrimSpace(version) != "" {
		if shouldDropProjectAPIVersionAlias(data, version) {
			delete(data, "version")
			return marshalProjectMetadata(data, raw)
		}
		return raw
	}
	for _, key := range []string{"productVersion", "engineVersion", "schemaVersion", "benchmarkProfile"} {
		if value, ok := data[key].(string); ok && strings.TrimSpace(value) != "" {
			data["version"] = strings.TrimSpace(value)
			return marshalProjectMetadata(data, raw)
		}
	}
	if shouldUseProjectAPIVersionAsVersion(data) {
		if value, ok := data["apiVersion"].(string); ok && strings.TrimSpace(value) != "" {
			data["version"] = strings.TrimSpace(value)
			return marshalProjectMetadata(data, raw)
		}
	}
	return raw
}

func marshalProjectMetadata(data map[string]any, fallback string) string {
	normalized, err := json.Marshal(data)
	if err != nil {
		return fallback
	}
	return string(normalized)
}

func shouldDropProjectAPIVersionAlias(data map[string]any, version string) bool {
	productName, _ := data["productName"].(string)
	productVersion, _ := data["productVersion"].(string)
	apiVersion, _ := data["apiVersion"].(string)
	return strings.EqualFold(strings.TrimSpace(productName), "GitLab") &&
		strings.TrimSpace(productVersion) == "" &&
		strings.TrimSpace(apiVersion) != "" &&
		strings.TrimSpace(version) == strings.TrimSpace(apiVersion)
}

func shouldUseProjectAPIVersionAsVersion(data map[string]any) bool {
	productName, _ := data["productName"].(string)
	switch strings.ToLower(strings.TrimSpace(productName)) {
	case "docker registry", "github", "gitee", "tekton":
		return true
	default:
		return false
	}
}

func projectMemberToPb(in *model.DevopsProjectMember) *pb.DevopsProjectMember {
	if in == nil {
		return nil
	}
	return &pb.DevopsProjectMember{
		Id:        in.ID.Hex(),
		ProjectId: in.ProjectID,
		UserId:    in.UserID,
		Username:  in.Username,
		Nickname:  in.Nickname,
		Role:      in.Role,
		Status:    in.Status,
		CreatedBy: in.CreatedBy,
		UpdatedBy: in.UpdatedBy,
		CreatedAt: in.CreateAt.Unix(),
		UpdatedAt: in.UpdateAt.Unix(),
	}
}

func projectMavenConfigToPb(in *model.DevopsProjectConfig) *pb.DevopsProjectMavenConfig {
	if in == nil {
		return nil
	}
	return &pb.DevopsProjectMavenConfig{
		Id:          in.ID.Hex(),
		ProjectId:   in.ProjectID,
		Name:        in.Name,
		Code:        in.Code,
		Content:     in.Content,
		Description: in.Description,
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
		CreatedAt:   in.CreateAt.Unix(),
		UpdatedAt:   in.UpdateAt.Unix(),
	}
}

func configTypeToPb(in *model.DevopsConfigType) *pb.DevopsConfigType {
	if in == nil {
		return nil
	}
	return &pb.DevopsConfigType{
		Id:          in.ID.Hex(),
		ParentId:    in.ParentID,
		Name:        in.Name,
		Code:        in.Code,
		StorageType: normalizeConfigStorageType(in.StorageType),
		Description: in.Description,
		SortOrder:   in.SortOrder,
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
		CreatedAt:   in.CreateAt.Unix(),
		UpdatedAt:   in.UpdateAt.Unix(),
	}
}

func configTypeTreeToPb(items []*model.DevopsConfigType) []*pb.DevopsConfigType {
	nodes := make(map[string]*pb.DevopsConfigType, len(items))
	roots := make([]*pb.DevopsConfigType, 0)
	for _, item := range items {
		node := configTypeToPb(item)
		if node == nil {
			continue
		}
		nodes[node.Id] = node
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		node := nodes[item.ID.Hex()]
		if node == nil {
			continue
		}
		if item.ParentID == "" {
			roots = append(roots, node)
			continue
		}
		parent := nodes[item.ParentID]
		if parent == nil {
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}
	return roots
}

func projectConfigToPb(in *model.DevopsProjectConfig) *pb.DevopsProjectConfig {
	if in == nil {
		return nil
	}
	return &pb.DevopsProjectConfig{
		Id:          in.ID.Hex(),
		ProjectId:   in.ProjectID,
		TypeId:      in.TypeID,
		TypeCode:    in.TypeCode,
		TypeName:    in.TypeName,
		Name:        in.Name,
		Code:        in.Code,
		Content:     in.Content,
		Description: in.Description,
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
		CreatedAt:   in.CreateAt.Unix(),
		UpdatedAt:   in.UpdateAt.Unix(),
	}
}

func isSuperAdminRole(roles []string) bool {
	for _, role := range roles {
		if strings.EqualFold(strings.TrimSpace(role), "SUPER_ADMIN") {
			return true
		}
	}
	return false
}

func memberProjectScope(ctx context.Context, svcCtx *svc.ServiceContext, userID uint64, roles []string) ([]string, bool, error) {
	if isSuperAdminRole(roles) {
		return nil, false, nil
	}
	ids, err := svcCtx.ProjectMemberModel.ListProjectIDsByUser(ctx, userID)
	if err != nil {
		logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
		return nil, false, err
	}
	return ids, true, nil
}

func ensureProjectAccess(ctx context.Context, svcCtx *svc.ServiceContext, projectID string, userID uint64, roles []string) error {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		logx.Errorf("请选择 DevOps 项目")
		return errorx.Msg("请选择 DevOps 项目")
	}
	if _, err := svcCtx.ProjectModel.FindOne(ctx, projectID); err != nil {
		logx.Errorf("查询 DevOps 项目失败: %v", err)
		return err
	}
	if isSuperAdminRole(roles) {
		return nil
	}
	ids, err := svcCtx.ProjectMemberModel.ListProjectIDsByUser(ctx, userID)
	if err != nil {
		logx.Errorf("校验项目权限失败: %v", err)
		return err
	}
	for _, id := range ids {
		if id == projectID {
			return nil
		}
	}
	logx.Errorf("无权访问当前 DevOps 项目，projectId: %s", projectID)
	return errorx.Msg("无权访问当前 DevOps 项目")
}

func validateMavenConfigBase(ctx context.Context, svcCtx *svc.ServiceContext, projectID, name, code, content, excludeID string) error {
	return validateProjectConfigBase(ctx, svcCtx, projectID, model.DefaultMavenSettingsTypeCode, name, code, content, excludeID, "Maven 配置")
}

func validateConfigTypeBase(ctx context.Context, svcCtx *svc.ServiceContext, parentID, name, code, excludeID string) error {
	if strings.TrimSpace(name) == "" {
		logx.Errorf("配置类型名称不能为空")
		return errorx.Msg("配置类型名称不能为空")
	}
	if strings.TrimSpace(code) == "" {
		logx.Errorf("配置类型编码不能为空")
		return errorx.Msg("配置类型编码不能为空")
	}
	if !projectMavenConfigCodePattern.MatchString(strings.TrimSpace(code)) {
		logx.Errorf("配置类型编码格式不正确")
		return errorx.Msg("配置类型编码只能包含字母、数字、下划线和中划线")
	}
	if parentID != "" {
		if _, err := svcCtx.ConfigTypeModel.FindOne(ctx, parentID); err != nil {
			logx.Errorf("父级配置类型不存在: %v", err)
			return err
		}
	}
	total, err := svcCtx.ConfigTypeModel.CountByCode(ctx, strings.TrimSpace(code), excludeID)
	if err != nil {
		logx.Errorf("校验配置类型编码失败: %v", err)
		return err
	}
	if total > 0 {
		logx.Errorf("配置类型编码已存在")
		return errorx.Msg("配置类型编码已存在")
	}
	total, err = svcCtx.ConfigTypeModel.CountByParentName(ctx, parentID, strings.TrimSpace(name), excludeID)
	if err != nil {
		logx.Errorf("校验配置类型名称失败: %v", err)
		return err
	}
	if total > 0 {
		logx.Errorf("同级配置类型名称已存在")
		return errorx.Msg("同级配置类型名称已存在")
	}
	return nil
}

func normalizeConfigStorageType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "json", "xml", "yml", "text", "config":
		return value
	default:
		return "text"
	}
}

func resolveConfigType(ctx context.Context, svcCtx *svc.ServiceContext, typeID, typeCode string) (*model.DevopsConfigType, error) {
	typeID = strings.TrimSpace(typeID)
	typeCode = strings.TrimSpace(typeCode)
	if typeID != "" {
		configType, err := svcCtx.ConfigTypeModel.FindOne(ctx, typeID)
		if err != nil {
			logx.Errorf("查询配置类型失败: %v", err)
			return nil, err
		}
		return configType, nil
	}
	if typeCode != "" {
		configType, err := svcCtx.ConfigTypeModel.FindOneByCode(ctx, typeCode)
		if err != nil {
			logx.Errorf("查询配置类型失败: %v", err)
			return nil, err
		}
		return configType, nil
	}
	logx.Errorf("配置类型不能为空")
	return nil, errorx.Msg("配置类型不能为空")
}

func validateProjectConfigBase(ctx context.Context, svcCtx *svc.ServiceContext, projectID, typeCode, name, code, content, excludeID, label string) error {
	if strings.TrimSpace(name) == "" {
		logx.Errorf("%s名称不能为空", label)
		return errorx.Msg(label + "名称不能为空")
	}
	if strings.TrimSpace(code) == "" {
		logx.Errorf("%s编码不能为空", label)
		return errorx.Msg(label + "编码不能为空")
	}
	if !projectMavenConfigCodePattern.MatchString(strings.TrimSpace(code)) {
		logx.Errorf("%s编码格式不正确", label)
		return errorx.Msg(label + "编码只能包含字母、数字、下划线和中划线")
	}
	if strings.TrimSpace(content) == "" {
		logx.Errorf("%s内容不能为空", label)
		return errorx.Msg(label + "内容不能为空")
	}
	total, err := svcCtx.ProjectConfigModel.CountByProjectTypeCode(ctx, projectID, strings.TrimSpace(typeCode), strings.TrimSpace(code), excludeID)
	if err != nil {
		logx.Errorf("校验%s编码失败: %v", label, err)
		return err
	}
	if total > 0 {
		logx.Errorf("%s编码已存在", label)
		return errorx.Msg(label + "编码已存在")
	}
	total, err = svcCtx.ProjectConfigModel.CountByProjectTypeName(ctx, projectID, strings.TrimSpace(typeCode), strings.TrimSpace(name), excludeID)
	if err != nil {
		logx.Errorf("校验%s名称失败: %v", label, err)
		return err
	}
	if total > 0 {
		logx.Errorf("%s名称已存在", label)
		return errorx.Msg(label + "名称已存在")
	}
	return nil
}

func ensureConfigTypeDeletable(ctx context.Context, svcCtx *svc.ServiceContext, configType *model.DevopsConfigType) error {
	if configType == nil {
		return model.ErrNotFound
	}
	if strings.TrimSpace(configType.Code) == model.DefaultMavenSettingsTypeCode {
		logx.Errorf("默认 Maven 配置类型不允许删除")
		return errorx.Msg("默认 Maven 配置类型不允许删除")
	}
	childCount, err := svcCtx.ConfigTypeModel.CountByParent(ctx, configType.ID.Hex())
	if err != nil {
		logx.Errorf("校验配置类型子节点失败: %v", err)
		return err
	}
	if childCount > 0 {
		logx.Errorf("配置类型存在子节点，不能删除")
		return errorx.Msg("配置类型存在子节点，不能删除")
	}
	configCount, err := svcCtx.ProjectConfigModel.CountByTypeCode(ctx, configType.Code)
	if err != nil {
		logx.Errorf("校验配置类型引用失败: %v", err)
		return err
	}
	if configCount > 0 {
		logx.Errorf("配置类型已被项目配置引用，不能删除")
		return errorx.Msg("配置类型已被项目配置引用，不能删除")
	}
	return nil
}

func ensureConfigTypeParent(ctx context.Context, svcCtx *svc.ServiceContext, id, parentID string) error {
	id = strings.TrimSpace(id)
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return nil
	}
	if id != "" && id == parentID {
		logx.Errorf("父级配置类型不能选择自身")
		return errorx.Msg("父级配置类型不能选择自身")
	}
	if _, err := svcCtx.ConfigTypeModel.FindOne(ctx, parentID); err != nil {
		logx.Errorf("父级配置类型不存在: %v", err)
		return err
	}
	if id == "" {
		return nil
	}
	items, err := svcCtx.ConfigTypeModel.All(ctx, -1)
	if err != nil {
		logx.Errorf("校验配置类型父级失败: %v", err)
		return err
	}
	parentByID := make(map[string]string, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		parentByID[item.ID.Hex()] = item.ParentID
	}
	for current := parentID; current != ""; current = parentByID[current] {
		if current == id {
			logx.Errorf("父级配置类型不能选择自身或子级")
			return errorx.Msg("父级配置类型不能选择自身或子级")
		}
	}
	return nil
}

func normalizeConfigTypeRef(ctx context.Context, svcCtx *svc.ServiceContext, typeID, typeCode string) (*model.DevopsConfigType, error) {
	configType, err := resolveConfigType(ctx, svcCtx, typeID, typeCode)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("配置类型不存在")
		}
		return nil, err
	}
	if configType.Status != 1 {
		logx.Errorf("配置类型已停用")
		return nil, errorx.Msg("配置类型已停用")
	}
	return configType, nil
}

func prepareProjectBuildChannels(ctx context.Context, svcCtx *svc.ServiceContext, channelIDs []string, defaultChannelID string) (string, []*model.DevopsChannel, error) {
	ids := normalizeIDs(channelIDs)
	if len(ids) == 0 {
		logx.Errorf("创建项目必须绑定至少一个构建渠道")
		return "", nil, errorx.Msg("创建项目必须绑定至少一个构建渠道")
	}
	defaultChannelID = strings.TrimSpace(defaultChannelID)
	if defaultChannelID == "" {
		logx.Errorf("请选择默认构建渠道")
		return "", nil, errorx.Msg("请选择默认构建渠道")
	}
	defaultInList := false
	channels := make([]*model.DevopsChannel, 0, len(ids))
	for _, id := range ids {
		if id == defaultChannelID {
			defaultInList = true
		}
		channel, err := svcCtx.ChannelModel.FindOne(ctx, id)
		if err != nil {
			logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
			return "", nil, err
		}
		if channel.Status != 1 {
			logx.Errorf("构建渠道必须是启用状态")
			return "", nil, errorx.Msg("构建渠道必须是启用状态")
		}
		if channel.IsSystem {
			logx.Errorf("构建渠道必须是用户配置的渠道实例")
			return "", nil, errorx.Msg("构建渠道必须是用户配置的渠道实例")
		}
		group, err := svcCtx.ChannelGroupModel.FindOne(ctx, channel.GroupID)
		if err != nil {
			logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
			return "", nil, err
		}
		if !group.IsBuildGroup() {
			logx.Errorf("构建渠道必须归属构建渠道分组")
			return "", nil, errorx.Msg("构建渠道必须归属构建渠道分组")
		}
		channels = append(channels, channel)
	}
	if !defaultInList {
		logx.Errorf("默认构建渠道必须包含在绑定渠道中")
		return "", nil, errorx.Msg("默认构建渠道必须包含在绑定渠道中")
	}
	return defaultChannelID, channels, nil
}

func prepareProjectGroupChannels(ctx context.Context, svcCtx *svc.ServiceContext, channelIDs []string, defaultChannelID, groupCode string) (string, string, []*model.DevopsChannel, error) {
	ids := normalizeIDs(channelIDs)
	if len(ids) == 0 {
		logx.Errorf("请选择需要绑定的渠道实例")
		return "", "", nil, errorx.Msg("请选择需要绑定的渠道实例")
	}
	defaultChannelID = strings.TrimSpace(defaultChannelID)
	groupCode = strings.TrimSpace(groupCode)
	channels := make([]*model.DevopsChannel, 0, len(ids))
	for _, id := range ids {
		channel, err := svcCtx.ChannelModel.FindOne(ctx, id)
		if err != nil {
			logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
			return "", "", nil, err
		}
		if channel.Status != 1 {
			logx.Errorf("渠道实例必须是启用状态")
			return "", "", nil, errorx.Msg("渠道实例必须是启用状态")
		}
		if channel.IsSystem {
			logx.Errorf("只能绑定用户配置的渠道实例")
			return "", "", nil, errorx.Msg("只能绑定用户配置的渠道实例")
		}
		group, err := svcCtx.ChannelGroupModel.FindOne(ctx, channel.GroupID)
		if err != nil {
			logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
			return "", "", nil, err
		}
		if groupCode == "" {
			groupCode = group.Code
		}
		if group.Code != groupCode {
			logx.Errorf("所选渠道实例必须属于同一个渠道分组")
			return "", "", nil, errorx.Msg("所选渠道实例必须属于同一个渠道分组")
		}
		channels = append(channels, channel)
	}
	if isBuildGroupCode(ctx, svcCtx.ChannelGroupModel, groupCode) {
		if defaultChannelID == "" {
			logx.Errorf("请选择默认构建渠道")
			return "", "", nil, errorx.Msg("请选择默认构建渠道")
		}
		defaultInList := false
		for _, channel := range channels {
			if channel.ID.Hex() == defaultChannelID {
				defaultInList = true
				break
			}
		}
		if !defaultInList {
			logx.Errorf("默认构建渠道必须包含在绑定渠道中")
			return "", "", nil, errorx.Msg("默认构建渠道必须包含在绑定渠道中")
		}
	} else if defaultChannelID != "" {
		defaultInList := false
		for _, channel := range channels {
			if channel.ID.Hex() == defaultChannelID {
				defaultInList = true
				break
			}
		}
		if !defaultInList {
			logx.Errorf("默认渠道必须包含在绑定渠道中")
			return "", "", nil, errorx.Msg("默认渠道必须包含在绑定渠道中")
		}
	}
	return defaultChannelID, groupCode, channels, nil
}

func defaultBuildChannelType(channels []*model.DevopsChannel, defaultChannelID string) string {
	for _, channel := range channels {
		if channel.ID.Hex() == defaultChannelID {
			switch channel.ChannelType {
			case "tekton":
				return "tekton"
			default:
				return "jenkins"
			}
		}
	}
	return ""
}

func normalizeProjectTektonBindingConfig(ctx context.Context, svcCtx *svc.ServiceContext, projectID string, channel *model.DevopsChannel, bindingConfig string) (string, error) {
	if channel == nil || channel.ChannelType != tektonsync.EngineTypeTekton {
		return bindingConfig, nil
	}
	project, err := svcCtx.ProjectModel.FindOne(ctx, projectID)
	if err != nil {
		logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
		return "", err
	}
	return tektonsync.NormalizeBindingConfigForProject(project.Code, bindingConfig)
}

func replaceProjectBuildChannels(ctx context.Context, svcCtx *svc.ServiceContext, projectID string, channels []*model.DevopsChannel, defaultChannelID string, allowCredential bool, projectCredentialID, bindingConfig, operator string) error {
	normalizedConfigs, err := validateProjectChannelBindings(ctx, svcCtx, projectID, channels, allowCredential, projectCredentialID, bindingConfig)
	if err != nil {
		return err
	}
	if err := svcCtx.ProjectChannelModel.DeleteSoftByProjectScope(ctx, projectID, model.BuildUsageScope, operator); err != nil {
		logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
		return err
	}
	for _, channel := range channels {
		normalizedBindingConfig := normalizedConfigs[channel.ID.Hex()]
		if normalizedBindingConfig == "" {
			normalizedBindingConfig = bindingConfig
		}
		data := &model.DevopsProjectChannelBinding{
			ProjectID:                projectID,
			ChannelID:                channel.ID.Hex(),
			ChannelGroupCode:         model.BuildChannelGroupCode, // 默认构建渠道分组
			ChannelName:              channel.Name,
			ChannelCode:              channel.Code,
			ChannelType:              channel.ChannelType,
			UsageScope:               model.BuildUsageScope, // 默认构建用途
			IsDefault:                channel.ID.Hex() == defaultChannelID,
			AllowUseGlobalCredential: allowCredential,
			ProjectCredentialID:      projectCredentialID,
			BindingConfig:            normalizedBindingConfig,
			Status:                   1,
			CreatedBy:                operator,
			UpdatedBy:                operator,
		}
		if err := svcCtx.ProjectChannelModel.Insert(ctx, data); err != nil {
			if model.IsDuplicateKey(err) {
				logx.Errorf("项目渠道绑定已存在")
				return errorx.Msg("项目渠道绑定已存在")
			}
			logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
			return err
		}
	}
	return nil
}

func replaceProjectGroupChannels(ctx context.Context, svcCtx *svc.ServiceContext, projectID, groupCode string, channels []*model.DevopsChannel, defaultChannelID string, allowCredential bool, projectCredentialID, bindingConfig, operator string) error {
	usageScope := groupCode
	if isBuildGroupCode(ctx, svcCtx.ChannelGroupModel, groupCode) {
		usageScope = model.BuildUsageScope
	}
	normalizedConfigs, err := validateProjectChannelBindings(ctx, svcCtx, projectID, channels, allowCredential, projectCredentialID, bindingConfig)
	if err != nil {
		return err
	}
	if err := svcCtx.ProjectChannelModel.DeleteSoftByProjectGroup(ctx, projectID, groupCode, operator); err != nil {
		logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
		return err
	}
	for _, channel := range channels {
		normalizedBindingConfig := normalizedConfigs[channel.ID.Hex()]
		if normalizedBindingConfig == "" {
			normalizedBindingConfig = bindingConfig
		}
		data := &model.DevopsProjectChannelBinding{
			ProjectID:                projectID,
			ChannelID:                channel.ID.Hex(),
			ChannelGroupCode:         groupCode,
			ChannelName:              channel.Name,
			ChannelCode:              channel.Code,
			ChannelType:              channel.ChannelType,
			UsageScope:               usageScope,
			IsDefault:                channel.ID.Hex() == defaultChannelID,
			AllowUseGlobalCredential: allowCredential,
			ProjectCredentialID:      projectCredentialID,
			BindingConfig:            normalizedBindingConfig,
			Status:                   1,
			CreatedBy:                operator,
			UpdatedBy:                operator,
		}
		if err := svcCtx.ProjectChannelModel.Insert(ctx, data); err != nil {
			if model.IsDuplicateKey(err) {
				logx.Errorf("项目渠道绑定已存在")
				return errorx.Msg("项目渠道绑定已存在")
			}
			logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
			return err
		}
	}
	return nil
}

func validateProjectChannelBindings(ctx context.Context, svcCtx *svc.ServiceContext, projectID string, channels []*model.DevopsChannel, allowCredential bool, projectCredentialID, bindingConfig string) (map[string]string, error) {
	result := make(map[string]string, len(channels))
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		groupCode := resolveChannelGroupCode(ctx, svcCtx, channel)
		if err := validateProjectBindingCredential(ctx, svcCtx, projectID, groupCode, channel.ChannelType, allowCredential, projectCredentialID); err != nil {
			logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
			return nil, err
		}
		normalizedBindingConfig, err := normalizeProjectTektonBindingConfig(ctx, svcCtx, projectID, channel, bindingConfig)
		if err != nil {
			logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
			return nil, err
		}
		if err := tektonsync.ValidateBindingConfig(ctx, svcCtx, channel, normalizedBindingConfig); err != nil {
			logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
			return nil, err
		}
		result[channel.ID.Hex()] = normalizedBindingConfig
	}
	return result, nil
}

func resolveChannelGroup(ctx context.Context, svcCtx *svc.ServiceContext, channel *model.DevopsChannel) (*model.DevopsChannelGroup, error) {
	if channel == nil || channel.GroupID == "" {
		logx.Errorf("渠道实例未配置分组")
		return nil, errorx.Msg("渠道实例未配置分组")
	}
	group, err := svcCtx.ChannelGroupModel.FindOne(ctx, channel.GroupID)
	if err != nil {
		logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
		return nil, err
	}
	return group, nil
}

func resolveChannelGroupCode(ctx context.Context, svcCtx *svc.ServiceContext, channel *model.DevopsChannel) string {
	group, err := resolveChannelGroup(ctx, svcCtx, channel)
	if err != nil {
		return ""
	}
	return group.Code
}

func resolveBindingGroupCode(ctx context.Context, svcCtx *svc.ServiceContext, binding *model.DevopsProjectChannelBinding) string {
	if binding == nil {
		return ""
	}
	if binding.ChannelGroupCode != "" {
		return binding.ChannelGroupCode
	}
	channel, err := svcCtx.ChannelModel.FindOne(ctx, binding.ChannelID)
	if err != nil {
		return ""
	}
	group, err := svcCtx.ChannelGroupModel.FindOne(ctx, channel.GroupID)
	if err != nil {
		return ""
	}
	return group.Code
}

func validateProjectBindingCredential(ctx context.Context, svcCtx *svc.ServiceContext, projectID, channelGroupCode, channelType string, allowGlobal bool, projectCredentialID string) error {
	projectCredentialID = strings.TrimSpace(projectCredentialID)
	if projectCredentialID == "" {
		return nil
	}
	credential, err := svcCtx.CredentialModel.FindOne(ctx, projectCredentialID)
	if err != nil {
		logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
		return err
	}
	if credential.Status != 1 {
		logx.Errorf("项目凭据已停用")
		return errorx.Msg("项目凭据已停用")
	}
	scope := strings.TrimSpace(credential.Scope)
	if credential.IsSystem || scope == "system" || credential.ProjectID != projectID || (scope != "" && scope != "project") {
		logx.Errorf("只能使用当前项目的项目级凭据")
		return errorx.Msg("只能使用当前项目的项目级凭据")
	}
	credentialGroupCode := strings.TrimSpace(credential.ChannelGroupCode)
	credentialChannelType := strings.TrimSpace(credential.ChannelType)
	if credentialGroupCode != strings.TrimSpace(channelGroupCode) || credentialChannelType != strings.TrimSpace(channelType) {
		logx.Errorf("项目凭据不属于当前渠道类型")
		return errorx.Msg("项目凭据不属于当前渠道类型")
	}
	channelTypeData, err := svcCtx.ChannelTypeModel.FindOneByCode(ctx, channelType)
	if err != nil {
		logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
		return err
	}
	if len(channelTypeData.CredentialTypes) > 0 && !stringInList(projectChannelCredentialTypes(channelTypeData.CredentialTypes), credential.CredentialType) {
		logx.Errorf("项目凭据类型不适用于当前渠道")
		return errorx.Msg("项目凭据类型不适用于当前渠道")
	}
	return nil
}

func projectChannelCredentialTypes(items []string) []string {
	return normalizeIDs(append(append([]string{}, baseProjectChannelCredentialTypes...), items...))
}

func stringInList(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func updateProjectDefaultBuildChannel(ctx context.Context, svcCtx *svc.ServiceContext, projectID string, channel *model.DevopsChannel, operator string) error {
	project, err := svcCtx.ProjectModel.FindOne(ctx, projectID)
	if err != nil {
		logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
		return err
	}
	project.PipelineEngineType = channel.ChannelType
	if channel.ChannelType != "tekton" {
		project.PipelineEngineType = "jenkins"
	}
	project.DefaultEngineChannelID = channel.ID.Hex()
	project.UpdatedBy = operator
	if err := svcCtx.ProjectModel.Update(ctx, project); err != nil {
		if model.IsDuplicateKey(err) {
			logx.Errorf("项目编码已存在")
			return errorx.Msg("项目编码已存在")
		}
		logx.Errorf("处理项目渠道辅助逻辑失败: %v", err)
		return err
	}
	return nil
}

func normalizeIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

// isBuildGroupCode 判断 groupCode 是否为构建渠道分组，通过查询分组属性动态判断。
func isBuildGroupCode(ctx context.Context, groupModel *model.DevopsChannelGroupModel, groupCode string) bool {
	if groupCode == "" {
		return false
	}
	group, err := groupModel.FindOneByCode(ctx, groupCode)
	if err != nil {
		return false
	}
	return group.IsBuildGroup()
}

// resolveGroup 根据 groupCode 查询分组对象。
func resolveGroup(ctx context.Context, groupModel *model.DevopsChannelGroupModel, groupCode string) *model.DevopsChannelGroup {
	group, err := groupModel.FindOneByCode(ctx, groupCode)
	if err != nil {
		return nil
	}
	return group
}
