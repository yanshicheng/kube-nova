package executionservicelogic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
	"sigs.k8s.io/yaml"
)

type tektonTriggerConfig struct {
	Enabled             bool                 `json:"enabled"`
	APIVersion          string               `json:"apiVersion"`
	Provider            string               `json:"provider"`
	Namespace           string               `json:"namespace"`
	EventListenerName   string               `json:"eventListenerName"`
	TriggerName         string               `json:"triggerName"`
	TriggerBindingName  string               `json:"triggerBindingName"`
	TriggerTemplateName string               `json:"triggerTemplateName"`
	ServiceAccountName  string               `json:"serviceAccountName"`
	ServiceType         string               `json:"serviceType"`
	IngressHost         string               `json:"ingressHost"`
	IngressPath         string               `json:"ingressPath"`
	IngressClassName    string               `json:"ingressClassName"`
	TLSSecretName       string               `json:"tlsSecretName"`
	WebhookPath         string               `json:"webhookPath"`
	WebhookURL          string               `json:"webhookUrl"`
	InterceptorType     string               `json:"interceptorType"`
	CELFilter           string               `json:"celFilter"`
	EventTypes          []string             `json:"eventTypes"`
	SecretName          string               `json:"secretName"`
	SecretKey           string               `json:"secretKey"`
	BindingParams       []tektonTriggerParam `json:"bindingParams"`
	TemplateParams      []tektonTriggerParam `json:"templateParams"`
	Labels              []tektonNameValue    `json:"labels"`
	Annotations         []tektonNameValue    `json:"annotations"`
}

type tektonTriggerParam struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

type tektonNameValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func syncTektonTrigger(ctx context.Context, data *model.DevopsPipeline, runtime *pipelineconfigservice.ResolvePipelineRuntimeResp) error {
	if !hasTektonTriggerConfig(data) {
		return nil
	}
	cfg, policy, client, err := prepareTektonTriggerSync(ctx, data, runtime)
	if err != nil {
		return err
	}
	if !policy.Enabled {
		return deleteTektonTriggerResources(ctx, client, cfg.Namespace, policy)
	}
	if strings.TrimSpace(policy.ServiceAccountName) != "" {
		if err := client.ServiceAccountExists(ctx, cfg.Namespace, policy.ServiceAccountName); err != nil {
			return errorx.Msg(err.Error())
		}
	}
	contents, err := renderTektonTriggerResources(data, cfg, policy)
	if err != nil {
		logx.Errorf("渲染 Tekton Trigger 资源失败: pipelineId=%s err=%v", data.ID.Hex(), err)
		return err
	}
	if _, err := client.ApplyTriggerResources(ctx, cfg.Namespace, contents); err != nil {
		logx.Errorf("同步 Tekton Trigger 资源失败: pipelineId=%s namespace=%s err=%v", data.ID.Hex(), cfg.Namespace, err)
		return err
	}
	return nil
}

func syncTektonTriggerWithPrevious(ctx context.Context, data, previous *model.DevopsPipeline, runtime *pipelineconfigservice.ResolvePipelineRuntimeResp) error {
	if previous != nil &&
		previous.EngineType == engineTekton &&
		hasTektonTriggerConfig(previous) &&
		tektonTriggerConfigSignature(previous) != tektonTriggerConfigSignature(data) {
		if err := deleteTektonTrigger(ctx, previous, runtime); err != nil {
			return err
		}
	}
	return syncTektonTrigger(ctx, data, runtime)
}

func deleteTektonTrigger(ctx context.Context, data *model.DevopsPipeline, runtime *pipelineconfigservice.ResolvePipelineRuntimeResp) error {
	cfg, policy, client, err := prepareTektonTriggerSync(ctx, data, runtime)
	if err != nil {
		return err
	}
	return deleteTektonTriggerResources(ctx, client, cfg.Namespace, policy)
}

func prepareTektonTriggerSync(ctx context.Context, data *model.DevopsPipeline, runtime *pipelineconfigservice.ResolvePipelineRuntimeResp) (tektonBindingConfig, tektonTriggerConfig, *devopstekton.Client, error) {
	if runtime == nil || runtime.Binding == nil || runtime.Channel == nil {
		return tektonBindingConfig{}, tektonTriggerConfig{}, nil, errorx.Msg("Tekton 运行时配置不完整")
	}
	cfg, err := parseTektonBindingConfig(runtime.Binding.BindingConfig)
	if err != nil {
		return tektonBindingConfig{}, tektonTriggerConfig{}, nil, err
	}
	policy, err := parseTektonTriggerConfig(data)
	if err != nil {
		return tektonBindingConfig{}, tektonTriggerConfig{}, nil, err
	}
	if strings.TrimSpace(policy.Namespace) != "" && policy.Namespace != cfg.Namespace {
		return tektonBindingConfig{}, tektonTriggerConfig{}, nil, errorx.Msg("Tekton Trigger namespace 必须是 " + cfg.Namespace)
	}
	policy.Namespace = cfg.Namespace
	client, err := devopstekton.NewClient(tektonRequestFromRuntime(runtime))
	if err != nil {
		logx.Errorf("创建 Tekton 客户端失败: %v", err)
		return tektonBindingConfig{}, tektonTriggerConfig{}, nil, err
	}
	if err := client.NamespaceExists(ctx, cfg.Namespace); err != nil {
		return tektonBindingConfig{}, tektonTriggerConfig{}, nil, errorx.Msg(err.Error())
	}
	return cfg, policy, client, nil
}

func hasTektonTriggerConfig(data *model.DevopsPipeline) bool {
	if data == nil {
		return false
	}
	if strings.TrimSpace(data.TektonTriggerConfig) != "" {
		return true
	}
	return tektonDagTriggerConfigRaw(data) != ""
}

func tektonTriggerConfigSignature(data *model.DevopsPipeline) string {
	if data == nil {
		return ""
	}
	if content := strings.TrimSpace(data.TektonTriggerConfig); content != "" {
		return content
	}
	return tektonDagTriggerConfigRaw(data)
}

func tektonDagTriggerConfigRaw(data *model.DevopsPipeline) string {
	if data == nil || strings.TrimSpace(data.TektonDagConfig) == "" {
		return ""
	}
	var dag struct {
		TriggerConfig *json.RawMessage `json:"triggerConfig"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(data.TektonDagConfig)), &dag); err != nil || dag.TriggerConfig == nil {
		return ""
	}
	raw := strings.TrimSpace(string(*dag.TriggerConfig))
	if raw == "null" {
		return ""
	}
	return raw
}

func parseTektonTriggerConfig(data *model.DevopsPipeline) (tektonTriggerConfig, error) {
	pipelineName := tektonPipelineName(data.Code)
	policy := tektonTriggerConfig{
		APIVersion:          "triggers.tekton.dev/v1beta1",
		EventListenerName:   tektonResourceName(pipelineName, "el"),
		TriggerName:         tektonResourceName(pipelineName, "trigger"),
		TriggerBindingName:  tektonResourceName(pipelineName, "binding"),
		TriggerTemplateName: tektonResourceName(pipelineName, "template"),
		ServiceType:         "ClusterIP",
	}
	content := strings.TrimSpace(data.TektonTriggerConfig)
	if content == "" && strings.TrimSpace(data.TektonDagConfig) != "" {
		var dag struct {
			TriggerConfig tektonTriggerConfig `json:"triggerConfig"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(data.TektonDagConfig)), &dag); err != nil {
			logx.Errorf("解析 Tekton DAG Trigger 配置失败: pipelineId=%s err=%v", data.ID.Hex(), err)
			return tektonTriggerConfig{}, errorx.Msg("Tekton DAG 配置必须是结构化 JSON")
		}
		policy = dag.TriggerConfig
	}
	if content == "" {
		policy.APIVersion = firstNotBlank(policy.APIVersion, "triggers.tekton.dev/v1beta1")
		policy.EventListenerName = firstNotBlank(policy.EventListenerName, tektonResourceName(pipelineName, "el"))
		policy.TriggerName = firstNotBlank(policy.TriggerName, tektonResourceName(pipelineName, "trigger"))
		policy.TriggerBindingName = firstNotBlank(policy.TriggerBindingName, tektonResourceName(pipelineName, "binding"))
		policy.TriggerTemplateName = firstNotBlank(policy.TriggerTemplateName, tektonResourceName(pipelineName, "template"))
		policy.ServiceType = firstNotBlank(policy.ServiceType, "ClusterIP")
		policy.SecretKey = firstNotBlank(policy.SecretKey, "token")
		return policy, nil
	}
	if err := json.Unmarshal([]byte(content), &policy); err != nil {
		logx.Errorf("解析 Tekton Trigger 配置失败: pipelineId=%s err=%v", data.ID.Hex(), err)
		return tektonTriggerConfig{}, errorx.Msg("Tekton Trigger 配置必须是结构化 JSON")
	}
	policy.APIVersion = firstNotBlank(policy.APIVersion, "triggers.tekton.dev/v1beta1")
	policy.EventListenerName = firstNotBlank(policy.EventListenerName, tektonResourceName(pipelineName, "el"))
	policy.TriggerName = firstNotBlank(policy.TriggerName, tektonResourceName(pipelineName, "trigger"))
	policy.TriggerBindingName = firstNotBlank(policy.TriggerBindingName, tektonResourceName(pipelineName, "binding"))
	policy.TriggerTemplateName = firstNotBlank(policy.TriggerTemplateName, tektonResourceName(pipelineName, "template"))
	policy.ServiceType = firstNotBlank(policy.ServiceType, "ClusterIP")
	policy.SecretKey = firstNotBlank(policy.SecretKey, "token")
	return policy, nil
}

func renderTektonTriggerResources(data *model.DevopsPipeline, cfg tektonBindingConfig, policy tektonTriggerConfig) ([]string, error) {
	pipelineName := strings.TrimSpace(data.TektonPipelineName)
	if pipelineName == "" {
		pipelineName = tektonPipelineName(data.Code)
	}
	labels := tektonTriggerLabels(data, policy)
	annotations := tektonTriggerAnnotations(policy)
	bindingParams := tektonTriggerBindingParams(policy, data)
	templateParams := tektonTriggerTemplateParams(policy, data)
	runtimeParams := tektonTriggerRuntimeParams(templateParams)
	workspaces, err := renderTektonTriggerWorkspaces(data, runtimeParams)
	if err != nil {
		return nil, err
	}
	runTemplate, err := renderTektonTriggerPipelineRunTemplate(data, cfg, pipelineName, templateParams, workspaces)
	if err != nil {
		return nil, err
	}
	triggerSpec := map[string]any{
		"bindings": []map[string]any{{"ref": policy.TriggerBindingName}},
		"template": map[string]any{"ref": policy.TriggerTemplateName},
	}
	if interceptors := tektonTriggerInterceptors(policy); len(interceptors) > 0 {
		triggerSpec["interceptors"] = interceptors
	}
	docs := []map[string]any{
		{
			"apiVersion": policy.APIVersion,
			"kind":       "TriggerBinding",
			"metadata":   tektonTriggerMetadata(policy.TriggerBindingName, cfg.Namespace, labels, annotations),
			"spec": map[string]any{
				"params": bindingParams,
			},
		},
		{
			"apiVersion": policy.APIVersion,
			"kind":       "TriggerTemplate",
			"metadata":   tektonTriggerMetadata(policy.TriggerTemplateName, cfg.Namespace, labels, annotations),
			"spec": map[string]any{
				"params":            templateParams,
				"resourcetemplates": []map[string]any{runTemplate},
			},
		},
		{
			"apiVersion": policy.APIVersion,
			"kind":       "Trigger",
			"metadata":   tektonTriggerMetadata(policy.TriggerName, cfg.Namespace, labels, annotations),
			"spec":       triggerSpec,
		},
		{
			"apiVersion": policy.APIVersion,
			"kind":       "EventListener",
			"metadata":   tektonTriggerMetadata(policy.EventListenerName, cfg.Namespace, labels, annotations),
			"spec": map[string]any{
				"serviceAccountName": firstNotBlank(policy.ServiceAccountName, cfg.ServiceAccountName),
				"resources": map[string]any{
					"kubernetesResource": map[string]any{
						"serviceType": policy.ServiceType,
					},
				},
				"triggers": []map[string]any{{"triggerRef": policy.TriggerName}},
			},
		},
	}
	if ingress := renderTektonTriggerIngress(policy, cfg.Namespace, labels, annotations); ingress != nil {
		docs = append(docs, ingress)
	}
	contents := make([]string, 0, len(docs))
	for _, doc := range docs {
		content, err := yaml.Marshal(doc)
		if err != nil {
			return nil, err
		}
		contents = append(contents, string(content))
	}
	return contents, nil
}

func renderTektonTriggerPipelineRunTemplate(data *model.DevopsPipeline, cfg tektonBindingConfig, pipelineName string, params []map[string]any, workspaces []map[string]any) (map[string]any, error) {
	runParams := make([]map[string]any, 0, len(params))
	for _, item := range params {
		name := strings.TrimSpace(fmt.Sprint(item["name"]))
		if name != "" {
			runParams = append(runParams, map[string]any{"name": name, "value": "$(tt.params." + name + ")"})
		}
	}
	spec := map[string]any{
		"pipelineRef": map[string]any{"name": pipelineName},
	}
	if len(runParams) > 0 {
		spec["params"] = runParams
	}
	hasDagConfig := hasTektonDagConfig(data.TektonDagConfig)
	var dagRunPolicy *tektonDagRunPolicy
	if hasDagConfig {
		policy, err := tektonRunPolicyFromPipeline(data)
		if err != nil {
			return nil, err
		}
		dagRunPolicy = &policy
		if err := applyTektonDagPipelineRunSpec(spec, data, cfg, policy); err != nil {
			return nil, err
		}
	} else if strings.TrimSpace(cfg.ServiceAccountName) != "" {
		applyTektonPipelineRunServiceAccount(spec, cfg.ServiceAccountName)
	}
	if len(workspaces) > 0 {
		spec["workspaces"] = workspaces
	}
	metadata := map[string]any{
		"generateName": pipelineName + "-",
		"labels": map[string]any{
			"kube-nova.io/pipeline-id":  data.ID.Hex(),
			"kube-nova.io/engine":       engineTekton,
			"kube-nova.io/trigger-type": "webhook",
		},
	}
	if dagRunPolicy != nil {
		mergeTektonNameValueMap(metadata["labels"].(map[string]any), dagRunPolicy.Labels)
		annotations := map[string]any{}
		mergeTektonNameValueMap(annotations, dagRunPolicy.Annotations)
		if len(annotations) > 0 {
			metadata["annotations"] = annotations
		}
	}
	return map[string]any{
		"apiVersion": "tekton.dev/v1",
		"kind":       "PipelineRun",
		"metadata":   metadata,
		"spec":       spec,
	}, nil
}

func tektonTriggerInterceptors(policy tektonTriggerConfig) []map[string]any {
	interceptorType := tektonTriggerInterceptorType(policy)
	if interceptorType == "" || interceptorType == "none" {
		return nil
	}
	if interceptorType == "cel" {
		filter := strings.TrimSpace(policy.CELFilter)
		if filter == "" {
			return nil
		}
		return []map[string]any{{
			"ref":    map[string]any{"name": "cel"},
			"params": []map[string]any{{"name": "filter", "value": filter}},
		}}
	}
	params := make([]map[string]any, 0, 2)
	if secretName := strings.TrimSpace(policy.SecretName); secretName != "" {
		params = append(params, map[string]any{
			"name": "secretRef",
			"value": map[string]any{
				"secretName": secretName,
				"secretKey":  firstNotBlank(policy.SecretKey, "token"),
			},
		})
	}
	if eventTypes := tektonTriggerEventTypes(policy); len(eventTypes) > 0 {
		params = append(params, map[string]any{"name": "eventTypes", "value": eventTypes})
	}
	interceptor := map[string]any{"ref": map[string]any{"name": interceptorType}}
	if len(params) > 0 {
		interceptor["params"] = params
	}
	return []map[string]any{interceptor}
}

func tektonTriggerInterceptorType(policy tektonTriggerConfig) string {
	value := strings.ToLower(strings.TrimSpace(policy.InterceptorType))
	if value != "" {
		return value
	}
	switch strings.ToLower(strings.TrimSpace(policy.Provider)) {
	case "github":
		return "github"
	case "gitlab":
		return "gitlab"
	default:
		return "none"
	}
}

func tektonTriggerEventTypes(policy tektonTriggerConfig) []string {
	result := make([]string, 0, len(policy.EventTypes))
	for _, item := range policy.EventTypes {
		if value := strings.TrimSpace(item); value != "" {
			result = append(result, value)
		}
	}
	if len(result) > 0 {
		return result
	}
	switch strings.ToLower(strings.TrimSpace(policy.Provider)) {
	case "github":
		return []string{"push"}
	case "gitlab":
		return []string{"Push Hook"}
	default:
		return nil
	}
}

func renderTektonTriggerIngress(policy tektonTriggerConfig, namespace string, labels, annotations map[string]any) map[string]any {
	host := strings.TrimSpace(policy.IngressHost)
	if host == "" {
		return nil
	}
	path := firstNotBlank(policy.WebhookPath, policy.IngressPath, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	spec := map[string]any{
		"rules": []map[string]any{{
			"host": host,
			"http": map[string]any{
				"paths": []map[string]any{{
					"path":     path,
					"pathType": "Prefix",
					"backend": map[string]any{
						"service": map[string]any{
							"name": "el-" + policy.EventListenerName,
							"port": map[string]any{"number": 8080},
						},
					},
				}},
			},
		}},
	}
	if className := strings.TrimSpace(policy.IngressClassName); className != "" {
		spec["ingressClassName"] = className
	}
	if tlsSecret := strings.TrimSpace(policy.TLSSecretName); tlsSecret != "" {
		spec["tls"] = []map[string]any{{"hosts": []string{host}, "secretName": tlsSecret}}
	}
	return map[string]any{
		"apiVersion": "networking.k8s.io/v1",
		"kind":       "Ingress",
		"metadata":   tektonTriggerMetadata(tektonTriggerIngressName(policy), namespace, labels, annotations),
		"spec":       spec,
	}
}

func tektonTriggerIngressName(policy tektonTriggerConfig) string {
	return tektonResourceName(policy.EventListenerName, "ingress")
}

func renderTektonTriggerWorkspaces(data *model.DevopsPipeline, runtimeParams map[string]string) ([]map[string]any, error) {
	if hasTektonDagConfig(data.TektonDagConfig) {
		return tektonDagPipelineRunWorkspaces(data, runtimeParams)
	}
	return tektonPipelineRunWorkspaces(data, runtimeParams)
}

func deleteTektonTriggerResources(ctx context.Context, client *devopstekton.Client, namespace string, policy tektonTriggerConfig) error {
	refs := []devopstekton.TriggerResourceRef{
		{APIVersion: policy.APIVersion, Kind: "EventListener", Name: policy.EventListenerName},
		{APIVersion: policy.APIVersion, Kind: "Trigger", Name: policy.TriggerName},
		{APIVersion: policy.APIVersion, Kind: "TriggerBinding", Name: policy.TriggerBindingName},
		{APIVersion: policy.APIVersion, Kind: "TriggerTemplate", Name: policy.TriggerTemplateName},
		{APIVersion: "networking.k8s.io/v1", Kind: "Ingress", Name: tektonTriggerIngressName(policy)},
	}
	if err := client.DeleteTriggerResources(ctx, namespace, refs); err != nil {
		logx.Errorf("删除 Tekton Trigger 资源失败: namespace=%s err=%v", namespace, err)
		return err
	}
	return nil
}

func tektonTriggerBindingParams(policy tektonTriggerConfig, data *model.DevopsPipeline) []map[string]any {
	params := policy.BindingParams
	if len(params) == 0 {
		params = tektonTriggerParamsFromPipeline(data)
	}
	result := make([]map[string]any, 0, len(params))
	for _, item := range params {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		value := strings.TrimSpace(item.Value)
		if value == "" {
			value = "$(body." + name + ")"
		}
		result = append(result, map[string]any{"name": name, "value": value})
	}
	return result
}

func tektonTriggerTemplateParams(policy tektonTriggerConfig, data *model.DevopsPipeline) []map[string]any {
	params := policy.TemplateParams
	if len(params) == 0 {
		params = policy.BindingParams
	}
	if len(params) == 0 {
		params = tektonTriggerParamsFromPipeline(data)
	}
	result := make([]map[string]any, 0, len(params))
	for _, item := range params {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		param := map[string]any{"name": name}
		if desc := strings.TrimSpace(item.Description); desc != "" {
			param["description"] = desc
		}
		if value := strings.TrimSpace(item.Value); value != "" && !strings.HasPrefix(value, "$(") {
			param["default"] = value
		}
		result = append(result, param)
	}
	return result
}

func tektonTriggerRuntimeParams(params []map[string]any) map[string]string {
	result := make(map[string]string, len(params))
	for _, item := range params {
		name := strings.TrimSpace(fmt.Sprint(item["name"]))
		if name != "" {
			result[name] = "$(tt.params." + name + ")"
		}
	}
	return result
}

func tektonTriggerParamsFromPipeline(data *model.DevopsPipeline) []tektonTriggerParam {
	result := make([]tektonTriggerParam, 0, len(data.Params))
	for _, item := range data.Params {
		name := strings.TrimSpace(item.Code)
		if name != "" {
			result = append(result, tektonTriggerParam{Name: name, Value: "$(body." + name + ")", Description: item.Description})
		}
	}
	return result
}

func tektonTriggerLabels(data *model.DevopsPipeline, policy tektonTriggerConfig) map[string]any {
	labels := map[string]any{
		"kube-nova.io/pipeline-id": data.ID.Hex(),
		"kube-nova.io/engine":      engineTekton,
	}
	for _, item := range policy.Labels {
		if key := strings.TrimSpace(item.Name); key != "" {
			labels[key] = strings.TrimSpace(item.Value)
		}
	}
	return labels
}

func tektonTriggerAnnotations(policy tektonTriggerConfig) map[string]any {
	annotations := map[string]any{}
	for _, item := range policy.Annotations {
		if key := strings.TrimSpace(item.Name); key != "" {
			annotations[key] = strings.TrimSpace(item.Value)
		}
	}
	return annotations
}

func tektonTriggerMetadata(name, namespace string, labels, annotations map[string]any) map[string]any {
	metadata := map[string]any{
		"name":      name,
		"namespace": namespace,
		"labels":    labels,
	}
	if len(annotations) > 0 {
		metadata["annotations"] = annotations
	}
	return metadata
}
