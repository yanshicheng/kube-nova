package executionservicelogic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var tektonSecretDataKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type tektonResourceBindingRecord struct {
	WorkspaceName    string `json:"workspaceName"`
	BindingType      string `json:"bindingType"`
	SourceType       string `json:"sourceType"`
	ResourceID       string `json:"resourceId"`
	ResourceName     string `json:"resourceName"`
	SecretName       string `json:"secretName"`
	Namespace        string `json:"namespace"`
	ChannelBindingID string `json:"channelBindingId"`
}

func ensureTektonWorkspaceBindings(ctx context.Context, svcCtx *svc.ServiceContext, client *devopstekton.Client, namespace string, pipeline *model.DevopsPipeline, runtimeParams map[string]string, renderCtx pipelineRenderContext) error {
	if err := ensureTektonSecretWorkspaces(ctx, svcCtx, client, namespace, pipeline, runtimeParams, renderCtx); err != nil {
		return err
	}
	if err := ensureTektonPVCWorkspaces(ctx, client, namespace, pipeline, runtimeParams); err != nil {
		return err
	}
	if err := ensureTektonDagGeneratedSecretWorkspaces(ctx, svcCtx, client, namespace, pipeline, runtimeParams, renderCtx); err != nil {
		return err
	}
	return ensureTektonDagWorkspaceBindings(ctx, client, namespace, pipeline, runtimeParams)
}

func ensureTektonSecretWorkspaces(ctx context.Context, svcCtx *svc.ServiceContext, client *devopstekton.Client, namespace string, pipeline *model.DevopsPipeline, runtimeParams map[string]string, renderCtx pipelineRenderContext) error {
	if pipeline == nil {
		return nil
	}
	enforceRequired := runtimeParams != nil
	enabledSteps := tektonEnabledStepIDs(pipeline.Steps)
	checkedExternalSecrets := map[string]struct{}{}
	syncedManagedSecrets := map[string]struct{}{}
	for _, item := range pipeline.Params {
		if item.ParamType != channelvars.ParamKubernetesSecretName {
			continue
		}
		config := normalizePipelineParamConfig(item)
		if strings.TrimSpace(config.MappingField) == "" {
			continue
		}
		if !tektonParamStepEnabled(item, enabledSteps) {
			continue
		}
		workspaceRequired := tektonSecretParamWorkspaceRequired(pipeline, item)
		if strings.TrimSpace(config.VoucherModel) != "" {
			secretName := tektonWorkspaceSecretName(pipeline, item, runtimeParams)
			if secretName == "" {
				if enforceRequired && workspaceRequired {
					return errorx.Msg("Kubernetes Secret 参数不能为空：" + displayPipelineParamName(item))
				}
				continue
			}
			if _, ok := syncedManagedSecrets[secretName]; ok {
				continue
			}
			secret, err := buildTektonWorkspaceSecret(ctx, svcCtx, namespace, pipeline, item, runtimeParams, renderCtx)
			if err != nil {
				logx.Errorf("构建 Tekton Workspace Secret 失败: pipeline=%s param=%s err=%v", pipeline.Code, item.Code, err)
				return err
			}
			if err := client.ApplySecret(ctx, namespace, secret); err != nil {
				logx.Errorf("同步 Tekton Workspace Secret 失败: namespace=%s secret=%s err=%v", namespace, secretName, err)
				return errorx.Msg("同步 Kubernetes Secret 失败")
			}
			syncedManagedSecrets[secretName] = struct{}{}
			continue
		}
		secretName := tektonPipelineParamValue(item, runtimeParams)
		if secretName == "" {
			if enforceRequired && workspaceRequired {
				return errorx.Msg("Kubernetes Secret 参数不能为空：" + displayPipelineParamName(item))
			}
			continue
		}
		if _, ok := checkedExternalSecrets[secretName]; ok {
			continue
		}
		checkedExternalSecrets[secretName] = struct{}{}
		if err := client.SecretExists(ctx, namespace, secretName); err != nil {
			logx.Errorf("Tekton Secret 校验失败: namespace=%s secret=%s err=%v", namespace, secretName, err)
			return errorx.Msg(err.Error())
		}
	}
	return nil
}

func ensureTektonPVCWorkspaces(ctx context.Context, client *devopstekton.Client, namespace string, pipeline *model.DevopsPipeline, runtimeParams map[string]string) error {
	if pipeline == nil {
		return nil
	}
	enforceRequired := runtimeParams != nil
	enabledSteps := tektonEnabledStepIDs(pipeline.Steps)
	checkedPVCs := map[string]struct{}{}
	for _, item := range pipeline.Params {
		config := normalizePipelineParamConfig(item)
		if strings.TrimSpace(config.MappingField) == "" {
			continue
		}
		if !tektonParamStepEnabled(item, enabledSteps) {
			continue
		}
		switch tektonWorkspaceBindingKind(item, config) {
		case "persistentVolumeClaim":
			pvcName := tektonPipelineParamValue(item, runtimeParams)
			workspaceRequired := tektonWorkspaceParamRequired(pipeline, item)
			if pvcName == "" {
				if enforceRequired && workspaceRequired {
					return errorx.Msg("Kubernetes PVC 参数不能为空：" + displayPipelineParamName(item))
				}
				continue
			}
			if _, ok := checkedPVCs[pvcName]; ok {
				continue
			}
			checkedPVCs[pvcName] = struct{}{}
			if err := client.PersistentVolumeClaimExists(ctx, namespace, pvcName); err != nil {
				logx.Errorf("Tekton PVC 校验失败: namespace=%s pvc=%s err=%v", namespace, pvcName, err)
				return errorx.Msg(err.Error())
			}
		case "volumeClaimTemplate":
			if err := validateTektonVolumeClaimTemplateParam(pipeline, item, runtimeParams, enforceRequired); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateTektonVolumeClaimTemplateParam(pipeline *model.DevopsPipeline, item model.PipelineParam, runtimeParams map[string]string, enforceRequired bool) error {
	config := normalizePipelineParamConfig(item)
	field, ok := tektonVolumeClaimTemplateField(config.Provider)
	if !ok {
		return nil
	}
	value := strings.TrimSpace(tektonPipelineParamValue(item, runtimeParams))
	workspaceRequired := tektonWorkspaceParamRequired(pipeline, item)
	if field == "storage" {
		if value == "" {
			if enforceRequired && workspaceRequired {
				return errorx.Msg("动态 PVC 存储容量不能为空：" + displayPipelineParamName(item))
			}
			return nil
		}
		if _, err := resource.ParseQuantity(value); err != nil {
			logx.Errorf("动态 PVC 存储容量不合法: param=%s value=%s err=%v", item.Code, value, err)
			return errorx.Msg("动态 PVC 存储容量不合法：" + displayPipelineParamName(item))
		}
	}
	if field == "accessMode" && value != "" && !isSupportedTektonAccessMode(value) {
		return errorx.Msg("动态 PVC accessMode 不支持：" + displayPipelineParamName(item))
	}
	if field == "volumeMode" && value != "" && value != "Filesystem" && value != "Block" {
		return errorx.Msg("动态 PVC volumeMode 不支持：" + displayPipelineParamName(item))
	}
	return nil
}

func ensureTektonDagWorkspaceBindings(ctx context.Context, client *devopstekton.Client, namespace string, pipeline *model.DevopsPipeline, runtimeParams map[string]string) error {
	if pipeline == nil || !hasTektonDagConfig(pipeline.TektonDagConfig) {
		return nil
	}
	workspaces, err := tektonDagPipelineRunWorkspaces(pipeline, runtimeParams)
	if err != nil {
		return err
	}
	checkedSecret := map[string]struct{}{}
	checkedPVC := map[string]struct{}{}
	checkedConfigMap := map[string]struct{}{}
	checkedStorageClass := map[string]struct{}{}
	for _, workspace := range workspaces {
		if secretName := tektonWorkspaceSecretNameFromRunBinding(workspace); secretName != "" {
			if _, ok := checkedSecret[secretName]; !ok {
				checkedSecret[secretName] = struct{}{}
				if err := client.SecretExists(ctx, namespace, secretName); err != nil {
					logx.Errorf("Tekton DAG Secret Workspace 校验失败: namespace=%s secret=%s err=%v", namespace, secretName, err)
					return errorx.Msg(err.Error())
				}
			}
		}
		if pvcName := tektonWorkspacePVCNameFromRunBinding(workspace); pvcName != "" {
			if _, ok := checkedPVC[pvcName]; !ok {
				checkedPVC[pvcName] = struct{}{}
				if err := client.PersistentVolumeClaimExists(ctx, namespace, pvcName); err != nil {
					logx.Errorf("Tekton DAG PVC Workspace 校验失败: namespace=%s pvc=%s err=%v", namespace, pvcName, err)
					return errorx.Msg(err.Error())
				}
			}
		}
		if configMapName := tektonWorkspaceConfigMapNameFromRunBinding(workspace); configMapName != "" {
			if _, ok := checkedConfigMap[configMapName]; !ok {
				checkedConfigMap[configMapName] = struct{}{}
				if err := client.ConfigMapExists(ctx, namespace, configMapName); err != nil {
					logx.Errorf("Tekton DAG ConfigMap Workspace 校验失败: namespace=%s configMap=%s err=%v", namespace, configMapName, err)
					return errorx.Msg(err.Error())
				}
			}
		}
		if storageClassName := tektonWorkspaceStorageClassFromRunBinding(workspace); storageClassName != "" {
			if _, ok := checkedStorageClass[storageClassName]; !ok {
				checkedStorageClass[storageClassName] = struct{}{}
				if err := client.StorageClassExists(ctx, storageClassName); err != nil {
					logx.Errorf("Tekton DAG 动态 PVC StorageClass 校验失败: storageClass=%s err=%v", storageClassName, err)
					return errorx.Msg(err.Error())
				}
			}
		}
	}
	return nil
}

func ensureTektonDagGeneratedSecretWorkspaces(ctx context.Context, svcCtx *svc.ServiceContext, client *devopstekton.Client, namespace string, pipeline *model.DevopsPipeline, runtimeParams map[string]string, renderCtx pipelineRenderContext) error {
	if pipeline == nil || !hasTektonDagConfig(pipeline.TektonDagConfig) {
		return nil
	}
	items, err := tektonDagWorkspaceConfigItems(pipeline)
	if err != nil {
		return err
	}
	records := make([]tektonResourceBindingRecord, 0)
	for _, item := range items {
		if strings.TrimSpace(firstNotBlank(item.BindingType, item.Strategy, item.Type)) != "generatedSecret" {
			continue
		}
		secretName := firstNotBlank(item.SecretName, tektonGeneratedWorkspaceSecretName(pipeline, item))
		if secretName == "" {
			return errorx.Msg("生成 Secret Workspace 名称不能为空：" + item.Name)
		}
		resourceID := tektonGeneratedWorkspaceResourceID(item, runtimeParams)
		if resourceID == "" {
			if item.Optional {
				continue
			}
			return errorx.Msg("生成 Secret Workspace 必须选择平台资源：" + item.Name)
		}
		secret, record, err := buildTektonDagGeneratedSecret(ctx, svcCtx, namespace, pipeline, item, resourceID, secretName, renderCtx)
		if err != nil {
			logx.Errorf("构建 Tekton DAG Workspace Secret 失败: pipeline=%s workspace=%s err=%v", pipeline.Code, item.Name, err)
			return err
		}
		if err := client.ApplySecret(ctx, namespace, secret); err != nil {
			logx.Errorf("同步 Tekton DAG Workspace Secret 失败: namespace=%s secret=%s err=%v", namespace, secretName, err)
			return errorx.Msg("同步 Kubernetes Secret 失败")
		}
		records = append(records, record)
	}
	if len(records) > 0 {
		content, err := json.Marshal(records)
		if err != nil {
			return err
		}
		pipeline.TektonResourceBindings = string(content)
	}
	return nil
}

func tektonDagWorkspaceConfigItems(pipeline *model.DevopsPipeline) ([]tektonDagWorkspace, error) {
	cfg, err := parseTektonDagConfig(pipeline.TektonDagConfig)
	if err != nil {
		return nil, err
	}
	items := append([]tektonDagWorkspace{}, cfg.Pipeline.Workspaces...)
	items = append(items, cfg.Workspaces...)
	policy, err := tektonRunPolicyFromPipeline(pipeline)
	if err != nil {
		return nil, err
	}
	items = append(items, policy.Workspaces...)
	return items, nil
}

func tektonGeneratedWorkspaceResourceID(item tektonDagWorkspace, runtimeParams map[string]string) string {
	name := strings.TrimSpace(item.Name)
	if item.RuntimeConfig && runtimeParams != nil {
		if value := strings.TrimSpace(runtimeParams[name]); value != "" && isLikelyObjectID(value) {
			return value
		}
	}
	return strings.TrimSpace(item.ResourceID)
}

func tektonGeneratedWorkspaceSecretName(pipeline *model.DevopsPipeline, item tektonDagWorkspace) string {
	pipelineKey := ""
	if pipeline != nil {
		pipelineKey = firstNotBlank(pipeline.ID.Hex(), pipeline.Code)
	}
	return tektonResourceName("workspace-secret", pipelineKey, item.Name)
}

func buildTektonDagGeneratedSecret(ctx context.Context, svcCtx *svc.ServiceContext, namespace string, pipeline *model.DevopsPipeline, item tektonDagWorkspace, resourceID, secretName string, renderCtx pipelineRenderContext) (*corev1.Secret, tektonResourceBindingRecord, error) {
	sourceType := strings.TrimSpace(firstNotBlank(item.SourceType, item.PlatformType))
	if sourceType == "" {
		sourceType = "platformCredential"
	}
	secretType := corev1.SecretTypeOpaque
	data := map[string][]byte{}
	resourceName := strings.TrimSpace(item.ResourceName)
	switch sourceType {
	case "platformCredential", "credential", "platformResource":
		nextType, nextData, err := tektonGeneratedSecretCredentialData(ctx, svcCtx, pipeline, item, resourceID, renderCtx)
		if err != nil {
			return nil, tektonResourceBindingRecord{}, err
		}
		secretType = nextType
		data = nextData
	case "configCenter":
		nextData, nextName, err := tektonGeneratedSecretConfigCenterData(ctx, svcCtx, pipeline, item, resourceID, renderCtx)
		if err != nil {
			return nil, tektonResourceBindingRecord{}, err
		}
		data = nextData
		if resourceName == "" {
			resourceName = nextName
		}
	default:
		return nil, tektonResourceBindingRecord{}, errorx.Msg("生成 Secret 来源类型不支持：" + sourceType)
	}
	if len(data) == 0 {
		return nil, tektonResourceBindingRecord{}, errorx.Msg("生成 Secret 数据不能为空：" + item.Name)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"kube-nova.io/managed-by":        "kube-nova",
				"kube-nova.io/devops-project-id": pipeline.ProjectID,
				"kube-nova.io/pipeline-id":       pipeline.ID.Hex(),
				"kube-nova.io/workspace-name":    item.Name,
				"kube-nova.io/source-type":       sourceType,
			},
			Annotations: map[string]string{
				"kube-nova.io/source-resource-id":   resourceID,
				"kube-nova.io/source-resource-name": resourceName,
			},
		},
		Type: secretType,
		Data: data,
	}
	record := tektonResourceBindingRecord{
		WorkspaceName:    item.Name,
		BindingType:      "generatedSecret",
		SourceType:       sourceType,
		ResourceID:       resourceID,
		ResourceName:     resourceName,
		SecretName:       secretName,
		Namespace:        namespace,
		ChannelBindingID: pipeline.BuildChannelBindingID,
	}
	return secret, record, nil
}

func tektonGeneratedSecretCredentialData(ctx context.Context, svcCtx *svc.ServiceContext, pipeline *model.DevopsPipeline, item tektonDagWorkspace, credentialID string, renderCtx pipelineRenderContext) (corev1.SecretType, map[string][]byte, error) {
	credentialType := firstNotBlank(
		stringJSONValue(item.Config, "credentialType"),
		stringJSONValue(item.Config, "voucherModel"),
		item.PlatformType,
	)
	if credentialType == "" {
		return "", nil, errorx.Msg("生成 Secret 必须配置凭证类型：" + item.Name)
	}
	provider := normalizeTektonSecretProvider(stringJSONValue(item.Config, "provider"), credentialType)
	fields, err := tektonSecretFields(provider, credentialType)
	if err != nil {
		return "", nil, err
	}
	values := make(map[string]string, len(fields))
	for _, field := range fields {
		resp, err := svcCtx.PipelineConfigRpc.ResolveCredentialValue(ctx, &pipelineconfigservice.ResolveCredentialValueReq{
			ProjectId:             pipeline.ProjectID,
			CredentialId:          credentialID,
			CredentialType:        credentialType,
			MappingField:          field,
			CredentialMode:        "field",
			CurrentUserId:         renderCtx.UserID,
			CurrentRoles:          renderCtx.Roles,
			BuildChannelBindingId: pipeline.BuildChannelBindingID,
		})
		if err != nil {
			logx.Errorf("解析 Workspace 生成 Secret 凭证字段失败: pipeline=%s workspace=%s field=%s err=%v", pipeline.Code, item.Name, field, err)
			return "", nil, err
		}
		values[field] = strings.TrimSpace(resp.GetValue())
	}
	return tektonSecretDataFromResolvedCredential(provider, values)
}

func tektonSecretDataFromResolvedCredential(provider string, values map[string]string) (corev1.SecretType, map[string][]byte, error) {
	secretType := corev1.SecretTypeOpaque
	data := map[string][]byte{}
	switch provider {
	case "basic-auth":
		if values["username"] == "" || values["password"] == "" {
			return "", nil, errorx.Msg("用户名密码凭证不完整")
		}
		secretType = corev1.SecretTypeBasicAuth
		data["username"] = []byte(values["username"])
		data["password"] = []byte(values["password"])
	case "ssh":
		if values["privateKey"] == "" {
			return "", nil, errorx.Msg("SSH Key 凭证不完整")
		}
		secretType = corev1.SecretTypeSSHAuth
		data["id_rsa"] = []byte(values["privateKey"])
		data["ssh-privatekey"] = []byte(values["privateKey"])
		if values["username"] != "" {
			data["username"] = []byte(values["username"])
		}
		if values["passphrase"] != "" {
			data["passphrase"] = []byte(values["passphrase"])
		}
	case "dockerconfigjson":
		if values["secretText"] == "" {
			return "", nil, errorx.Msg("Docker configjson 必须使用 Secret Text 凭证")
		}
		secretType = corev1.SecretTypeDockerConfigJson
		data[corev1.DockerConfigJsonKey] = []byte(values["secretText"])
	default:
		for key, value := range values {
			if strings.TrimSpace(value) != "" {
				data[key] = []byte(value)
			}
		}
	}
	return secretType, data, nil
}

func tektonGeneratedSecretConfigCenterData(ctx context.Context, svcCtx *svc.ServiceContext, pipeline *model.DevopsPipeline, item tektonDagWorkspace, configID string, renderCtx pipelineRenderContext) (map[string][]byte, string, error) {
	resp, err := svcCtx.PipelineConfigRpc.ResolveProjectConfig(ctx, &pipelineconfigservice.ResolveProjectConfigReq{
		ProjectId:     pipeline.ProjectID,
		ConfigId:      configID,
		TypeId:        firstNotBlank(stringJSONValue(item.Config, "configTypeId"), stringJSONValue(item.Config, "typeId")),
		TypeCode:      firstNotBlank(stringJSONValue(item.Config, "configTypeCode"), stringJSONValue(item.Config, "typeCode")),
		CurrentUserId: renderCtx.UserID,
		CurrentRoles:  renderCtx.Roles,
	})
	if err != nil {
		logx.Errorf("解析 Workspace 生成 Secret 配置中心失败: pipeline=%s workspace=%s err=%v", pipeline.Code, item.Name, err)
		return nil, "", err
	}
	key := firstNotBlank(stringJSONValue(item.Config, "secretKey"), resp.GetCode(), "content")
	if !tektonSecretDataKeyPattern.MatchString(key) {
		return nil, "", errorx.Msg("配置中心生成 Secret 的数据键不合法：" + key)
	}
	content := resp.GetContent()
	if strings.EqualFold(stringJSONValue(item.Config, "encoding"), "base64") {
		content = base64.StdEncoding.EncodeToString([]byte(content))
	}
	return map[string][]byte{key: []byte(content)}, resp.GetName(), nil
}

func tektonWorkspaceSecretNameFromRunBinding(workspace map[string]any) string {
	return tektonNestedString(workspace, "secret", "secretName")
}

func tektonWorkspacePVCNameFromRunBinding(workspace map[string]any) string {
	return tektonNestedString(workspace, "persistentVolumeClaim", "claimName")
}

func tektonWorkspaceConfigMapNameFromRunBinding(workspace map[string]any) string {
	return firstNotBlank(
		tektonNestedString(workspace, "configMap", "name"),
		tektonNestedString(workspace, "configMap", "configMapName"),
	)
}

func tektonWorkspaceStorageClassFromRunBinding(workspace map[string]any) string {
	spec, ok := workspace["volumeClaimTemplate"].(map[string]any)
	if !ok {
		return ""
	}
	return tektonNestedString(spec, "spec", "storageClassName")
}

func tektonNestedString(item map[string]any, keys ...string) string {
	var current any = item
	for _, key := range keys {
		data, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = data[key]
	}
	if current == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(current))
}

func tektonSecretParamWorkspaceRequired(pipeline *model.DevopsPipeline, item model.PipelineParam) bool {
	return tektonWorkspaceParamRequired(pipeline, item)
}

func tektonWorkspaceParamRequired(pipeline *model.DevopsPipeline, item model.PipelineParam) bool {
	if pipeline == nil {
		return true
	}
	workspaceName := strings.TrimSpace(normalizePipelineParamConfig(item).MappingField)
	if workspaceName == "" {
		return true
	}
	stepNodeID := strings.TrimSpace(item.StepNodeID)
	if stepNodeID == "" {
		return true
	}
	for _, step := range pipeline.Steps {
		if strings.TrimSpace(step.ID) != stepNodeID || !step.Enabled {
			continue
		}
		resource, err := devopstekton.ParseStepResource(step.StageContent)
		if err != nil {
			logx.Errorf("解析 Tekton 步骤 workspace 失败: step=%s err=%v", step.StepCode, err)
			return true
		}
		items, ok, err := unstructured.NestedSlice(resource.Object.Object, "spec", "workspaces")
		if err != nil || !ok {
			return true
		}
		for _, raw := range items {
			data, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if strings.TrimSpace(fmt.Sprint(data["name"])) != workspaceName {
				continue
			}
			optional, _ := data["optional"].(bool)
			return !optional
		}
		return true
	}
	return true
}

func tektonEnabledStepIDs(steps []model.PipelineStep) map[string]struct{} {
	result := make(map[string]struct{}, len(steps))
	for _, step := range steps {
		nodeID := strings.TrimSpace(step.ID)
		if !step.Enabled || nodeID == "" {
			continue
		}
		result[nodeID] = struct{}{}
	}
	return result
}

func tektonParamStepEnabled(item model.PipelineParam, enabledSteps map[string]struct{}) bool {
	stepNodeID := strings.TrimSpace(item.StepNodeID)
	if stepNodeID == "" {
		return true
	}
	_, ok := enabledSteps[stepNodeID]
	return ok
}

func buildTektonWorkspaceSecret(ctx context.Context, svcCtx *svc.ServiceContext, namespace string, pipeline *model.DevopsPipeline, item model.PipelineParam, runtimeParams map[string]string, renderCtx pipelineRenderContext) (*corev1.Secret, error) {
	config := normalizePipelineParamConfig(item)
	credentialID := tektonWorkspaceCredentialID(item, runtimeParams)
	if !isLikelyObjectID(credentialID) {
		return nil, errorx.Msg("Kubernetes Secret 参数必须选择平台凭证：" + displayPipelineParamName(item))
	}
	provider := normalizeTektonSecretProvider(config.Provider, config.VoucherModel)
	values, err := resolveTektonSecretCredentialFields(ctx, svcCtx, pipeline, item, credentialID, provider, renderCtx)
	if err != nil {
		return nil, err
	}
	secretType := corev1.SecretTypeOpaque
	data := map[string][]byte{}
	switch provider {
	case "basic-auth":
		if values["username"] == "" || values["password"] == "" {
			return nil, errorx.Msg("用户名密码凭证不完整")
		}
		secretType = corev1.SecretTypeBasicAuth
		data["username"] = []byte(values["username"])
		data["password"] = []byte(values["password"])
	case "ssh":
		if values["privateKey"] == "" {
			return nil, errorx.Msg("SSH Key 凭证不完整")
		}
		secretType = corev1.SecretTypeSSHAuth
		data["id_rsa"] = []byte(values["privateKey"])
		data["ssh-privatekey"] = []byte(values["privateKey"])
		if values["username"] != "" {
			data["username"] = []byte(values["username"])
		}
		if values["passphrase"] != "" {
			data["passphrase"] = []byte(values["passphrase"])
		}
	case "dockerconfigjson":
		if values["secretText"] == "" {
			return nil, errorx.Msg("Docker configjson 必须使用 Secret Text 凭证")
		}
		secretType = corev1.SecretTypeDockerConfigJson
		data[corev1.DockerConfigJsonKey] = []byte(values["secretText"])
	default:
		for key, value := range values {
			if strings.TrimSpace(value) == "" {
				continue
			}
			data[key] = []byte(value)
		}
	}
	if len(data) == 0 {
		return nil, errorx.Msg("平台凭证未生成可用 Secret 数据")
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tektonWorkspaceSecretName(pipeline, item, runtimeParams),
			Namespace: namespace,
			Labels: map[string]string{
				"kube-nova.io/pipeline-id": pipeline.ID.Hex(),
				"kube-nova.io/param-code":  item.Code,
			},
		},
		Type: secretType,
		Data: data,
	}, nil
}

func tektonWorkspaceCredentialID(item model.PipelineParam, runtimeParams map[string]string) string {
	value := strings.TrimSpace(tektonPipelineParamValue(item, runtimeParams))
	if isLikelyObjectID(value) {
		return value
	}
	return strings.TrimSpace(normalizePipelineParamConfig(item).CredentialID)
}

func tektonWorkspaceSecretName(pipeline *model.DevopsPipeline, item model.PipelineParam, runtimeParams map[string]string) string {
	config := normalizePipelineParamConfig(item)
	if strings.TrimSpace(config.VoucherModel) == "" {
		return tektonPipelineParamValue(item, runtimeParams)
	}
	if tektonWorkspaceCredentialID(item, runtimeParams) == "" {
		return ""
	}
	pipelineKey := ""
	if pipeline != nil {
		pipelineKey = firstNotBlank(pipeline.ID.Hex(), pipeline.Code)
	}
	return tektonResourceName("workspace-secret", pipelineKey, item.StepNodeID, item.Code)
}

func normalizeTektonSecretProvider(provider, credentialType string) string {
	switch strings.TrimSpace(provider) {
	case "basic-auth", "ssh", "dockerconfigjson", "opaque":
		return strings.TrimSpace(provider)
	}
	switch strings.TrimSpace(credentialType) {
	case "username_password":
		return "basic-auth"
	case "ssh_key":
		return "ssh"
	case "secret_text", "token", "kubeconfig", "certificate", "json":
		return "opaque"
	default:
		return "opaque"
	}
}

func resolveTektonSecretCredentialFields(ctx context.Context, svcCtx *svc.ServiceContext, pipeline *model.DevopsPipeline, item model.PipelineParam, credentialID, provider string, renderCtx pipelineRenderContext) (map[string]string, error) {
	config := normalizePipelineParamConfig(item)
	fields, err := tektonSecretFields(provider, config.VoucherModel)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(fields))
	for _, field := range fields {
		resp, err := svcCtx.PipelineConfigRpc.ResolveCredentialValue(ctx, &pipelineconfigservice.ResolveCredentialValueReq{
			ProjectId:             pipeline.ProjectID,
			CredentialId:          credentialID,
			CredentialType:        config.VoucherModel,
			MappingField:          field,
			CredentialMode:        "field",
			CurrentUserId:         renderCtx.UserID,
			CurrentRoles:          renderCtx.Roles,
			BuildChannelBindingId: pipeline.BuildChannelBindingID,
		})
		if err != nil {
			logx.Errorf("解析 Tekton Secret 凭证字段失败: pipeline=%s param=%s field=%s err=%v", pipeline.Code, item.Code, field, err)
			return nil, err
		}
		result[field] = strings.TrimSpace(resp.GetValue())
	}
	return result, nil
}

func tektonSecretFields(provider, credentialType string) ([]string, error) {
	switch provider {
	case "basic-auth":
		if credentialType != "username_password" {
			return nil, errorx.Msg("basic-auth Secret 只能使用用户名密码凭证")
		}
		return []string{"username", "password"}, nil
	case "ssh":
		if credentialType != "ssh_key" {
			return nil, errorx.Msg("ssh Secret 只能使用 SSH Key 凭证")
		}
		return []string{"username", "privateKey", "passphrase"}, nil
	case "dockerconfigjson":
		if credentialType != "secret_text" {
			return nil, errorx.Msg("dockerconfigjson Secret 只能使用 Secret Text 凭证")
		}
		return []string{"secretText"}, nil
	default:
		switch credentialType {
		case "username_password":
			return []string{"username", "password"}, nil
		case "token":
			return []string{"token"}, nil
		case "ssh_key":
			return []string{"username", "privateKey", "passphrase"}, nil
		case "kubeconfig":
			return []string{"kubeconfig"}, nil
		case "secret_text":
			return []string{"secretText"}, nil
		case "certificate":
			return []string{"certificate"}, nil
		case "json":
			return []string{"jsonBase64"}, nil
		default:
			return nil, errorx.Msg("凭证类型不支持生成 Kubernetes Secret")
		}
	}
}
