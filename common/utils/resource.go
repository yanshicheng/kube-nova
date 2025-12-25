package utils

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
)

// K8sResourceValidator 统一的 K8s 资源验证器
type K8sResourceValidator struct {
	ExpectedNamespace string
	ExpectedName      string
	ExpectedLabels    map[string]string
}

// validateK8sResource 统一验证 K8s 资源
func (v *K8sResourceValidator) Validate(obj runtime.Object) error {
	var actualNamespace, actualName string
	// actualName
	//var actualLabels map[string]string

	// 根据资源类型提取信息
	switch resource := obj.(type) {
	case *appsv1.Deployment:
		actualNamespace = resource.Namespace
		actualName = resource.Name
		//actualLabels = resource.Labels

	case *appsv1.StatefulSet:
		actualNamespace = resource.Namespace
		actualName = resource.Name
		//actualLabels = resource.Labels

	case *appsv1.DaemonSet:
		actualNamespace = resource.Namespace
		actualName = resource.Name
		//actualLabels = resource.Labels

	case *batchv1.Job:
		actualNamespace = resource.Namespace
		actualName = resource.Name
		//actualLabels = resource.Labels

	case *batchv1.CronJob:
		actualNamespace = resource.Namespace
		actualName = resource.Name
		//actualLabels = resource.Labels

	case *corev1.Pod:
		actualNamespace = resource.Namespace
		actualName = resource.Name
		//actualLabels = resource.Labels

	case *corev1.Service:
		actualNamespace = resource.Namespace
		actualName = resource.Name
		//actualLabels = resource.Labels

	case *networkingv1.Ingress:
		actualNamespace = resource.Namespace
		actualName = resource.Name
		//actualLabels = resource.Labels

	case *corev1.ConfigMap:
		actualNamespace = resource.Namespace
		actualName = resource.Name
		//actualLabels = resource.Labels

	case *corev1.Secret:
		actualNamespace = resource.Namespace
		actualName = resource.Name
		//actualLabels = resource.Labels

	case *corev1.PersistentVolumeClaim:
		actualNamespace = resource.Namespace
		actualName = resource.Name
		//actualLabels = resource.Labels

	default:
		return fmt.Errorf("不支持的资源类型: %T", obj)
	}

	// 1. 验证 Namespace
	if actualNamespace != v.ExpectedNamespace {
		return fmt.Errorf("资源的 namespace [%s] 与传入的 namespace [%s] 不一致",
			actualNamespace, v.ExpectedNamespace)
	}

	// 2. 验证 Name
	if actualName != v.ExpectedName {
		return fmt.Errorf("资源的 name [%s] 与期望的 name [%s] 不一致 (格式应为: nameEn-version)",
			actualName, v.ExpectedName)
	}
	//
	// 3. 验证 Labels
	//if v.ExpectedLabels != nil && len(v.ExpectedLabels) > 0 {
	//	if actualLabels == nil {
	//		return fmt.Errorf("资源缺少必需的 labels")
	//	}
	//
	//	for key, expectedValue := range v.ExpectedLabels {
	//		actualValue, exists := actualLabels[key]
	//		if !exists {
	//			return fmt.Errorf("资源缺少必需的 label: %s", key)
	//		}
	//		if actualValue != expectedValue {
	//			return fmt.Errorf("label [%s] 的值 [%s] 与传入的值 [%s] 不一致",
	//				key, actualValue, expectedValue)
	//		}
	//	}
	//}

	return nil
}

// ParseAndConvertK8sResource 解析 YAML 并根据 resourceType 转换为对应的 K8s 资源对象
func ParseAndConvertK8sResource(yamlStr string, resourceType string) (runtime.Object, error) {
	// 创建解码器
	decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode

	// 解析 YAML
	obj, _, err := decode([]byte(yamlStr), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("解析 YAML 失败: %v", err)
	}

	// 根据 resourceType 进行类型转换
	upperType := strings.ToUpper(resourceType)

	switch upperType {
	case "DEPLOYMENT":
		deployment, ok := obj.(*appsv1.Deployment)
		if !ok {
			return nil, fmt.Errorf("YAML 内容不是有效的 Deployment 资源")
		}
		return deployment, nil

	case "STATEFULSET":
		statefulSet, ok := obj.(*appsv1.StatefulSet)
		if !ok {
			return nil, fmt.Errorf("YAML 内容不是有效的 StatefulSet 资源")
		}
		return statefulSet, nil

	case "DAEMONSET":
		daemonSet, ok := obj.(*appsv1.DaemonSet)
		if !ok {
			return nil, fmt.Errorf("YAML 内容不是有效的 DaemonSet 资源")
		}
		return daemonSet, nil

	case "JOB":
		job, ok := obj.(*batchv1.Job)
		if !ok {
			return nil, fmt.Errorf("YAML 内容不是有效的 Job 资源")
		}
		return job, nil

	case "CRONJOB":
		cronJob, ok := obj.(*batchv1.CronJob)
		if !ok {
			return nil, fmt.Errorf("YAML 内容不是有效的 CronJob 资源")
		}
		return cronJob, nil

	case "POD":
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return nil, fmt.Errorf("YAML 内容不是有效的 Pod 资源")
		}
		return pod, nil

	case "SERVICE":
		service, ok := obj.(*corev1.Service)
		if !ok {
			return nil, fmt.Errorf("YAML 内容不是有效的 Service 资源")
		}
		return service, nil

	case "INGRESS":
		ingress, ok := obj.(*networkingv1.Ingress)
		if !ok {
			return nil, fmt.Errorf("YAML 内容不是有效的 Ingress 资源")
		}
		return ingress, nil

	case "CONFIGMAP":
		configMap, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return nil, fmt.Errorf("YAML 内容不是有效的 ConfigMap 资源")
		}
		return configMap, nil

	case "SECRET":
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return nil, fmt.Errorf("YAML 内容不是有效的 Secret 资源")
		}
		return secret, nil

	case "PVC":
		pvc, ok := obj.(*corev1.PersistentVolumeClaim)
		if !ok {
			return nil, fmt.Errorf("YAML 内容不是有效的 PVC 资源")
		}
		return pvc, nil

	default:
		return nil, fmt.Errorf("不支持的资源类型: %s", resourceType)
	}
}
