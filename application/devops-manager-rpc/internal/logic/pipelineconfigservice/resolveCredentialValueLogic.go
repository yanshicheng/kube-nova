package pipelineconfigservicelogic

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveCredentialValueLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveCredentialValueLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveCredentialValueLogic {
	return &ResolveCredentialValueLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResolveCredentialValueLogic) ResolveCredentialValue(in *pb.ResolveCredentialValueReq) (*pb.ResolveCredentialValueResp, error) {
	if err := ensureProjectAccess(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("解析凭证值失败: %v", err)
		return nil, err
	}
	credentialID := strings.TrimSpace(in.CredentialId)
	if credentialID == "" {
		l.Errorf("解析凭证值失败：请选择凭证")
		return nil, errorx.Msg("请选择凭证")
	}
	credential, err := l.svcCtx.CredentialModel.FindOne(l.ctx, credentialID)
	if err != nil {
		if err == model.ErrInvalidObjectID {
			l.Errorf("解析凭证值失败：凭证 ID 格式错误，projectId: %s, credentialType: %s, mappingField: %s, credentialIdLength: %d",
				in.ProjectId, in.CredentialType, in.MappingField, len(credentialID))
			return nil, errorx.Msg("凭证 ID 格式错误，请重新选择凭证")
		}
		l.Errorf("查询凭证失败: %v", err)
		return nil, errorx.Msg("查询凭证失败")
	}
	if credential.Status != 1 {
		l.Errorf("解析凭证值失败：凭证已停用，credentialId: %s", credentialID)
		return nil, errorx.Msg("凭证已停用")
	}
	if in.CredentialType != "" && credential.CredentialType != in.CredentialType {
		l.Errorf("解析凭证值失败：凭证类型不匹配，credentialId: %s, expect: %s, actual: %s",
			credentialID, in.CredentialType, credential.CredentialType)
		return nil, errorx.Msg("凭证类型不匹配")
	}
	scope := normalizeCredentialScope(credential.Scope, credential.ProjectID, credential.IsSystem)
	if scope == "project" && credential.ProjectID != in.ProjectId {
		l.Errorf("解析凭证值失败：凭证不属于当前项目，credentialId: %s, projectId: %s, credentialProjectId: %s",
			credentialID, in.ProjectId, credential.ProjectID)
		return nil, errorx.Msg("凭证不属于当前项目")
	}
	if !isSuperAdminRole(in.CurrentRoles) && scope != "project" {
		l.Errorf("解析凭证值失败：无权使用系统级凭证，credentialId: %s", credentialID)
		return nil, errorx.Msg("无权使用系统级凭证")
	}
	credentialMode := normalizeCredentialMode(in.CredentialMode, in.MappingField)
	if credentialMode == credentialModeJenkinsID {
		if !isJenkinsCredentialIDSupportedType(credential.CredentialType) {
			l.Errorf("解析凭证值失败：该凭证类型不支持 Jenkins credentialId 模式，credentialType: %s", credential.CredentialType)
			return nil, errorx.Msg("该凭证类型不支持 Jenkins credentialId 模式")
		}
		buildChannelBindingID := strings.TrimSpace(in.BuildChannelBindingId)
		if buildChannelBindingID == "" {
			l.Errorf("解析凭证值失败：生成 Jenkins credentialId 必须选择构建渠道")
			return nil, errorx.Msg("生成 Jenkins credentialId 必须选择构建渠道")
		}
		if value, ok, err := latestJenkinsCredentialID(l.ctx, l.svcCtx, in.ProjectId, buildChannelBindingID, credentialID); err != nil {
			l.Errorf("查询 Jenkins 凭证同步记录失败: %v", err)
			return nil, errorx.Msg("查询 Jenkins 凭证同步记录失败")
		} else if ok {
			return &pb.ResolveCredentialValueResp{Value: value}, nil
		}
		data, err := syncJenkinsCredentials(l.ctx, l.svcCtx, &pb.SyncJenkinsCredentialsReq{
			ProjectId:             in.ProjectId,
			BuildChannelBindingId: buildChannelBindingID,
			CredentialIds:         []string{credentialID},
			CurrentUserId:         in.CurrentUserId,
			CurrentRoles:          in.CurrentRoles,
		})
		if err != nil {
			l.Errorf("同步 Jenkins 凭证失败: %v", err)
			return nil, err
		}
		if len(data) == 0 || strings.TrimSpace(data[0].JenkinsCredentialId) == "" {
			l.Errorf("解析凭证值失败：Jenkins 凭证同步结果为空，credentialId: %s", credentialID)
			return nil, errorx.Msg("Jenkins 凭证同步结果为空")
		}
		return &pb.ResolveCredentialValueResp{Value: strings.TrimSpace(data[0].JenkinsCredentialId)}, nil
	}
	secret, err := credential.Secret()
	if err != nil {
		l.Errorf("解密凭证失败: %v", err)
		return nil, errorx.Msg("解密凭证失败")
	}
	field := strings.TrimSpace(in.MappingField)
	if credentialMode == credentialModeJSONData || credentialMode == credentialModeJSONBase64 {
		field = credentialMode
	}
	if !credentialSupportsField(credential.CredentialType, field) {
		l.Errorf("解析凭证值失败：凭证映射字段不支持，credentialType: %s, mappingField: %s",
			credential.CredentialType, field)
		return nil, errorx.Msg("凭证映射字段不支持")
	}
	value, ok := channelCredentialSecretField(secret, credential.CredentialType, field)
	if !ok {
		l.Errorf("解析凭证值失败：凭证映射字段不支持，credentialType: %s, mappingField: %s",
			credential.CredentialType, field)
		return nil, errorx.Msg("凭证映射字段不支持")
	}

	return &pb.ResolveCredentialValueResp{Value: value}, nil
}

func isJenkinsCredentialIDSupportedType(credentialType string) bool {
	switch strings.TrimSpace(credentialType) {
	case "username_password", "token", "secret_text", "ssh_key", "kubeconfig", "json":
		return true
	default:
		return false
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

func credentialSecretField(secret model.CredentialSecret, field string) (string, bool) {
	switch field {
	case "username":
		return secret.Username, true
	case "password":
		return secret.Password, true
	case "token":
		return secret.Token, true
	case "privateKey":
		return secret.PrivateKey, true
	case "passphrase":
		return secret.Passphrase, true
	case "kubeconfig":
		return secret.Kubeconfig, true
	case "secretText":
		return secret.SecretText, true
	case "certificate":
		return secret.Certificate, true
	case "jsonData":
		return secret.JsonData, true
	case "jsonBase64":
		return base64.StdEncoding.EncodeToString([]byte(secret.JsonData)), true
	default:
		return "", false
	}
}
