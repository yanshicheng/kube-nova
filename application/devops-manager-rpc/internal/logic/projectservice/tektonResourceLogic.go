package projectservicelogic

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type tektonResourceItem struct {
	name             string
	namespace        string
	kind             string
	apiVersion       string
	status           string
	phase            string
	reason           string
	message          string
	keys             []string
	storageClassName string
	storage          string
	accessModes      []string
	volumeMode       string
	secrets          []string
	imagePullSecrets []string
	containerNames   []string
	createdAt        int64
	yaml             string
}

func tektonResourceToPb(item tektonResourceItem, ref tektonSecretBindingRef, resourceType string) *pb.TektonResource {
	return &pb.TektonResource{
		Name:             item.name,
		Namespace:        item.namespace,
		BindingId:        ref.Binding.ID.Hex(),
		ChannelName:      ref.Channel.Name,
		ResourceType:     resourceType,
		Kind:             item.kind,
		ApiVersion:       item.apiVersion,
		Status:           item.status,
		Phase:            item.phase,
		Reason:           item.reason,
		Message:          item.message,
		Keys:             item.keys,
		StorageClassName: item.storageClassName,
		Storage:          item.storage,
		AccessModes:      item.accessModes,
		VolumeMode:       item.volumeMode,
		Secrets:          item.secrets,
		ImagePullSecrets: item.imagePullSecrets,
		ContainerNames:   item.containerNames,
		CreatedAt:        item.createdAt,
		Yaml:             item.yaml,
	}
}

func normalizeTektonResourceType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "configmap", "config-map":
		return "configMap"
	case "pvc", "persistentvolumeclaim":
		return "pvc"
	case "serviceaccount", "service-account":
		return "serviceAccount"
	case "secret":
		return "secret"
	case "pod":
		return "pod"
	case "pipeline":
		return "pipeline"
	case "pipelinerun", "pipeline-run":
		return "pipelineRun"
	default:
		return strings.TrimSpace(value)
	}
}

func applyTektonResourceLabels(meta *metav1.ObjectMeta, projectID, bindingID, resourceType string) {
	if meta == nil {
		return
	}
	if meta.Labels == nil {
		meta.Labels = map[string]string{}
	}
	meta.Labels["kube-nova.io/managed-by"] = "kube-nova"
	meta.Labels["kube-nova.io/devops-project-id"] = strings.TrimSpace(projectID)
	meta.Labels["kube-nova.io/devops-binding-id"] = strings.TrimSpace(bindingID)
	meta.Labels["kube-nova.io/resource"] = "tekton-" + strings.TrimSpace(resourceType)
}

func validateTektonResourceName(inputName, yamlName string) error {
	inputName = strings.TrimSpace(inputName)
	yamlName = strings.TrimSpace(yamlName)
	if inputName == "" || yamlName == "" {
		return errorx.Msg("资源名称不能为空")
	}
	if inputName != yamlName {
		return errorx.Msg("资源名称必须和 YAML metadata.name 一致")
	}
	return nil
}

func validateCoreResourceKind(content, kind string) error {
	obj, err := parseTektonUnstructured(content)
	if err != nil {
		return err
	}
	if strings.TrimSpace(obj.GetKind()) == "" {
		return errorx.Msg("YAML kind 不能为空")
	}
	if !strings.EqualFold(strings.TrimSpace(obj.GetKind()), kind) {
		return errorx.Msg("YAML kind 必须是 " + kind)
	}
	if strings.TrimSpace(obj.GetAPIVersion()) == "" {
		return errorx.Msg("YAML apiVersion 不能为空")
	}
	if strings.TrimSpace(obj.GetAPIVersion()) != "v1" {
		return errorx.Msg(kind + " apiVersion 必须是 v1")
	}
	return nil
}

func parseConfigMap(content string) (*corev1.ConfigMap, error) {
	if err := validateCoreResourceKind(content, "ConfigMap"); err != nil {
		return nil, err
	}
	var cm corev1.ConfigMap
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(content)), &cm); err != nil {
		return nil, errorx.Msg("ConfigMap YAML 解析失败")
	}
	if strings.TrimSpace(cm.Name) == "" {
		return nil, errorx.Msg("ConfigMap 名称不能为空")
	}
	return &cm, nil
}

func parsePVC(content string) (*corev1.PersistentVolumeClaim, error) {
	if err := validateCoreResourceKind(content, "PersistentVolumeClaim"); err != nil {
		return nil, err
	}
	var pvc corev1.PersistentVolumeClaim
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(content)), &pvc); err != nil {
		return nil, errorx.Msg("PVC YAML 解析失败")
	}
	if strings.TrimSpace(pvc.Name) == "" {
		return nil, errorx.Msg("PVC 名称不能为空")
	}
	return &pvc, nil
}

func parseServiceAccount(content string) (*corev1.ServiceAccount, error) {
	if err := validateCoreResourceKind(content, "ServiceAccount"); err != nil {
		return nil, err
	}
	var sa corev1.ServiceAccount
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(content)), &sa); err != nil {
		return nil, errorx.Msg("ServiceAccount YAML 解析失败")
	}
	if strings.TrimSpace(sa.Name) == "" {
		return nil, errorx.Msg("ServiceAccount 名称不能为空")
	}
	return &sa, nil
}

func parseSecret(content, projectID, bindingID string) (*corev1.Secret, error) {
	if err := validateCoreResourceKind(content, "Secret"); err != nil {
		return nil, err
	}
	var secret corev1.Secret
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(content)), &secret); err != nil {
		return nil, errorx.Msg("Secret YAML 解析失败")
	}
	if strings.TrimSpace(secret.Name) == "" {
		return nil, errorx.Msg("Secret 名称不能为空")
	}
	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	secret.Labels["kube-nova.io/devops-project-id"] = strings.TrimSpace(projectID)
	secret.Labels["kube-nova.io/devops-binding-id"] = strings.TrimSpace(bindingID)
	return &secret, nil
}

func configMapsToResource(items []devopstekton.ConfigMapInfo, ref tektonSecretBindingRef, resourceType string) []*pb.TektonResource {
	result := make([]*pb.TektonResource, 0, len(items))
	for _, item := range items {
		result = append(result, tektonResourceToPb(tektonResourceItem{
			name:       item.Name,
			namespace:  item.Namespace,
			kind:       "ConfigMap",
			apiVersion: "v1",
			keys:       item.Keys,
			createdAt:  item.CreatedAt,
		}, ref, resourceType))
	}
	return result
}

func pvcsToResource(items []devopstekton.PVCInfo, ref tektonSecretBindingRef, resourceType string) []*pb.TektonResource {
	result := make([]*pb.TektonResource, 0, len(items))
	for _, item := range items {
		result = append(result, tektonResourceToPb(tektonResourceItem{
			name:             item.Name,
			namespace:        item.Namespace,
			kind:             "PersistentVolumeClaim",
			apiVersion:       "v1",
			status:           item.Status,
			storageClassName: item.StorageClassName,
			storage:          item.Storage,
			accessModes:      item.AccessModes,
			volumeMode:       item.VolumeMode,
			createdAt:        item.CreatedAt,
		}, ref, resourceType))
	}
	return result
}

func serviceAccountsToResource(items []devopstekton.ServiceAccountInfo, ref tektonSecretBindingRef, resourceType string) []*pb.TektonResource {
	result := make([]*pb.TektonResource, 0, len(items))
	for _, item := range items {
		result = append(result, tektonResourceToPb(tektonResourceItem{
			name:             item.Name,
			namespace:        item.Namespace,
			kind:             "ServiceAccount",
			apiVersion:       "v1",
			secrets:          item.Secrets,
			imagePullSecrets: item.ImagePullSecrets,
			createdAt:        item.CreatedAt,
		}, ref, resourceType))
	}
	return result
}

func secretsToResource(items []devopstekton.SecretInfo, ref tektonSecretBindingRef, resourceType string) []*pb.TektonResource {
	result := make([]*pb.TektonResource, 0, len(items))
	for _, item := range items {
		result = append(result, tektonResourceToPb(tektonResourceItem{
			name:       item.Name,
			namespace:  item.Namespace,
			kind:       "Secret",
			apiVersion: "v1",
			status:     item.Type,
			keys:       item.Keys,
			createdAt:  item.CreatedAt,
		}, ref, resourceType))
	}
	return result
}

func podsToResource(items []devopstekton.PodInfo, ref tektonSecretBindingRef, resourceType string) []*pb.TektonResource {
	result := make([]*pb.TektonResource, 0, len(items))
	for _, item := range items {
		result = append(result, tektonResourceToPb(tektonResourceItem{
			name:           item.Name,
			namespace:      item.Namespace,
			kind:           "Pod",
			apiVersion:     "v1",
			status:         item.Phase,
			phase:          item.Phase,
			reason:         item.Reason,
			message:        item.Message,
			containerNames: item.ContainerNames,
			createdAt:      item.CreatedAt,
		}, ref, resourceType))
	}
	return result
}

func pipelinesToResource(items []devopstekton.PipelineInfo, ref tektonSecretBindingRef, resourceType string) []*pb.TektonResource {
	result := make([]*pb.TektonResource, 0, len(items))
	for _, item := range items {
		result = append(result, tektonResourceToPb(tektonResourceItem{
			name:        item.Name,
			namespace:   item.Namespace,
			kind:        "Pipeline",
			apiVersion:  "tekton.dev/v1",
			message:     item.Description,
			keys:        item.Params,
			accessModes: item.Workspaces,
			secrets:     item.Results,
			createdAt:   item.CreatedAt,
		}, ref, resourceType))
	}
	return result
}

func pipelineRunsToResource(items []devopstekton.PipelineRunInfo, ref tektonSecretBindingRef, resourceType string) []*pb.TektonResource {
	result := make([]*pb.TektonResource, 0, len(items))
	for _, item := range items {
		result = append(result, tektonResourceToPb(tektonResourceItem{
			name:       item.Name,
			namespace:  item.Namespace,
			kind:       "PipelineRun",
			apiVersion: "tekton.dev/v1",
			status:     item.Status,
			phase:      item.Status,
			reason:     item.Reason,
			message:    item.Message,
			createdAt:  item.CreatedAt,
		}, ref, resourceType))
	}
	return result
}

func configMapToResourceDetail(cm *corev1.ConfigMap, ref tektonSecretBindingRef, resourceType string) *pb.TektonResource {
	keys := make([]string, 0, len(cm.Data)+len(cm.BinaryData))
	for key := range cm.Data {
		keys = append(keys, key)
	}
	for key := range cm.BinaryData {
		keys = append(keys, key)
	}
	return tektonResourceToPb(tektonResourceItem{
		name:       cm.Name,
		namespace:  cm.Namespace,
		kind:       "ConfigMap",
		apiVersion: "v1",
		keys:       keys,
		createdAt:  cm.CreationTimestamp.Unix(),
		yaml:       marshalCoreConfigMapYAML(cm),
	}, ref, resourceType)
}

func pvcToResourceDetail(pvc *corev1.PersistentVolumeClaim, ref tektonSecretBindingRef, resourceType string) *pb.TektonResource {
	accessModes := make([]string, 0, len(pvc.Spec.AccessModes))
	for _, mode := range pvc.Spec.AccessModes {
		accessModes = append(accessModes, string(mode))
	}
	storageClassName := ""
	if pvc.Spec.StorageClassName != nil {
		storageClassName = *pvc.Spec.StorageClassName
	}
	volumeMode := ""
	if pvc.Spec.VolumeMode != nil {
		volumeMode = string(*pvc.Spec.VolumeMode)
	}
	storage := ""
	if quantity, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		storage = quantity.String()
	}
	return tektonResourceToPb(tektonResourceItem{
		name:             pvc.Name,
		namespace:        pvc.Namespace,
		kind:             "PersistentVolumeClaim",
		apiVersion:       "v1",
		status:           string(pvc.Status.Phase),
		storageClassName: storageClassName,
		storage:          storage,
		accessModes:      accessModes,
		volumeMode:       volumeMode,
		createdAt:        pvc.CreationTimestamp.Unix(),
		yaml:             marshalCorePVCYAML(pvc),
	}, ref, resourceType)
}

func serviceAccountToResourceDetail(sa *corev1.ServiceAccount, ref tektonSecretBindingRef, resourceType string) *pb.TektonResource {
	secrets := make([]string, 0, len(sa.Secrets))
	for _, secret := range sa.Secrets {
		if name := strings.TrimSpace(secret.Name); name != "" {
			secrets = append(secrets, name)
		}
	}
	imagePullSecrets := make([]string, 0, len(sa.ImagePullSecrets))
	for _, secret := range sa.ImagePullSecrets {
		if name := strings.TrimSpace(secret.Name); name != "" {
			imagePullSecrets = append(imagePullSecrets, name)
		}
	}
	return tektonResourceToPb(tektonResourceItem{
		name:             sa.Name,
		namespace:        sa.Namespace,
		kind:             "ServiceAccount",
		apiVersion:       "v1",
		secrets:          secrets,
		imagePullSecrets: imagePullSecrets,
		createdAt:        sa.CreationTimestamp.Unix(),
		yaml:             marshalCoreServiceAccountYAML(sa),
	}, ref, resourceType)
}

func secretToResourceDetail(secret *corev1.Secret, ref tektonSecretBindingRef, resourceType string) *pb.TektonResource {
	keys := make([]string, 0, len(secret.Data))
	for key := range secret.Data {
		keys = append(keys, key)
	}
	return tektonResourceToPb(tektonResourceItem{
		name:       secret.Name,
		namespace:  secret.Namespace,
		kind:       "Secret",
		apiVersion: "v1",
		status:     string(secret.Type),
		keys:       keys,
		createdAt:  secret.CreationTimestamp.Unix(),
		yaml:       marshalCoreSecretYAML(secret),
	}, ref, resourceType)
}

func podFromYAML(content string) (tektonResourceItem, error) {
	obj, err := parseTektonUnstructured(content)
	if err != nil {
		return tektonResourceItem{}, err
	}
	var pod corev1.Pod
	if err := yaml.Unmarshal([]byte(content), &pod); err != nil {
		return tektonResourceItem{}, errorx.Msg("Pod YAML 解析失败")
	}
	if strings.TrimSpace(obj.GetKind()) == "" {
		obj.SetKind("Pod")
	}
	if strings.TrimSpace(obj.GetAPIVersion()) == "" {
		obj.SetAPIVersion("v1")
	}
	containerNames := make([]string, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers))
	for _, container := range pod.Spec.InitContainers {
		containerNames = append(containerNames, container.Name)
	}
	for _, container := range pod.Spec.Containers {
		containerNames = append(containerNames, container.Name)
	}
	return tektonResourceItem{
		name:           obj.GetName(),
		namespace:      obj.GetNamespace(),
		kind:           obj.GetKind(),
		apiVersion:     obj.GetAPIVersion(),
		status:         string(pod.Status.Phase),
		phase:          string(pod.Status.Phase),
		reason:         pod.Status.Reason,
		message:        pod.Status.Message,
		containerNames: containerNames,
		createdAt:      obj.GetCreationTimestamp().Unix(),
		yaml:           marshalTektonYAML(obj.Object),
	}, nil
}

func pipelineFromYAML(content string) (tektonResourceItem, error) {
	obj, err := parseTektonUnstructured(content)
	if err != nil {
		return tektonResourceItem{}, err
	}
	params, workspaces, results := tektonCollections(obj)
	if strings.TrimSpace(obj.GetKind()) == "" {
		obj.SetKind("Pipeline")
	}
	if strings.TrimSpace(obj.GetAPIVersion()) == "" {
		obj.SetAPIVersion("tekton.dev/v1")
	}
	return tektonResourceItem{
		name:        obj.GetName(),
		namespace:   obj.GetNamespace(),
		kind:        obj.GetKind(),
		apiVersion:  obj.GetAPIVersion(),
		message:     tektonDescription(obj),
		keys:        params,
		accessModes: workspaces,
		secrets:     results,
		createdAt:   obj.GetCreationTimestamp().Unix(),
		yaml:        marshalTektonYAML(obj.Object),
	}, nil
}

func pipelineRunFromYAML(content string) (tektonResourceItem, error) {
	obj, err := parseTektonUnstructured(content)
	if err != nil {
		return tektonResourceItem{}, err
	}
	status := tektonRunStatus(obj)
	if strings.TrimSpace(obj.GetKind()) == "" {
		obj.SetKind("PipelineRun")
	}
	if strings.TrimSpace(obj.GetAPIVersion()) == "" {
		obj.SetAPIVersion("tekton.dev/v1")
	}
	return tektonResourceItem{
		name:       obj.GetName(),
		namespace:  obj.GetNamespace(),
		kind:       obj.GetKind(),
		apiVersion: obj.GetAPIVersion(),
		status:     status,
		phase:      status,
		reason:     tektonReason(obj),
		message:    tektonMessage(obj),
		createdAt:  obj.GetCreationTimestamp().Unix(),
		yaml:       marshalTektonYAML(obj.Object),
	}, nil
}

func parseTektonUnstructured(content string) (*unstructured.Unstructured, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errorx.Msg("YAML 不能为空")
	}
	jsonData, err := yaml.YAMLToJSON([]byte(content))
	if err != nil {
		return nil, errorx.Msg("YAML 解析失败")
	}
	var raw map[string]any
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		return nil, errorx.Msg("YAML 解析失败")
	}
	return &unstructured.Unstructured{Object: raw}, nil
}

func tektonCollections(obj *unstructured.Unstructured) ([]string, []string, []string) {
	params := tektonNamesFromSlice(obj, "spec", "params")
	workspaces := tektonNamesFromSlice(obj, "spec", "workspaces")
	results := tektonNamesFromSlice(obj, "spec", "results")
	return params, workspaces, results
}

func tektonNamesFromSlice(obj *unstructured.Unstructured, fields ...string) []string {
	items, ok, err := unstructured.NestedSlice(obj.Object, fields...)
	if err != nil || !ok || len(items) == 0 {
		return nil
	}
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(data["name"]))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}

func tektonDescription(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	if desc := strings.TrimSpace(obj.GetAnnotations()["tekton.dev/displayName"]); desc != "" {
		return desc
	}
	if desc, ok, _ := unstructured.NestedString(obj.Object, "spec", "description"); ok {
		return strings.TrimSpace(desc)
	}
	return ""
}

func tektonRunStatus(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	conditions, ok, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !ok || len(conditions) == 0 {
		return ""
	}
	for _, item := range conditions {
		condition, ok := item.(map[string]any)
		if !ok || strings.TrimSpace(strings.ToLower(fmt.Sprint(condition["type"]))) != "succeeded" {
			continue
		}
		return strings.ToLower(strings.TrimSpace(fmt.Sprint(condition["status"])))
	}
	return ""
}

func tektonReason(obj *unstructured.Unstructured) string {
	conditions, ok, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !ok || len(conditions) == 0 {
		return ""
	}
	for _, item := range conditions {
		condition, ok := item.(map[string]any)
		if !ok || strings.TrimSpace(fmt.Sprint(condition["type"])) != "Succeeded" {
			continue
		}
		return strings.TrimSpace(fmt.Sprint(condition["reason"]))
	}
	return ""
}

func tektonMessage(obj *unstructured.Unstructured) string {
	conditions, ok, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !ok || len(conditions) == 0 {
		return ""
	}
	for _, item := range conditions {
		condition, ok := item.(map[string]any)
		if !ok || strings.TrimSpace(fmt.Sprint(condition["type"])) != "Succeeded" {
			continue
		}
		return strings.TrimSpace(fmt.Sprint(condition["message"]))
	}
	return ""
}

func marshalCoreConfigMapYAML(cm *corev1.ConfigMap) string {
	cp := cm.DeepCopy()
	cp.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"}
	return marshalTektonYAML(cp)
}

func marshalCorePVCYAML(pvc *corev1.PersistentVolumeClaim) string {
	cp := pvc.DeepCopy()
	cp.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "PersistentVolumeClaim"}
	return marshalTektonYAML(cp)
}

func marshalCoreServiceAccountYAML(sa *corev1.ServiceAccount) string {
	cp := sa.DeepCopy()
	cp.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "ServiceAccount"}
	return marshalTektonYAML(cp)
}

func marshalCoreSecretYAML(secret *corev1.Secret) string {
	cp := secret.DeepCopy()
	cp.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"}
	return marshalTektonYAML(cp)
}

func marshalTektonYAML(obj any) string {
	data, err := yaml.Marshal(obj)
	if err != nil {
		return ""
	}
	return string(data)
}
