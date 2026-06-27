package pipelineconfigservicelogic

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/jenkins"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

var jenkinsCredentialCodePattern = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

type jenkinsSyncRuntime struct {
	project *model.DevopsProject
	binding *model.DevopsProjectChannelBinding
	channel *model.DevopsChannel
	manager *jenkins.Manager
	folder  string
}

func syncJenkinsCredentials(ctx context.Context, svcCtx *svc.ServiceContext, in *pb.SyncJenkinsCredentialsReq) ([]*pb.JenkinsCredentialMapping, error) {
	credentialIDs := uniqueNonEmptyStrings(in.CredentialIds)
	if len(credentialIDs) == 0 {
		return []*pb.JenkinsCredentialMapping{}, nil
	}
	runtime, err := buildJenkinsSyncRuntime(ctx, svcCtx, in.ProjectId, in.BuildChannelBindingId)
	if err != nil {
		logx.Errorf("同步 Jenkins 凭证失败: %v", err)
		return nil, err
	}
	result := make([]*pb.JenkinsCredentialMapping, 0, len(credentialIDs))
	for _, credentialID := range credentialIDs {
		mapping, err := syncOneJenkinsCredential(ctx, svcCtx, runtime, credentialID, fmt.Sprint(in.CurrentUserId))
		if err != nil {
			logx.Errorf("同步 Jenkins 凭证失败，credentialId: %s, 错误: %v", credentialID, err)
			return nil, err
		}
		result = append(result, mapping)
	}
	return result, nil
}

func buildJenkinsSyncRuntime(ctx context.Context, svcCtx *svc.ServiceContext, projectID, buildChannelBindingID string) (*jenkinsSyncRuntime, error) {
	project, err := svcCtx.ProjectModel.FindOne(ctx, projectID)
	if err != nil {
		return nil, err
	}
	binding, err := svcCtx.ProjectChannelModel.FindOne(ctx, buildChannelBindingID)
	if err != nil {
		return nil, err
	}
	if binding.ProjectID != project.ID.Hex() {
		return nil, errorx.Msg("构建渠道绑定不属于当前项目")
	}
	if binding.Status != 1 {
		return nil, errorx.Msg("构建渠道绑定已停用")
	}
	if binding.ChannelGroupCode != jenkinsEngineChannelGroupCode || binding.UsageScope != "build" {
		return nil, errorx.Msg("请选择构建渠道绑定")
	}
	if binding.ChannelType != "jenkins" {
		return nil, errorx.Msg("一期只支持 Jenkins 构建渠道")
	}
	channel, err := svcCtx.ChannelModel.FindOne(ctx, binding.ChannelID)
	if err != nil {
		return nil, err
	}
	if channel.Status != 1 {
		return nil, errorx.Msg("构建渠道已停用")
	}
	credential, ready, message, err := resolveBindingCredential(ctx, svcCtx, project.ID.Hex(), binding, channel)
	if err != nil {
		return nil, err
	}
	if !ready {
		if message != "" {
			return nil, errorx.Msg(message)
		}
		return nil, errorx.Msg("构建渠道凭据不可用")
	}
	username, password, token := "", "", ""
	if credential != nil {
		username = credential.Username
		password = credential.Password
		token = credential.Token
	}
	if username == "" {
		username = channel.Username
	}
	if password == "" && token == "" {
		password = channel.Password
		token = channel.Token
	}
	return &jenkinsSyncRuntime{
		project: project,
		binding: binding,
		channel: channel,
		manager: jenkins.NewManager(jenkins.ClientConfig{
			Endpoint:        channel.Endpoint,
			Username:        username,
			Password:        password,
			Token:           token,
			InsecureSkipTLS: channel.InsecureSkipTLS,
		}),
		folder: jenkinsFolderName(project.Name, project.Code),
	}, nil
}

func syncOneJenkinsCredential(ctx context.Context, svcCtx *svc.ServiceContext, runtime *jenkinsSyncRuntime, credentialID, operator string) (*pb.JenkinsCredentialMapping, error) {
	credential, err := svcCtx.CredentialModel.FindOne(ctx, credentialID)
	if err != nil {
		if err == model.ErrInvalidObjectID {
			logx.Errorf("同步 Jenkins 凭证失败：凭证 ID 格式错误，credentialIdLength: %d", len(strings.TrimSpace(credentialID)))
			return nil, errorx.Msg("凭证 ID 格式错误，请重新选择凭证")
		}
		logx.Errorf("查询 Jenkins 凭证失败: %v", err)
		return nil, errorx.Msg("查询 Jenkins 凭证失败")
	}
	if credential.Status != 1 {
		return nil, errorx.Msg("凭证已停用")
	}
	scope := normalizeCredentialScope(credential.Scope, credential.ProjectID, credential.IsSystem)
	if scope != "project" || credential.ProjectID != runtime.project.ID.Hex() {
		return nil, errorx.Msg("Jenkins 流水线凭证必须属于当前项目")
	}
	secret, err := credential.Secret()
	if err != nil {
		return nil, err
	}
	jenkinsCredentialID := stableJenkinsCredentialID(runtime.project.Code, credential.Code)
	mapping := &pb.JenkinsCredentialMapping{
		CredentialId:        credential.ID.Hex(),
		JenkinsCredentialId: jenkinsCredentialID,
		SyncStatus:          "synced",
	}
	err = runtime.manager.UpsertFolderCredential(ctx, runtime.folder, jenkins.CredentialConfig{
		ID:          jenkinsCredentialID,
		Description: credential.Name,
		Type:        jenkinsCredentialType(credential.CredentialType),
		Username:    secret.Username,
		Password:    secret.Password,
		Token:       secret.Token,
		SecretText:  secret.SecretText,
		PrivateKey:  secret.PrivateKey,
		Passphrase:  secret.Passphrase,
		Kubeconfig:  secret.Kubeconfig,
		Certificate: secret.Certificate,
		JsonData:    secret.JsonData,
	})
	syncRecord := &model.DevopsCredentialSync{
		ProjectID:             runtime.project.ID.Hex(),
		BuildChannelBindingID: runtime.binding.ID.Hex(),
		CredentialID:          credential.ID.Hex(),
		JenkinsCredentialID:   jenkinsCredentialID,
		SyncStatus:            "synced",
		UpdatedBy:             operator,
		LastSyncAt:            time.Now(),
	}
	if err != nil {
		syncRecord.SyncStatus = "failed"
		syncRecord.SyncMessage = err.Error()
		if saveErr := svcCtx.CredentialSyncModel.Upsert(ctx, syncRecord); saveErr != nil {
			logx.Errorf("%s", saveErr)
		}
		return nil, errorx.Msg("同步 Jenkins 凭证失败：" + credential.Name)
	}
	if err := svcCtx.CredentialSyncModel.Upsert(ctx, syncRecord); err != nil {
		return nil, err
	}
	return mapping, nil
}

func syncOneResolvedJenkinsCredential(ctx context.Context, svcCtx *svc.ServiceContext, runtime *jenkinsSyncRuntime, credential *model.DevopsCredential, operator string) (*pb.JenkinsCredentialMapping, error) {
	if credential == nil {
		return nil, errorx.Msg("凭证不存在")
	}
	if credential.Status != 1 {
		return nil, errorx.Msg("凭证已停用")
	}
	secret, err := credential.Secret()
	if err != nil {
		return nil, err
	}
	jenkinsCredentialID := stableJenkinsCredentialID(runtime.project.Code, credential.Code)
	mapping := &pb.JenkinsCredentialMapping{
		CredentialId:        credential.ID.Hex(),
		JenkinsCredentialId: jenkinsCredentialID,
		SyncStatus:          "synced",
	}
	err = runtime.manager.UpsertFolderCredential(ctx, runtime.folder, jenkins.CredentialConfig{
		ID:          jenkinsCredentialID,
		Description: credential.Name,
		Type:        jenkinsCredentialType(credential.CredentialType),
		Username:    secret.Username,
		Password:    secret.Password,
		Token:       secret.Token,
		SecretText:  secret.SecretText,
		PrivateKey:  secret.PrivateKey,
		Passphrase:  secret.Passphrase,
		Kubeconfig:  secret.Kubeconfig,
		Certificate: secret.Certificate,
		JsonData:    secret.JsonData,
	})
	syncRecord := &model.DevopsCredentialSync{
		ProjectID:             runtime.project.ID.Hex(),
		BuildChannelBindingID: runtime.binding.ID.Hex(),
		CredentialID:          credential.ID.Hex(),
		JenkinsCredentialID:   jenkinsCredentialID,
		SyncStatus:            "synced",
		UpdatedBy:             operator,
		LastSyncAt:            time.Now(),
	}
	if err != nil {
		syncRecord.SyncStatus = "failed"
		syncRecord.SyncMessage = err.Error()
		if saveErr := svcCtx.CredentialSyncModel.Upsert(ctx, syncRecord); saveErr != nil {
			logx.Errorf("%v", saveErr)
		}
		return nil, errorx.Msg("同步 Jenkins 凭证失败：" + credential.Name)
	}
	if err := svcCtx.CredentialSyncModel.Upsert(ctx, syncRecord); err != nil {
		return nil, err
	}
	return mapping, nil
}

func jenkinsCredentialType(credentialType string) string {
	switch strings.TrimSpace(credentialType) {
	case "token", "json":
		return "secret_text"
	default:
		return credentialType
	}
}

func latestJenkinsCredentialID(ctx context.Context, svcCtx *svc.ServiceContext, projectID, buildChannelBindingID, credentialID string) (string, bool, error) {
	record, err := svcCtx.CredentialSyncModel.FindOne(ctx, projectID, buildChannelBindingID, credentialID)
	if err != nil {
		if err == model.ErrNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	if record.SyncStatus != "synced" || strings.TrimSpace(record.JenkinsCredentialID) == "" {
		return "", false, nil
	}
	return record.JenkinsCredentialID, true, nil
}

func stableJenkinsCredentialID(projectCode, credentialCode string) string {
	projectCode = sanitizeJenkinsCode(projectCode)
	credentialCode = sanitizeJenkinsCode(credentialCode)
	if projectCode == "" {
		projectCode = "project"
	}
	if credentialCode == "" {
		credentialCode = "credential"
	}
	return "kn-" + projectCode + "-" + credentialCode
}

func jenkinsFolderName(projectName, projectCode string) string {
	folder := sanitizeJenkinsFolder(projectName)
	if folder == "" {
		folder = sanitizeJenkinsFolder(projectCode)
	}
	if folder == "" {
		return "default-project"
	}
	return folder
}

func sanitizeJenkinsFolder(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, "\\", "-")
	return value
}

func sanitizeJenkinsCode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = jenkinsCredentialCodePattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-_")
	return value
}

func uniqueNonEmptyStrings(items []string) []string {
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
