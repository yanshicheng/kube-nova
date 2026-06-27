package application

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	flaggerv1 "github.com/fluxcd/flagger/pkg/apis/flagger/v1beta1"
	"github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	"github.com/yanshicheng/kube-nova/common/utils"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/yaml"
)

type batchResourceDocument struct {
	Index         int
	Kind          string
	Name          string
	Namespace     string
	ClusterScoped bool
	ResourceType  string
	Object        runtime.Object
}

type batchApplicationMeta struct {
	NameCn       string
	NameEn       string
	Version      string
	Description  string
	ResourceName string
	ResourceType string
	OneTime      bool
}

var clusterScopedKinds = map[string]struct{}{
	"clusterrole":        {},
	"clusterrolebinding": {},
	"storageclass":       {},
	"persistentvolume":   {},
	"ingressclass":       {},
}

var primaryWorkloadKinds = map[string]string{
	"deployment":  "deployment",
	"statefulset": "statefulset",
	"daemonset":   "daemonset",
	"cronjob":     "cronjob",
	"job":         "job",
	"pod":         "pod",
}

var batchCreateOrder = map[string]int{
	"clusterrole":               10,
	"clusterrolebinding":        10,
	"storageclass":              10,
	"persistentvolume":          10,
	"ingressclass":              10,
	"configmap":                 20,
	"secret":                    20,
	"serviceaccount":            20,
	"role":                      20,
	"rolebinding":               20,
	"persistentvolumeclaim":     30,
	"service":                   40,
	"deployment":                50,
	"statefulset":               50,
	"daemonset":                 50,
	"cronjob":                   50,
	"job":                       50,
	"pod":                       50,
	"horizontalpodautoscaler":   60,
	"verticalpodautoscaler":     60,
	"canary":                    60,
	"networkpolicy":             70,
	"ingress":                   80,
	"probe":                     90,
	"prometheusrule":            90,
	"persistentvolumeclaimlist": 100,
	"default":                   999,
}

func parseBatchCreateDocuments(yamlStr string) ([]*batchResourceDocument, error) {
	reader := yamlutil.NewYAMLReader(bufio.NewReader(strings.NewReader(yamlStr)))
	documents := make([]*batchResourceDocument, 0)

	for index := 0; ; index++ {
		docBytes, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取 YAML 文档失败: %v", err)
		}

		docBytes = []byte(strings.TrimSpace(string(docBytes)))
		if len(docBytes) == 0 {
			continue
		}

		var typeMeta struct {
			APIVersion string `yaml:"apiVersion"`
			Kind       string `yaml:"kind"`
		}
		if err := yaml.Unmarshal(docBytes, &typeMeta); err != nil {
			return nil, fmt.Errorf("解析第 %d 段 YAML 类型失败: %v", index+1, err)
		}
		if typeMeta.Kind == "" {
			return nil, fmt.Errorf("第 %d 段 YAML 缺少 kind", index+1)
		}

		obj, err := decodeBatchResourceDocument(docBytes, typeMeta.Kind)
		if err != nil {
			return nil, fmt.Errorf("解析第 %d 段 YAML 失败: %v", index+1, err)
		}

		metaObj, err := meta.Accessor(obj)
		if err != nil {
			return nil, fmt.Errorf("读取第 %d 段资源元数据失败: %v", index+1, err)
		}
		if metaObj.GetName() == "" {
			return nil, fmt.Errorf("第 %d 段 YAML 缺少 metadata.name", index+1)
		}

		normalizedKind := strings.ToLower(typeMeta.Kind)
		_, isClusterScoped := clusterScopedKinds[normalizedKind]

		documents = append(documents, &batchResourceDocument{
			Index:         index,
			Kind:          typeMeta.Kind,
			Name:          metaObj.GetName(),
			Namespace:     metaObj.GetNamespace(),
			ClusterScoped: isClusterScoped,
			ResourceType:  primaryWorkloadKinds[normalizedKind],
			Object:        obj,
		})
	}

	if len(documents) == 0 {
		return nil, fmt.Errorf("YAML 内容为空")
	}

	return documents, nil
}

func decodeBatchResourceDocument(docBytes []byte, kind string) (runtime.Object, error) {
	switch strings.ToLower(kind) {
	case "deployment":
		var obj appsv1.Deployment
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "statefulset":
		var obj appsv1.StatefulSet
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "daemonset":
		var obj appsv1.DaemonSet
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "cronjob":
		var obj batchv1.CronJob
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "job":
		var obj batchv1.Job
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "pod":
		var obj corev1.Pod
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "service":
		var obj corev1.Service
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "ingress":
		var obj networkingv1.Ingress
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "configmap":
		var obj corev1.ConfigMap
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "secret":
		var obj corev1.Secret
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "persistentvolumeclaim":
		var obj corev1.PersistentVolumeClaim
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "serviceaccount":
		var obj corev1.ServiceAccount
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "role":
		var obj rbacv1.Role
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "rolebinding":
		var obj rbacv1.RoleBinding
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "networkpolicy":
		var obj networkingv1.NetworkPolicy
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "horizontalpodautoscaler":
		var obj autoscalingv2.HorizontalPodAutoscaler
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "verticalpodautoscaler":
		var obj vpav1.VerticalPodAutoscaler
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "canary":
		var obj flaggerv1.Canary
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "clusterrole":
		var obj rbacv1.ClusterRole
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "clusterrolebinding":
		var obj rbacv1.ClusterRoleBinding
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "storageclass":
		var obj storagev1.StorageClass
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "persistentvolume":
		var obj corev1.PersistentVolume
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "ingressclass":
		var obj networkingv1.IngressClass
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "probe":
		var obj v1.Probe
		return &obj, yaml.Unmarshal(docBytes, &obj)
	case "prometheusrule":
		var obj v1.PrometheusRule
		return &obj, yaml.Unmarshal(docBytes, &obj)
	default:
		return nil, fmt.Errorf("暂不支持的资源类型: %s", kind)
	}
}

func normalizeBatchDocuments(documents []*batchResourceDocument, namespace string) error {
	for _, document := range documents {
		metaObj, err := meta.Accessor(document.Object)
		if err != nil {
			return fmt.Errorf("读取资源 %s/%s 元数据失败: %v", document.Kind, document.Name, err)
		}

		if document.ClusterScoped {
			metaObj.SetNamespace("")
			document.Namespace = ""
			continue
		}

		resourceNamespace := metaObj.GetNamespace()
		if resourceNamespace == "" {
			resourceNamespace = namespace
			metaObj.SetNamespace(namespace)
		}
		if resourceNamespace != namespace {
			return fmt.Errorf("资源 %s/%s 的 namespace 必须为 %s", document.Kind, document.Name, namespace)
		}
		document.Namespace = resourceNamespace
	}

	sort.SliceStable(documents, func(i, j int) bool {
		left := batchCreateOrder[strings.ToLower(documents[i].Kind)]
		right := batchCreateOrder[strings.ToLower(documents[j].Kind)]
		if left == 0 {
			left = batchCreateOrder["default"]
		}
		if right == 0 {
			right = batchCreateOrder["default"]
		}
		if left == right {
			return documents[i].Index < documents[j].Index
		}
		return left < right
	})

	return nil
}

func buildBatchApplicationMeta(documents []*batchResourceDocument) (*batchApplicationMeta, error) {
	workloads := make([]*batchResourceDocument, 0)
	for _, document := range documents {
		if document.ResourceType != "" {
			workloads = append(workloads, document)
		}
	}

	if len(workloads) == 0 {
		return nil, nil
	}
	if len(workloads) > 1 {
		return nil, fmt.Errorf("批量创建只允许包含一个主工作负载")
	}

	primary := workloads[0]
	metaObj, err := meta.Accessor(primary.Object)
	if err != nil {
		return nil, fmt.Errorf("读取主工作负载元数据失败: %v", err)
	}

	labels := metaObj.GetLabels()
	annotations := metaObj.GetAnnotations()
	if labels == nil {
		labels = make(map[string]string)
	}
	if annotations == nil {
		annotations = make(map[string]string)
	}

	nameEn := firstNonEmpty(labels["app"], metaObj.GetName())
	version := firstNonEmpty(labels["version"], annotations[utils.AnnotationVersion], "v1")
	nameCn := firstNonEmpty(annotations[utils.AnnotationServiceName], metaObj.GetName())
	description := firstNonEmpty(annotations["description"], annotations[utils.AnnotationDescription])

	applyWorkloadIdentity(primary.Object, nameEn, version)

	return &batchApplicationMeta{
		NameCn:       nameCn,
		NameEn:       nameEn,
		Version:      version,
		Description:  description,
		ResourceName: metaObj.GetName(),
		ResourceType: primary.ResourceType,
		OneTime:      primary.ResourceType == "job" || primary.ResourceType == "pod",
	}, nil
}

func applyWorkloadIdentity(obj runtime.Object, nameEn, version string) {
	switch resource := obj.(type) {
	case *appsv1.Deployment:
		ensureAppVersionLabels(&resource.ObjectMeta, nameEn, version)
		ensureAppVersionLabels(&resource.Spec.Template.ObjectMeta, nameEn, version)
	case *appsv1.StatefulSet:
		ensureAppVersionLabels(&resource.ObjectMeta, nameEn, version)
		ensureAppVersionLabels(&resource.Spec.Template.ObjectMeta, nameEn, version)
	case *appsv1.DaemonSet:
		ensureAppVersionLabels(&resource.ObjectMeta, nameEn, version)
		ensureAppVersionLabels(&resource.Spec.Template.ObjectMeta, nameEn, version)
	case *batchv1.Job:
		ensureAppVersionLabels(&resource.ObjectMeta, nameEn, version)
		ensureAppVersionLabels(&resource.Spec.Template.ObjectMeta, nameEn, version)
	case *batchv1.CronJob:
		ensureAppVersionLabels(&resource.ObjectMeta, nameEn, version)
		ensureAppVersionLabels(&resource.Spec.JobTemplate.Spec.Template.ObjectMeta, nameEn, version)
	case *corev1.Pod:
		ensureAppVersionLabels(&resource.ObjectMeta, nameEn, version)
	}
}

func ensureAppVersionLabels(meta *metav1.ObjectMeta, nameEn, version string) {
	if meta.Labels == nil {
		meta.Labels = make(map[string]string)
	}
	if meta.Labels["app"] == "" {
		meta.Labels["app"] = nameEn
	}
	if meta.Labels["version"] == "" {
		meta.Labels["version"] = version
	}
}

func applyBatchAnnotations(documents []*batchResourceDocument, info *utils.AnnotationsInfo) {
	for _, document := range documents {
		switch resource := document.Object.(type) {
		case *appsv1.Deployment:
			injectAnnotations(resource, "DEPLOYMENT", info)
			ensureLegacyAnnotations(&resource.ObjectMeta, info)
			ensureLegacyAnnotations(&resource.Spec.Template.ObjectMeta, info)
		case *appsv1.StatefulSet:
			injectAnnotations(resource, "STATEFULSET", info)
			ensureLegacyAnnotations(&resource.ObjectMeta, info)
			ensureLegacyAnnotations(&resource.Spec.Template.ObjectMeta, info)
		case *appsv1.DaemonSet:
			injectAnnotations(resource, "DAEMONSET", info)
			ensureLegacyAnnotations(&resource.ObjectMeta, info)
			ensureLegacyAnnotations(&resource.Spec.Template.ObjectMeta, info)
		case *batchv1.Job:
			injectAnnotations(resource, "JOB", info)
			ensureLegacyAnnotations(&resource.ObjectMeta, info)
			ensureLegacyAnnotations(&resource.Spec.Template.ObjectMeta, info)
		case *batchv1.CronJob:
			injectAnnotations(resource, "CRONJOB", info)
			ensureLegacyAnnotations(&resource.ObjectMeta, info)
			ensureLegacyAnnotations(&resource.Spec.JobTemplate.Spec.Template.ObjectMeta, info)
		case *corev1.Pod:
			injectAnnotations(resource, "POD", info)
			ensureLegacyAnnotations(&resource.ObjectMeta, info)
		case *corev1.Service:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *networkingv1.Ingress:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *corev1.ConfigMap:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *corev1.Secret:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *corev1.PersistentVolumeClaim:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *corev1.ServiceAccount:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *rbacv1.Role:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *rbacv1.RoleBinding:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *networkingv1.NetworkPolicy:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *autoscalingv2.HorizontalPodAutoscaler:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *vpav1.VerticalPodAutoscaler:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *flaggerv1.Canary:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *rbacv1.ClusterRole:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *rbacv1.ClusterRoleBinding:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *storagev1.StorageClass:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *corev1.PersistentVolume:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *networkingv1.IngressClass:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *v1.Probe:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		case *v1.PrometheusRule:
			applyStandardAnnotations(&resource.ObjectMeta, info)
		}
	}
}

func applyStandardAnnotations(meta *metav1.ObjectMeta, info *utils.AnnotationsInfo) {
	utils.AddAnnotations(meta, info)
	ensureLegacyAnnotations(meta, info)
}

func ensureLegacyAnnotations(meta *metav1.ObjectMeta, info *utils.AnnotationsInfo) {
	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}
	meta.Annotations["created-by"] = utils.PlatformName
	if info.Description != "" {
		meta.Annotations["description"] = info.Description
	}
}

func createBatchResource(client cluster.Client, document *batchResourceDocument) error {
	switch resource := document.Object.(type) {
	case *appsv1.Deployment:
		_, err := client.Deployment().Create(resource)
		return err
	case *appsv1.StatefulSet:
		_, err := client.StatefulSet().Create(resource)
		return err
	case *appsv1.DaemonSet:
		_, err := client.DaemonSet().Create(resource)
		return err
	case *batchv1.CronJob:
		_, err := client.CronJob().Create(resource)
		return err
	case *batchv1.Job:
		_, err := client.Job().Create(resource)
		return err
	case *corev1.Pod:
		_, err := client.Pods().Create(resource)
		return err
	case *corev1.Service:
		_, err := client.Services().Create(resource)
		return err
	case *networkingv1.Ingress:
		_, err := client.Ingresses().Create(resource)
		return err
	case *corev1.ConfigMap:
		_, err := client.ConfigMaps().Create(resource)
		return err
	case *corev1.Secret:
		_, err := client.Secrets().Create(resource)
		return err
	case *corev1.PersistentVolumeClaim:
		return client.PVC().Create(resource)
	case *corev1.ServiceAccount:
		return client.ServiceAccounts().Create(resource)
	case *rbacv1.Role:
		return client.Roles().Create(resource)
	case *rbacv1.RoleBinding:
		return client.RoleBindings().Create(resource)
	case *networkingv1.NetworkPolicy:
		_, err := client.NetworkPolicies().Create(resource)
		return err
	case *autoscalingv2.HorizontalPodAutoscaler:
		_, err := client.HPA().Create(resource)
		return err
	case *vpav1.VerticalPodAutoscaler:
		_, err := client.VPA().Create(resource)
		return err
	case *flaggerv1.Canary:
		_, err := client.Flagger().Create(resource)
		return err
	case *rbacv1.ClusterRole:
		_, err := client.ClusterRoles().Create(resource)
		return err
	case *rbacv1.ClusterRoleBinding:
		_, err := client.ClusterRoleBindings().Create(resource)
		return err
	case *storagev1.StorageClass:
		_, err := client.StorageClasses().Create(resource)
		return err
	case *corev1.PersistentVolume:
		_, err := client.PersistentVolumes().Create(resource)
		return err
	case *networkingv1.IngressClass:
		_, err := client.IngressClasses().Create(resource)
		return err
	case *v1.Probe:
		return client.Probe().Create(resource)
	case *v1.PrometheusRule:
		return client.PrometheusRule().Create(resource)
	default:
		return fmt.Errorf("不支持创建资源类型: %s", document.Kind)
	}
}

func deleteBatchResource(client cluster.Client, document *batchResourceDocument) error {
	switch resource := document.Object.(type) {
	case *appsv1.Deployment:
		return client.Deployment().Delete(resource.Namespace, resource.Name)
	case *appsv1.StatefulSet:
		return client.StatefulSet().Delete(resource.Namespace, resource.Name)
	case *appsv1.DaemonSet:
		return client.DaemonSet().Delete(resource.Namespace, resource.Name)
	case *batchv1.CronJob:
		return client.CronJob().Delete(resource.Namespace, resource.Name)
	case *batchv1.Job:
		return client.Job().Delete(resource.Namespace, resource.Name)
	case *corev1.Pod:
		return client.Pods().Delete(resource.Namespace, resource.Name)
	case *corev1.Service:
		return client.Services().Delete(resource.Namespace, resource.Name)
	case *networkingv1.Ingress:
		return client.Ingresses().Delete(resource.Namespace, resource.Name)
	case *corev1.ConfigMap:
		return client.ConfigMaps().Delete(resource.Namespace, resource.Name)
	case *corev1.Secret:
		return client.Secrets().Delete(resource.Namespace, resource.Name)
	case *corev1.PersistentVolumeClaim:
		return client.PVC().Delete(resource.Namespace, resource.Name)
	case *corev1.ServiceAccount:
		return client.ServiceAccounts().Delete(resource.Namespace, resource.Name)
	case *rbacv1.Role:
		return client.Roles().Delete(resource.Namespace, resource.Name)
	case *rbacv1.RoleBinding:
		return client.RoleBindings().Delete(resource.Namespace, resource.Name)
	case *networkingv1.NetworkPolicy:
		return client.NetworkPolicies().Delete(resource.Namespace, resource.Name)
	case *autoscalingv2.HorizontalPodAutoscaler:
		return client.HPA().Delete(resource.Namespace, resource.Name)
	case *vpav1.VerticalPodAutoscaler:
		return client.VPA().Delete(resource.Namespace, resource.Name)
	case *flaggerv1.Canary:
		return client.Flagger().Delete(resource.Namespace, resource.Name)
	case *rbacv1.ClusterRole:
		return client.ClusterRoles().Delete(resource.Name)
	case *rbacv1.ClusterRoleBinding:
		return client.ClusterRoleBindings().Delete(resource.Name)
	case *storagev1.StorageClass:
		return client.StorageClasses().Delete(resource.Name)
	case *corev1.PersistentVolume:
		return client.PersistentVolumes().Delete(resource.Name)
	case *networkingv1.IngressClass:
		return client.IngressClasses().Delete(resource.Name)
	case *v1.Probe:
		return client.Probe().Delete(resource.Namespace, resource.Name)
	case *v1.PrometheusRule:
		return client.PrometheusRule().Delete(resource.Namespace, resource.Name)
	default:
		return fmt.Errorf("不支持回滚资源类型: %s", document.Kind)
	}
}

func rollbackBatchResources(client cluster.Client, createdDocuments []*batchResourceDocument) []string {
	rollbackErrors := make([]string, 0)
	for index := len(createdDocuments) - 1; index >= 0; index-- {
		document := createdDocuments[index]
		if err := deleteBatchResource(client, document); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Sprintf("%s/%s 回滚失败: %v", document.Kind, document.Name, err))
		}
	}
	return rollbackErrors
}

func summarizeBatchResources(documents []*batchResourceDocument) string {
	items := make([]string, 0, len(documents))
	for _, document := range documents {
		if document.ClusterScoped {
			items = append(items, fmt.Sprintf("%s/%s", document.Kind, document.Name))
			continue
		}
		items = append(items, fmt.Sprintf("%s/%s/%s", document.Kind, document.Namespace, document.Name))
	}
	return strings.Join(items, ", ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
