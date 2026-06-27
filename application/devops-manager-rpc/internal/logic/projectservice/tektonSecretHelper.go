package projectservicelogic

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/tektonsync"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	tektonSecretNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)
	tektonSecretKeyPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

type tektonSecretBindingRef struct {
	Binding       *model.DevopsProjectChannelBinding
	Channel       *model.DevopsChannel
	BindingConfig tektonsync.BindingConfig
	TaskNamespace string
	Client        *devopstekton.Client
}

func listTektonSecretBindingRefs(ctx context.Context, svcCtx *svc.ServiceContext, projectID string, userID uint64, roles []string) ([]tektonSecretBindingRef, error) {
	if err := ensureProjectAccess(ctx, svcCtx, projectID, userID, roles); err != nil {
		return nil, err
	}
	bindings, _, err := svcCtx.ProjectChannelModel.List(ctx, model.DevopsProjectChannelBindingListFilter{
		ProjectID:        strings.TrimSpace(projectID),
		ChannelGroupCode: model.BuildChannelGroupCode,
		ChannelType:      tektonsync.EngineTypeTekton,
		Status:           1,
		Page:             1,
		PageSize:         200,
	})
	if err != nil {
		logx.Errorf("查询项目 Tekton 渠道绑定失败: %v", err)
		return nil, err
	}
	result := make([]tektonSecretBindingRef, 0, len(bindings))
	for _, binding := range bindings {
		ref, err := buildTektonSecretBindingRef(ctx, svcCtx, projectID, binding)
		if err != nil {
			logx.Errorf("跳过不可用 Tekton Secret 渠道绑定: binding=%s err=%v", binding.ID.Hex(), err)
			continue
		}
		result = append(result, ref)
	}
	return result, nil
}

func resolveTektonSecretBinding(ctx context.Context, svcCtx *svc.ServiceContext, projectID, bindingID string, userID uint64, roles []string) (tektonSecretBindingRef, error) {
	if err := ensureProjectAccess(ctx, svcCtx, projectID, userID, roles); err != nil {
		return tektonSecretBindingRef{}, err
	}
	bindingID = strings.TrimSpace(bindingID)
	if bindingID == "" {
		return tektonSecretBindingRef{}, errorx.Msg("请选择 Tekton 渠道")
	}
	binding, err := svcCtx.ProjectChannelModel.FindOne(ctx, bindingID)
	if err != nil {
		logx.Errorf("查询项目 Tekton 渠道绑定失败: bindingId=%s err=%v", bindingID, err)
		return tektonSecretBindingRef{}, err
	}
	return buildTektonSecretBindingRef(ctx, svcCtx, projectID, binding)
}

func buildTektonSecretBindingRef(ctx context.Context, svcCtx *svc.ServiceContext, projectID string, binding *model.DevopsProjectChannelBinding) (tektonSecretBindingRef, error) {
	if binding == nil || binding.ProjectID != strings.TrimSpace(projectID) {
		return tektonSecretBindingRef{}, errorx.Msg("Tekton 渠道不属于当前项目")
	}
	if binding.Status != 1 || binding.ChannelType != tektonsync.EngineTypeTekton {
		return tektonSecretBindingRef{}, errorx.Msg("请选择启用的 Tekton 渠道")
	}
	channel, err := svcCtx.ChannelModel.FindOne(ctx, binding.ChannelID)
	if err != nil {
		logx.Errorf("查询 Tekton 渠道实例失败: channelId=%s err=%v", binding.ChannelID, err)
		return tektonSecretBindingRef{}, err
	}
	if channel.Status != 1 || channel.ChannelType != tektonsync.EngineTypeTekton {
		return tektonSecretBindingRef{}, errorx.Msg("请选择启用的 Tekton 渠道")
	}
	cfg, err := tektonsync.ParseBindingConfig(binding.BindingConfig)
	if err != nil {
		logx.Errorf("解析 Tekton 项目绑定配置失败: binding=%s err=%v", binding.ID.Hex(), err)
		return tektonSecretBindingRef{}, err
	}
	channelCfg, err := devopstekton.ParseChannelConfig(channel.Config)
	if err != nil {
		logx.Errorf("解析 Tekton 渠道配置失败: channel=%s err=%v", channel.Code, err)
		return tektonSecretBindingRef{}, errorx.Msg(err.Error())
	}
	client, err := tektonsync.NewSyncer(svcCtx).ClientFromChannel(ctx, channel)
	if err != nil {
		logx.Errorf("创建 Tekton 客户端失败: channel=%s err=%v", channel.Code, err)
		return tektonSecretBindingRef{}, err
	}
	if err := client.EnsureNamespace(ctx, cfg.Namespace); err != nil {
		logx.Errorf("Tekton Secret namespace 创建或校验失败: namespace=%s err=%v", cfg.Namespace, err)
		return tektonSecretBindingRef{}, errorx.Msg(err.Error())
	}
	return tektonSecretBindingRef{
		Binding:       binding,
		Channel:       channel,
		BindingConfig: cfg,
		TaskNamespace: channelCfg.TaskNamespace,
		Client:        client,
	}, nil
}

func tektonSecretBindingOption(ref tektonSecretBindingRef) *pb.TektonSecretBindingOption {
	return &pb.TektonSecretBindingOption{
		Id:                 ref.Binding.ID.Hex(),
		ProjectId:          ref.Binding.ProjectID,
		ChannelId:          ref.Binding.ChannelID,
		ChannelName:        ref.Channel.Name,
		ChannelCode:        ref.Channel.Code,
		Namespace:          ref.BindingConfig.Namespace,
		TaskNamespace:      ref.TaskNamespace,
		ServiceAccountName: ref.BindingConfig.ServiceAccountName,
		Status:             ref.Binding.Status,
		HealthStatus:       ref.Binding.HealthStatus,
	}
}

func tektonSecretInfoToPb(info devopstekton.SecretInfo, ref tektonSecretBindingRef) *pb.TektonSecret {
	return &pb.TektonSecret{
		Name:              info.Name,
		Type:              info.Type,
		Namespace:         info.Namespace,
		BindingId:         ref.Binding.ID.Hex(),
		ChannelName:       ref.Channel.Name,
		Keys:              info.Keys,
		ManagedByKubeNova: info.ManagedByKubeNova,
		CreatedAt:         info.CreatedAt,
	}
}

func tektonPVCInfoToPb(info devopstekton.PVCInfo, ref tektonSecretBindingRef) *pb.TektonPVC {
	return &pb.TektonPVC{
		Name:             info.Name,
		Namespace:        info.Namespace,
		BindingId:        ref.Binding.ID.Hex(),
		ChannelName:      ref.Channel.Name,
		StorageClassName: info.StorageClassName,
		Storage:          info.Storage,
		AccessModes:      info.AccessModes,
		VolumeMode:       info.VolumeMode,
		Status:           info.Status,
		CreatedAt:        info.CreatedAt,
	}
}

func tektonSecretToPb(secret *corev1.Secret, ref tektonSecretBindingRef, includeData bool) *pb.TektonSecret {
	if secret == nil {
		return nil
	}
	keys := make([]string, 0, len(secret.Data))
	data := make(map[string]string, len(secret.Data))
	for key, value := range secret.Data {
		keys = append(keys, key)
		if includeData {
			data[key] = string(value)
		}
	}
	sort.Strings(keys)
	if !includeData {
		data = nil
	}
	return &pb.TektonSecret{
		Name:              secret.Name,
		Type:              string(secret.Type),
		Namespace:         secret.Namespace,
		BindingId:         ref.Binding.ID.Hex(),
		ChannelName:       ref.Channel.Name,
		Keys:              keys,
		Data:              data,
		ManagedByKubeNova: secret.Labels["kube-nova.io/managed-by"] == "kube-nova",
		CreatedAt:         secret.CreationTimestamp.Unix(),
	}
}

func buildKubernetesSecret(req *pb.SaveTektonSecretReq, namespace string) (*corev1.Secret, error) {
	name := strings.TrimSpace(req.Name)
	if !tektonSecretNamePattern.MatchString(name) || len(name) > 253 {
		return nil, errorx.Msg("Secret 名称只能包含小写字母、数字、中划线和点，且首尾必须是字母或数字")
	}
	secretType, err := normalizeKubernetesSecretType(req.Type)
	if err != nil {
		return nil, err
	}
	data, err := normalizeKubernetesSecretData(secretType, req.Data)
	if err != nil {
		return nil, err
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"kube-nova.io/devops-project-id": req.ProjectId,
				"kube-nova.io/devops-binding-id": req.BindingId,
			},
			Annotations: map[string]string{
				"kube-nova.io/updated-by": strings.TrimSpace(req.Operator),
			},
		},
		Type:      secretType,
		Data:      data,
		Immutable: nil,
	}, nil
}

func tektonSecretClientError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return err
	}
	switch {
	case strings.Contains(msg, "Kubernetes Secret 不存在"),
		strings.Contains(msg, "Kubernetes Secret 已存在"),
		strings.Contains(msg, "Kubernetes Secret 参数不完整"),
		strings.Contains(msg, "Tekton namespace 不能为空"),
		strings.Contains(msg, "不属于当前项目 Tekton 渠道"):
		return errorx.Msg(msg)
	default:
		return err
	}
}

func normalizeKubernetesSecretType(raw string) (corev1.SecretType, error) {
	switch strings.TrimSpace(raw) {
	case "", "Opaque", "opaque":
		return corev1.SecretTypeOpaque, nil
	case "kubernetes.io/basic-auth", "basic-auth":
		return corev1.SecretTypeBasicAuth, nil
	case "kubernetes.io/ssh-auth", "ssh", "ssh-auth":
		return corev1.SecretTypeSSHAuth, nil
	case "kubernetes.io/dockerconfigjson", "dockerconfigjson", "docker-config-json":
		return corev1.SecretTypeDockerConfigJson, nil
	default:
		return "", errorx.Msg("不支持的 Kubernetes Secret 类型")
	}
}

func normalizeKubernetesSecretData(secretType corev1.SecretType, input map[string]string) (map[string][]byte, error) {
	if len(input) == 0 {
		return nil, errorx.Msg("Secret 数据不能为空")
	}
	data := make(map[string][]byte, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		if key == "" || !tektonSecretKeyPattern.MatchString(key) {
			return nil, errorx.Msg("Secret 数据键只能包含字母、数字、点、下划线和中划线")
		}
		data[key] = []byte(value)
	}
	for _, key := range requiredSecretKeys(secretType) {
		if strings.TrimSpace(string(data[key])) == "" {
			return nil, errorx.Msg("Secret 类型缺少必要数据键: " + key)
		}
	}
	if secretType == corev1.SecretTypeDockerConfigJson && !json.Valid(data[corev1.DockerConfigJsonKey]) {
		return nil, errorx.Msg("Docker configjson 必须是合法 JSON")
	}
	return data, nil
}

func requiredSecretKeys(secretType corev1.SecretType) []string {
	switch secretType {
	case corev1.SecretTypeBasicAuth:
		return []string{corev1.BasicAuthUsernameKey, corev1.BasicAuthPasswordKey}
	case corev1.SecretTypeSSHAuth:
		return []string{corev1.SSHAuthPrivateKey}
	case corev1.SecretTypeDockerConfigJson:
		return []string{corev1.DockerConfigJsonKey}
	default:
		return nil
	}
}

func copyKubernetesSecretForApply(source *corev1.Secret) *corev1.Secret {
	labels := make(map[string]string, len(source.Labels))
	for key, value := range source.Labels {
		labels[key] = value
	}
	annotations := make(map[string]string, len(source.Annotations))
	for key, value := range source.Annotations {
		annotations[key] = value
	}
	data := make(map[string][]byte, len(source.Data))
	for key, value := range source.Data {
		copied := make([]byte, len(value))
		copy(copied, value)
		data[key] = copied
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        source.Name,
			Labels:      labels,
			Annotations: annotations,
		},
		Type: source.Type,
		Data: data,
	}
}
