package channelservicelogic

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	portalprojectservice "github.com/yanshicheng/kube-nova/application/portal-rpc/client/portalprojectservice"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

const maskedSecret = "******"

var codePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var baseChannelCredentialTypes = []string{"username_password", "token"}

func channelGroupToPb(in *model.DevopsChannelGroup) *pb.DevopsChannelGroup {
	if in == nil {
		return nil
	}
	return &pb.DevopsChannelGroup{
		Id:                  in.ID.Hex(),
		Name:                in.Name,
		Code:                in.Code,
		Description:         in.Description,
		SortOrder:           in.SortOrder,
		IsSystem:            in.IsSystem,
		Status:              in.Status,
		CreatedBy:           in.CreatedBy,
		UpdatedBy:           in.UpdatedBy,
		CreatedAt:           in.CreateAt.Unix(),
		UpdatedAt:           in.UpdateAt.Unix(),
		GroupType:           in.GroupType,
		AllowedChannelTypes: in.AllowedChannelTypes,
		Icon:                in.Icon,
		IconColor:           in.IconColor,
	}
}

func channelToPb(in *model.DevopsChannel) *pb.DevopsChannel {
	if in == nil {
		return nil
	}
	return &pb.DevopsChannel{
		Id:                 in.ID.Hex(),
		GroupId:            in.GroupID,
		Name:               in.Name,
		Code:               in.Code,
		ChannelType:        in.ChannelType,
		Endpoint:           in.Endpoint,
		Description:        in.Description,
		GlobalCredentialId: in.GlobalCredentialID,
		CredentialId:       in.CredentialID,
		Config:             in.Config,
		Labels:             in.Labels,
		HealthStatus:       in.HealthStatus,
		LastCheckAt:        in.LastCheckAt,
		LastCheckMessage:   in.LastCheckMessage,
		Metadata:           normalizeMetadataJSON(in.Metadata),
		Status:             in.Status,
		IsSystem:           in.IsSystem,
		AuthType:           in.AuthType,
		Username:           in.Username,
		Password:           maskSecret(in.Password),
		Token:              maskSecret(in.Token),
		InsecureSkipTls:    in.InsecureSkipTLS,
		Icon:               in.Icon,
		IconColor:          in.IconColor,
		CreatedBy:          in.CreatedBy,
		UpdatedBy:          in.UpdatedBy,
		CreatedAt:          in.CreateAt.Unix(),
		UpdatedAt:          in.UpdateAt.Unix(),
	}
}

func channelTypeToPb(in *model.DevopsChannelType) *pb.DevopsChannelType {
	if in == nil {
		return nil
	}
	return &pb.DevopsChannelType{
		Id:               in.ID.Hex(),
		Code:             in.Code,
		Name:             in.Name,
		GroupCode:        in.GroupCode,
		CredentialTypes:  normalizeCredentialTypeList(in.CredentialTypes),
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
		CreatedAt:        in.CreateAt.Unix(),
		UpdatedAt:        in.UpdateAt.Unix(),
	}
}

func normalizeChannelTypeMappingFields(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return channelvars.NormalizeVariableSchema(raw)
	}
	value, err := channelvars.NormalizeVariableSchema(raw)
	if errors.Is(err, channelvars.ErrLegacyVariableField) {
		return "", errorx.Msg("映射字段不能使用旧 dynamic.*Url 协议")
	}
	return value, err
}

func credentialToPb(in *model.DevopsCredential) *pb.DevopsCredential {
	if in == nil {
		return nil
	}
	scope := normalizeCredentialScope(in.Scope, in.ProjectID, in.IsSystem)
	return &pb.DevopsCredential{
		Id:               in.ID.Hex(),
		Name:             in.Name,
		Code:             in.Code,
		CredentialType:   in.CredentialType,
		Username:         in.Username,
		Password:         maskSecret(in.Password),
		Token:            maskSecret(in.Token),
		PrivateKey:       maskSecret(in.PrivateKey),
		Passphrase:       maskSecret(in.Passphrase),
		Kubeconfig:       maskSecret(in.Kubeconfig),
		SecretText:       maskSecret(in.SecretText),
		Certificate:      maskSecret(in.Certificate),
		JsonData:         maskSecret(in.JsonData),
		Description:      in.Description,
		Status:           in.Status,
		IsSystem:         in.IsSystem,
		Scope:            scope,
		ProjectId:        in.ProjectID,
		ChannelGroupCode: in.ChannelGroupCode,
		ChannelType:      in.ChannelType,
		CreatedBy:        in.CreatedBy,
		UpdatedBy:        in.UpdatedBy,
		CreatedAt:        in.CreateAt.Unix(),
		UpdatedAt:        in.UpdateAt.Unix(),
	}
}

func applyCredentialSync(out *pb.DevopsCredential, sync *model.DevopsCredentialSync) {
	if out == nil {
		return
	}
	if sync == nil {
		out.JenkinsSyncStatus = "pending"
		return
	}
	out.JenkinsSyncStatus = sync.SyncStatus
	out.JenkinsSyncMessage = sync.SyncMessage
	out.JenkinsCredentialId = sync.JenkinsCredentialID
	if !sync.LastSyncAt.IsZero() {
		out.JenkinsLastSyncAt = sync.LastSyncAt.Unix()
	}
}

func normalizeCredentialScope(scope, projectID string, isSystem bool) string {
	scope = strings.TrimSpace(scope)
	if isSystem {
		return "system"
	}
	if scope == "project" {
		return "project"
	}
	if scope == "system" || strings.TrimSpace(projectID) == "" {
		return "system"
	}
	return "project"
}

func isSuperAdminRole(roles []string) bool {
	for _, role := range roles {
		if strings.EqualFold(strings.TrimSpace(role), "SUPER_ADMIN") {
			return true
		}
	}
	return false
}

func userProjectIDs(ctx context.Context, svcCtx *svc.ServiceContext, userID uint64, roles []string) ([]string, bool, error) {
	if isSuperAdminRole(roles) {
		return nil, false, nil
	}
	ids, err := svcCtx.ProjectMemberModel.ListProjectIDsByUser(ctx, userID)
	if err != nil {
		logx.Errorf("处理渠道辅助逻辑失败: %v", err)
		return nil, false, err
	}
	platformID, _ := ctx.Value("platformId").(uint64)
	if platformID == 0 || len(ids) == 0 {
		return []string{}, true, nil
	}
	portalResp, err := svcCtx.PortalRpc.ListProjects(ctx, &portalprojectservice.PortalListProjectsReq{
		Page:       1,
		PageSize:   uint64(len(ids)),
		UserId:     userID,
		PlatformId: platformID,
	})
	if err != nil {
		logx.Errorf("处理渠道辅助逻辑失败: %v", err)
		return nil, false, err
	}
	allowed := make(map[string]struct{}, len(portalResp.Data))
	for _, item := range portalResp.Data {
		if item == nil || item.Uuid == "" {
			continue
		}
		allowed[item.Uuid] = struct{}{}
	}
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		project, err := svcCtx.ProjectModel.FindOne(ctx, id)
		if err != nil || project == nil || project.PortalProjectUuid == "" {
			continue
		}
		if _, ok := allowed[project.PortalProjectUuid]; ok {
			filtered = append(filtered, id)
		}
	}
	return filtered, true, nil
}

func ensureCredentialAccess(ctx context.Context, svcCtx *svc.ServiceContext, credential *model.DevopsCredential, userID uint64, roles []string) error {
	if credential == nil {
		logx.Errorf("凭据不存在")
		return errorx.Msg("凭据不存在")
	}
	scope := normalizeCredentialScope(credential.Scope, credential.ProjectID, credential.IsSystem)
	if isSuperAdminRole(roles) {
		return nil
	}
	if scope == "system" {
		logx.Errorf("无权操作系统级凭证")
		return errorx.Msg("无权操作系统级凭证")
	}
	if credential.ProjectID == "" {
		logx.Errorf("凭据未绑定项目")
		return errorx.Msg("凭据未绑定项目")
	}
	projectIDs, _, err := userProjectIDs(ctx, svcCtx, userID, roles)
	if err != nil {
		logx.Errorf("处理渠道辅助逻辑失败: %v", err)
		return err
	}
	if !containsString(projectIDs, credential.ProjectID) {
		logx.Errorf("无权操作该项目凭证")
		return errorx.Msg("无权操作该项目凭证")
	}
	return nil
}

func ensureCredentialReadAccess(ctx context.Context, svcCtx *svc.ServiceContext, credential *model.DevopsCredential, userID uint64, roles []string) error {
	return ensureCredentialAccess(ctx, svcCtx, credential, userID, roles)
}

func ensureCredentialWriteAccess(ctx context.Context, svcCtx *svc.ServiceContext, credential *model.DevopsCredential, userID uint64, roles []string) error {
	return ensureCredentialAccess(ctx, svcCtx, credential, userID, roles)
}

func ensureCredentialTargetAccess(ctx context.Context, svcCtx *svc.ServiceContext, scope, projectID string, userID uint64, roles []string) error {
	scope = normalizeCredentialScope(scope, projectID, false)
	if scope == "system" {
		if !isSuperAdminRole(roles) {
			logx.Errorf("只有 SUPER_ADMIN 可以创建或维护系统级凭证")
			return errorx.Msg("只有 SUPER_ADMIN 可以创建或维护系统级凭证")
		}
		return nil
	}
	if strings.TrimSpace(projectID) == "" {
		logx.Errorf("项目级凭证必须选择项目")
		return errorx.Msg("项目级凭证必须选择项目")
	}
	if isSuperAdminRole(roles) {
		return nil
	}
	projectIDs, _, err := userProjectIDs(ctx, svcCtx, userID, roles)
	if err != nil {
		logx.Errorf("处理渠道辅助逻辑失败: %v", err)
		return err
	}
	if !containsString(projectIDs, projectID) {
		logx.Errorf("只能操作自己绑定项目的凭证")
		return errorx.Msg("只能操作自己绑定项目的凭证")
	}
	return nil
}

func hostToPb(in *model.DevopsHost) *pb.DevopsHost {
	if in == nil {
		return nil
	}
	return &pb.DevopsHost{
		Id:               in.ID.Hex(),
		Name:             in.Name,
		Ip:               in.IP,
		Port:             in.Port,
		CredentialId:     in.CredentialID,
		Labels:           in.Labels,
		Description:      in.Description,
		HealthStatus:     in.HealthStatus,
		LastCheckAt:      in.LastCheckAt,
		LastCheckMessage: in.LastCheckMessage,
		Metadata:         normalizeMetadataJSON(in.Metadata),
		Status:           in.Status,
		CreatedBy:        in.CreatedBy,
		UpdatedBy:        in.UpdatedBy,
		CreatedAt:        in.CreateAt.Unix(),
		UpdatedAt:        in.UpdateAt.Unix(),
	}
}

func normalizeAuthType(authType string) string {
	authType = strings.TrimSpace(authType)
	if authType == "" {
		return "none"
	}
	return authType
}

func normalizeMetadataJSON(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return raw
	}
	if version, ok := data["version"].(string); ok && strings.TrimSpace(version) != "" {
		if shouldDropAPIVersionAlias(data, version) {
			delete(data, "version")
			normalized, err := json.Marshal(data)
			if err != nil {
				return raw
			}
			return string(normalized)
		}
		return raw
	}
	for _, key := range []string{"productVersion", "engineVersion", "schemaVersion", "benchmarkProfile"} {
		if value, ok := data[key].(string); ok && strings.TrimSpace(value) != "" {
			data["version"] = strings.TrimSpace(value)
			normalized, err := json.Marshal(data)
			if err != nil {
				return raw
			}
			return string(normalized)
		}
	}
	if shouldUseAPIVersionAsVersion(data) {
		if value, ok := data["apiVersion"].(string); ok && strings.TrimSpace(value) != "" {
			data["version"] = strings.TrimSpace(value)
			normalized, err := json.Marshal(data)
			if err != nil {
				return raw
			}
			return string(normalized)
		}
	}
	return raw
}

func shouldDropAPIVersionAlias(data map[string]any, version string) bool {
	productName, _ := data["productName"].(string)
	productVersion, _ := data["productVersion"].(string)
	apiVersion, _ := data["apiVersion"].(string)
	return strings.EqualFold(strings.TrimSpace(productName), "GitLab") &&
		strings.TrimSpace(productVersion) == "" &&
		strings.TrimSpace(apiVersion) != "" &&
		strings.TrimSpace(version) == strings.TrimSpace(apiVersion)
}

func shouldUseAPIVersionAsVersion(data map[string]any) bool {
	productName, _ := data["productName"].(string)
	switch strings.ToLower(strings.TrimSpace(productName)) {
	case "docker registry", "github", "gitee", "tekton":
		return true
	default:
		return false
	}
}

func maskSecret(secret string) string {
	if strings.TrimSpace(secret) == "" {
		return ""
	}
	return maskedSecret
}

func normalizeStringList(items []string) []string {
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func normalizeCredentialTypeList(items []string) []string {
	return normalizeStringList(append(append([]string{}, baseChannelCredentialTypes...), items...))
}

func credentialTypeAllowedForChannel(items []string, credentialType string) bool {
	return containsString(normalizeCredentialTypeList(items), credentialType)
}

func keepMaskedSecret(input, exist string) string {
	input = strings.TrimSpace(input)
	if input == "" || input == maskedSecret {
		return exist
	}
	return input
}

func syncGroupAllowedChannelType(ctx context.Context, svcCtx *svc.ServiceContext, groupCode, channelType, updatedBy string, add bool) error {
	group, err := svcCtx.ChannelGroupModel.FindOneByCode(ctx, groupCode)
	if err != nil {
		return err
	}
	if group.IsSystem {
		return nil
	}
	if add {
		return svcCtx.ChannelGroupModel.AddAllowedChannelType(ctx, groupCode, channelType, updatedBy)
	}
	return svcCtx.ChannelGroupModel.RemoveAllowedChannelType(ctx, groupCode, channelType, updatedBy)
}

func validateChannelScope(ctx context.Context, svcCtx *svc.ServiceContext, groupID, channelType, credentialID string) error {
	group, err := svcCtx.ChannelGroupModel.FindOne(ctx, groupID)
	if err != nil {
		logx.Errorf("处理渠道辅助逻辑失败: %v", err)
		return err
	}
	channelTypeData, err := svcCtx.ChannelTypeModel.FindOneByCode(ctx, channelType)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			logx.Errorf("渠道类型不存在")
			return errorx.Msg("渠道类型不存在")
		}
		logx.Errorf("处理渠道辅助逻辑失败: %v", err)
		return err
	}
	if channelTypeData.Status != 1 {
		logx.Errorf("渠道类型已停用")
		return errorx.Msg("渠道类型已停用")
	}
	if group.Code != "custom" && channelTypeData.GroupCode != group.Code {
		logx.Errorf("渠道类型与当前分组不匹配")
		return errorx.Msg("渠道类型与当前分组不匹配")
	}
	if strings.TrimSpace(credentialID) == "" {
		return nil
	}
	credential, err := svcCtx.CredentialModel.FindOne(ctx, credentialID)
	if err != nil {
		logx.Errorf("处理渠道辅助逻辑失败: %v", err)
		return err
	}
	if credential.Status != 1 {
		logx.Errorf("凭据已停用")
		return errorx.Msg("凭据已停用")
	}
	if !credentialChannelTargetMatches(credential, group.Code, channelType) {
		logx.Errorf("凭据不属于当前渠道类型")
		return errorx.Msg("凭据不属于当前渠道类型")
	}
	if len(channelTypeData.CredentialTypes) > 0 && !credentialTypeAllowedForChannel(channelTypeData.CredentialTypes, credential.CredentialType) {
		logx.Errorf("凭据类型不适用于当前渠道类型")
		return errorx.Msg("凭据类型不适用于当前渠道类型")
	}
	return nil
}

func credentialChannelTargetMatches(credential *model.DevopsCredential, groupCode, channelType string) bool {
	if credential == nil {
		return false
	}
	credentialGroupCode := strings.TrimSpace(credential.ChannelGroupCode)
	credentialChannelType := strings.TrimSpace(credential.ChannelType)
	if credentialGroupCode == "" && credentialChannelType == "" {
		return true
	}
	return credentialGroupCode == strings.TrimSpace(groupCode) && credentialChannelType == strings.TrimSpace(channelType)
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func validateCredentialPayload(data *model.DevopsCredential) error {
	switch data.CredentialType {
	case "username_password":
		if data.Username == "" || data.Password == "" {
			logx.Errorf("用户名密码凭据需要填写用户名和密码")
			return errorx.Msg("用户名密码凭据需要填写用户名和密码")
		}
	case "token":
		if data.Token == "" {
			logx.Errorf("Token 凭据需要填写 Token")
			return errorx.Msg("Token 凭据需要填写 Token")
		}
	case "ssh_key":
		if data.Username == "" || data.PrivateKey == "" {
			logx.Errorf("SSH Key 凭据需要填写用户名和私钥")
			return errorx.Msg("SSH Key 凭据需要填写用户名和私钥")
		}
	case "kubeconfig":
		if data.Kubeconfig == "" {
			logx.Errorf("Kubeconfig 凭据需要填写 kubeconfig")
			return errorx.Msg("Kubeconfig 凭据需要填写 kubeconfig")
		}
	case "secret_text":
		if data.SecretText == "" {
			logx.Errorf("Secret Text 凭据需要填写密文")
			return errorx.Msg("Secret Text 凭据需要填写密文")
		}
	case "certificate":
		if data.Certificate == "" {
			logx.Errorf("证书凭据需要填写证书内容")
			return errorx.Msg("证书凭据需要填写证书内容")
		}
	case "json":
		if !json.Valid([]byte(data.JsonData)) {
			logx.Errorf("JSON 凭据需要填写合法 JSON")
			return errorx.Msg("JSON 凭据需要填写合法 JSON")
		}
	default:
		logx.Errorf("凭据类型不支持")
		return errorx.Msg("凭据类型不支持")
	}
	return nil
}

func validateCredentialChannelTarget(ctx context.Context, svcCtx *svc.ServiceContext, groupCode, channelType, credentialType string) error {
	groupCode = strings.TrimSpace(groupCode)
	channelType = strings.TrimSpace(channelType)
	if groupCode == "" && channelType == "" {
		return nil
	}
	if groupCode == "" || channelType == "" {
		logx.Errorf("渠道凭证必须同时选择渠道和渠道类型")
		return errorx.Msg("渠道凭证必须同时选择渠道和渠道类型")
	}
	channelTypeData, err := svcCtx.ChannelTypeModel.FindOneByCode(ctx, channelType)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			logx.Errorf("渠道类型不存在")
			return errorx.Msg("渠道类型不存在")
		}
		logx.Errorf("处理渠道辅助逻辑失败: %v", err)
		return err
	}
	if channelTypeData.Status != 1 {
		logx.Errorf("渠道类型已停用")
		return errorx.Msg("渠道类型已停用")
	}
	if channelTypeData.GroupCode != groupCode {
		logx.Errorf("渠道类型与渠道不匹配")
		return errorx.Msg("渠道类型与渠道不匹配")
	}
	if len(channelTypeData.CredentialTypes) > 0 && !credentialTypeAllowedForChannel(channelTypeData.CredentialTypes, credentialType) {
		logx.Errorf("凭证类型不适用于当前渠道类型")
		return errorx.Msg("凭证类型不适用于当前渠道类型")
	}
	return nil
}

func validateHostCredential(ctx context.Context, svcCtx *svc.ServiceContext, credentialID string) error {
	if strings.TrimSpace(credentialID) == "" {
		return nil
	}
	credential, err := svcCtx.CredentialModel.FindOne(ctx, credentialID)
	if err != nil {
		logx.Errorf("处理渠道辅助逻辑失败: %v", err)
		return err
	}
	if credential.Status != 1 {
		logx.Errorf("凭据已停用")
		return errorx.Msg("凭据已停用")
	}
	if credential.CredentialType != "username_password" && credential.CredentialType != "ssh_key" {
		logx.Errorf("主机资产只支持用户名密码或 SSH Key 凭据")
		return errorx.Msg("主机资产只支持用户名密码或 SSH Key 凭据")
	}
	return nil
}

func validateDictionaryCode(code, field string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		logx.Errorf("%s", field+"不能为空")
		return errorx.Msg(field + "不能为空")
	}
	if !codePattern.MatchString(code) {
		logx.Errorf("%s", field+"只能包含字母、数字、下划线和中划线")
		return errorx.Msg(field + "只能包含字母、数字、下划线和中划线")
	}
	return nil
}
