package logic

import (
	"context"
	"fmt"
	"strings"

	flaggerv1 "github.com/fluxcd/flagger/pkg/apis/flagger/v1beta1"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	"github.com/zeromicro/go-zero/core/logx"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/yaml"
)

// ResourceValidator 资源验证器
type ResourceValidator struct {
	ExpectedNamespace string
	ExpectedTargetRef *types.TargetRefInfo
	Logger            logx.Logger
}

// ValidateCanary 验证 Canary 资源
func (v *ResourceValidator) ValidateCanary(canary *flaggerv1.Canary) error {
	// 验证 namespace
	if canary.Namespace != v.ExpectedNamespace {
		return fmt.Errorf("namespace 不匹配: 期望 %s, 实际 %s", v.ExpectedNamespace, canary.Namespace)
	}

	// 验证 TargetRef
	if v.ExpectedTargetRef != nil {
		targetRef := canary.Spec.TargetRef
		if targetRef.APIVersion != v.ExpectedTargetRef.ApiVersion {
			return fmt.Errorf("targetRef.apiVersion 不匹配: 期望 %s, 实际 %s",
				v.ExpectedTargetRef.ApiVersion, targetRef.APIVersion)
		}
		if targetRef.Kind != v.ExpectedTargetRef.Kind {
			return fmt.Errorf("targetRef.kind 不匹配: 期望 %s, 实际 %s",
				v.ExpectedTargetRef.Kind, targetRef.Kind)
		}
		if targetRef.Name != v.ExpectedTargetRef.Name {
			return fmt.Errorf("targetRef.name 不匹配: 期望 %s, 实际 %s",
				v.ExpectedTargetRef.Name, targetRef.Name)
		}
	}

	v.Logger.Infof("Canary 资源验证通过: namespace=%s, targetRef=%s/%s",
		canary.Namespace, canary.Spec.TargetRef.Kind, canary.Spec.TargetRef.Name)
	return nil
}

// ValidateHPA 验证 HPA 资源
func (v *ResourceValidator) ValidateHPA(hpa *autoscalingv2.HorizontalPodAutoscaler) error {
	// 验证 namespace
	if hpa.Namespace != v.ExpectedNamespace {
		return fmt.Errorf("namespace 不匹配: 期望 %s, 实际 %s", v.ExpectedNamespace, hpa.Namespace)
	}

	// 验证 ScaleTargetRef
	if v.ExpectedTargetRef != nil {
		targetRef := hpa.Spec.ScaleTargetRef
		if targetRef.APIVersion != v.ExpectedTargetRef.ApiVersion {
			return fmt.Errorf("scaleTargetRef.apiVersion 不匹配: 期望 %s, 实际 %s",
				v.ExpectedTargetRef.ApiVersion, targetRef.APIVersion)
		}
		if targetRef.Kind != v.ExpectedTargetRef.Kind {
			return fmt.Errorf("scaleTargetRef.kind 不匹配: 期望 %s, 实际 %s",
				v.ExpectedTargetRef.Kind, targetRef.Kind)
		}
		if targetRef.Name != v.ExpectedTargetRef.Name {
			return fmt.Errorf("scaleTargetRef.name 不匹配: 期望 %s, 实际 %s",
				v.ExpectedTargetRef.Name, targetRef.Name)
		}
	}

	v.Logger.Infof("HPA 资源验证通过: namespace=%s, scaleTargetRef=%s/%s",
		hpa.Namespace, hpa.Spec.ScaleTargetRef.Kind, hpa.Spec.ScaleTargetRef.Name)
	return nil
}

// ValidateVPA 验证 VPA 资源
func (v *ResourceValidator) ValidateVPA(vpa *vpav1.VerticalPodAutoscaler) error {
	// 验证 namespace
	if vpa.Namespace != v.ExpectedNamespace {
		return fmt.Errorf("namespace 不匹配: 期望 %s, 实际 %s", v.ExpectedNamespace, vpa.Namespace)
	}

	// 验证 TargetRef
	if v.ExpectedTargetRef != nil && vpa.Spec.TargetRef != nil {
		targetRef := vpa.Spec.TargetRef
		if targetRef.APIVersion != v.ExpectedTargetRef.ApiVersion {
			return fmt.Errorf("targetRef.apiVersion 不匹配: 期望 %s, 实际 %s",
				v.ExpectedTargetRef.ApiVersion, targetRef.APIVersion)
		}
		if targetRef.Kind != v.ExpectedTargetRef.Kind {
			return fmt.Errorf("targetRef.kind 不匹配: 期望 %s, 实际 %s",
				v.ExpectedTargetRef.Kind, targetRef.Kind)
		}
		if targetRef.Name != v.ExpectedTargetRef.Name {
			return fmt.Errorf("targetRef.name 不匹配: 期望 %s, 实际 %s",
				v.ExpectedTargetRef.Name, targetRef.Name)
		}
	}

	v.Logger.Infof("VPA 资源验证通过: namespace=%s, targetRef=%s/%s",
		vpa.Namespace, vpa.Spec.TargetRef.Kind, vpa.Spec.TargetRef.Name)
	return nil
}

// SetOwnerReference 设置 OwnerReference
func SetOwnerReference(obj metav1.Object, ownerName, ownerKind, ownerAPIVersion string, ownerUID string) {
	controller := true
	blockOwnerDeletion := true

	ownerRef := metav1.OwnerReference{
		APIVersion:         ownerAPIVersion,
		Kind:               ownerKind,
		Name:               ownerName,
		UID:                k8stypes.UID(ownerUID),
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}

	// 检查是否已存在相同的 OwnerReference
	existingRefs := obj.GetOwnerReferences()
	for i, ref := range existingRefs {
		if ref.Kind == ownerKind && ref.Name == ownerName {
			// 更新现有引用
			existingRefs[i] = ownerRef
			obj.SetOwnerReferences(existingRefs)
			return
		}
	}

	// 添加新的 OwnerReference
	obj.SetOwnerReferences(append(existingRefs, ownerRef))
}

// GetClusterClientAndVersion 获取集群客户端和版本详情
func GetClusterClientAndVersion(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	versionId uint64,
	logger logx.Logger,
) (cluster.Client, *managerservice.GetOnecProjectVersionDetailResp, error) {
	// 获取版本详情
	versionDetail, err := svcCtx.ManagerRpc.VersionDetail(ctx, &managerservice.GetOnecProjectVersionDetailReq{
		Id: versionId,
	})
	if err != nil {
		logger.Errorf("获取版本详情失败: %v", err)
		return nil, nil, fmt.Errorf("获取版本详情失败: %v", err)
	}

	// 获取集群客户端
	client, err := svcCtx.K8sManager.GetCluster(ctx, versionDetail.ClusterUuid)
	if err != nil {
		logger.Errorf("获取集群客户端失败: %v", err)
		return nil, nil, fmt.Errorf("获取集群客户端失败: %v", err)
	}

	return client, versionDetail, nil
}

// GetTargetRefFromVersion 根据版本信息构建 TargetRefInfo
func GetTargetRefFromVersion(versionDetail *managerservice.GetOnecProjectVersionDetailResp) *types.TargetRefInfo {
	resourceType := strings.Title(strings.ToLower(versionDetail.ResourceType))

	apiVersion := "apps/v1"
	if resourceType == "Cronjob" {
		apiVersion = "batch/v1"
	} else if resourceType == "Job" {
		apiVersion = "batch/v1"
	}

	return &types.TargetRefInfo{
		ApiVersion: apiVersion,
		Kind:       resourceType,
		Name:       versionDetail.ResourceName,
	}
}

// GetOwnerResourceUID 获取目标资源的 UID（用于设置 OwnerReference）
func GetOwnerResourceUID(
	ctx context.Context,
	client cluster.Client,
	namespace, resourceName, resourceType string,
	logger logx.Logger,
) (string, error) {
	resourceType = strings.ToUpper(resourceType)

	var uid string
	var err error

	switch resourceType {
	case "DEPLOYMENT":
		deployment, err := client.Deployment().Get(namespace, resourceName)
		if err != nil {
			return "", fmt.Errorf("获取 Deployment UID 失败: %v", err)
		}
		uid = string(deployment.UID)
	case "STATEFULSET":
		statefulSet, err := client.StatefulSet().Get(namespace, resourceName)
		if err != nil {
			return "", fmt.Errorf("获取 StatefulSet UID 失败: %v", err)
		}
		uid = string(statefulSet.UID)
	case "DAEMONSET":
		daemonSet, err := client.DaemonSet().Get(namespace, resourceName)
		if err != nil {
			return "", fmt.Errorf("获取 DaemonSet UID 失败: %v", err)
		}
		uid = string(daemonSet.UID)
	default:
		return "", fmt.Errorf("不支持的资源类型: %s", resourceType)
	}

	logger.Infof("获取资源 UID 成功: %s/%s = %s", resourceType, resourceName, uid)
	return uid, err
}

// ParseYAMLToObject 解析 YAML 到指定类型的对象
func ParseYAMLToObject(yamlStr string, obj runtime.Object) error {
	if err := yaml.Unmarshal([]byte(yamlStr), obj); err != nil {
		return fmt.Errorf("YAML 解析失败: %v", err)
	}
	return nil
}

// AddAuditLog 添加审计日志（通用方法）
func AddAuditLog(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	versionDetail *managerservice.GetOnecProjectVersionDetailResp,
	title, action, actionDetail string,
	logger logx.Logger,
) {
	username, _ := ctx.Value("username").(string)
	if username == "" {
		username = "system"
	}
	_, err := svcCtx.ManagerRpc.ProjectAuditLogAdd(ctx, &managerservice.AddOnecProjectAuditLogReq{
		ProjectId:    versionDetail.VersionId,
		Title:        title,
		ActionDetail: actionDetail,
		Status:       1,
	})
	if err != nil {
		logger.Errorf("记录审计日志失败: %v", err)
	}
}
