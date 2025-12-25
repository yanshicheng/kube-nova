package autoscaling

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/logic"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8stypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type VPAGetDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 VPA 详情
func NewVPAGetDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VPAGetDetailLogic {
	return &VPAGetDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *VPAGetDetailLogic) VPAGetDetail(req *types.VersionIdRequest) (resp *types.VPADetail, err error) {
	if req == nil {
		l.Error("VPAGetDetail: 请求参数为 nil")
		return nil, fmt.Errorf("请求参数不能为空")
	}

	client, versionDetail, err := logic.GetClusterClientAndVersion(l.ctx, l.svcCtx, req.VersionId, l.Logger)
	if err != nil {
		return nil, err
	}

	if client == nil {
		l.Error("集群客户端为空")
		return nil, fmt.Errorf("集群客户端为空")
	}

	if versionDetail == nil {
		l.Error("版本详情为空")
		return nil, fmt.Errorf("版本详情为空")
	}

	vpaOperator := client.VPA()
	if vpaOperator == nil {
		l.Error("VPA 操作器为空")
		return nil, fmt.Errorf("VPA 操作器未初始化")
	}

	// 查询对应的资源
	var detail *k8stypes.VPADetail
	switch strings.ToLower(versionDetail.ResourceType) {
	case "deployment":
		deployment, err := client.Deployment().Get(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 Deployment 详情失败: %v", err)
			l.Errorf("Deployment %s/%s 不存在，无法查询 VPA", versionDetail.Namespace, versionDetail.ResourceName)
			return nil, nil
		}

		if deployment == nil {
			l.Errorf("Deployment %s/%s 为空", versionDetail.Namespace, versionDetail.ResourceName)
			return nil, nil
		}

		detail, err = vpaOperator.GetDetailByTargetRef(versionDetail.Namespace, k8stypes.TargetRefInfo{
			APIVersion: deployment.APIVersion,
			Kind:       deployment.Kind,
			Name:       versionDetail.ResourceName,
		})
		if err != nil {
			l.Infof("资源 %s/%s 没有关联的 VPA", versionDetail.Namespace, versionDetail.ResourceName)
			return nil, nil
		}

	case "statefulset":
		statefulSet, err := client.StatefulSet().Get(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 StatefulSet 详情失败: %v", err)
			l.Errorf("StatefulSet %s/%s 不存在，无法查询 VPA", versionDetail.Namespace, versionDetail.ResourceName)
			return nil, nil
		}

		if statefulSet == nil {
			l.Errorf("StatefulSet %s/%s 为空", versionDetail.Namespace, versionDetail.ResourceName)
			return nil, nil
		}

		detail, err = vpaOperator.GetDetailByTargetRef(versionDetail.Namespace, k8stypes.TargetRefInfo{
			APIVersion: statefulSet.APIVersion,
			Kind:       statefulSet.Kind,
			Name:       versionDetail.ResourceName,
		})
		if err != nil {
			l.Infof("资源 %s/%s 没有关联的 VPA", versionDetail.Namespace, versionDetail.ResourceName)
			return nil, nil
		}

	case "daemonset":
		daemonSet, err := client.DaemonSet().Get(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 DaemonSet 详情失败: %v", err)
			l.Errorf("DaemonSet %s/%s 不存在，无法查询 VPA", versionDetail.Namespace, versionDetail.ResourceName)
			return nil, nil
		}

		if daemonSet == nil {
			l.Errorf("DaemonSet %s/%s 为空", versionDetail.Namespace, versionDetail.ResourceName)
			return nil, nil
		}

		detail, err = vpaOperator.GetDetailByTargetRef(versionDetail.Namespace, k8stypes.TargetRefInfo{
			APIVersion: daemonSet.APIVersion,
			Kind:       daemonSet.Kind,
			Name:       versionDetail.ResourceName,
		})
		if err != nil {
			l.Infof("资源 %s/%s 没有关联的 VPA", versionDetail.Namespace, versionDetail.ResourceName)
			return nil, nil
		}

	default:
		l.Errorf("不支持的资源类型: %s，跳过 VPA 查询", versionDetail.ResourceType)
		return nil, nil
	}

	if detail == nil {
		l.Infof("资源 %s/%s 没有关联的 VPA", versionDetail.Namespace, versionDetail.ResourceName)
		return nil, nil
	}

	// 转换
	resp = l.convertVPADetail(detail)
	l.Infof("获取 VPA 详情成功: %s/%s", detail.Namespace, detail.Name)
	return resp, nil
}

// 转换 VPADetail
func (l *VPAGetDetailLogic) convertVPADetail(detail *k8stypes.VPADetail) *types.VPADetail {
	if detail == nil {
		l.Error("convertVPADetail: detail 为 nil")
		return nil
	}

	result := &types.VPADetail{
		Name:              detail.Name,
		Namespace:         detail.Namespace,
		TargetRef:         l.convertTargetRefInfo(detail.TargetRef),
		Age:               detail.Age,
		CreationTimestamp: detail.CreationTimestamp,
	}

	// 处理 UpdatePolicy
	if detail.UpdatePolicy != nil {
		result.UpdatePolicy = l.convertVPAUpdatePolicy(detail.UpdatePolicy)
	}

	// 处理 ResourcePolicy
	result.ResourcePolicy = l.convertVPAResourcePolicy(detail.ResourcePolicy)

	// 处理 Recommendation
	result.Recommendation = l.convertVPARecommendation(detail.Recommendation)

	// 处理 Conditions
	if len(detail.Conditions) > 0 {
		result.Conditions = l.convertVPAConditions(detail.Conditions)
	}

	// 处理 Labels 和 Annotations
	if len(detail.Labels) > 0 {
		result.Labels = detail.Labels
	}

	if len(detail.Annotations) > 0 {
		result.Annotations = detail.Annotations
	}

	return result
}

// 转换 TargetRefInfo
func (l *VPAGetDetailLogic) convertTargetRefInfo(ref k8stypes.TargetRefInfo) types.TargetRefInfo {
	return types.TargetRefInfo{
		Kind:       ref.Kind,
		Name:       ref.Name,
		ApiVersion: ref.APIVersion,
	}
}

// 转换 VPAUpdatePolicy
func (l *VPAGetDetailLogic) convertVPAUpdatePolicy(policy *k8stypes.VPAUpdatePolicy) types.VPAUpdatePolicy {
	result := types.VPAUpdatePolicy{
		UpdateMode: policy.UpdateMode,
	}

	if policy.MinReplicas != nil {
		result.MinReplicas = *policy.MinReplicas
	}

	return result
}

// 转换 VPAResourcePolicy
func (l *VPAGetDetailLogic) convertVPAResourcePolicy(policy k8stypes.VPAResourcePolicyConfig) types.VPAResourcePolicyConfig {
	result := types.VPAResourcePolicyConfig{}

	if len(policy.ContainerPolicies) > 0 {
		result.ContainerPolicies = make([]types.VPAResourcePolicy, 0, len(policy.ContainerPolicies))
		for _, cp := range policy.ContainerPolicies {
			result.ContainerPolicies = append(result.ContainerPolicies, types.VPAResourcePolicy{
				ContainerName: cp.ContainerName,
				Mode:          cp.Mode,
				MinAllowed: types.VPAResourceConstraints{
					Cpu:    cp.MinAllowed.CPU,
					Memory: cp.MinAllowed.Memory,
				},
				MaxAllowed: types.VPAResourceConstraints{
					Cpu:    cp.MaxAllowed.CPU,
					Memory: cp.MaxAllowed.Memory,
				},
				ControlledResources: cp.ControlledResources,
				ControlledValues:    cp.ControlledValues,
			})
		}
	}

	return result
}

// 转换 VPARecommendation
func (l *VPAGetDetailLogic) convertVPARecommendation(rec k8stypes.VPARecommendationConfig) types.VPARecommendationConfig {
	result := types.VPARecommendationConfig{}

	if len(rec.ContainerRecommendations) > 0 {
		result.ContainerRecommendations = make([]types.VPARecommendation, 0, len(rec.ContainerRecommendations))
		for _, cr := range rec.ContainerRecommendations {
			result.ContainerRecommendations = append(result.ContainerRecommendations, types.VPARecommendation{
				ContainerName: cr.ContainerName,
				Target: types.VPAResourceRecommendation{
					Cpu:    cr.Target.CPU,
					Memory: cr.Target.Memory,
				},
				LowerBound: types.VPAResourceRecommendation{
					Cpu:    cr.LowerBound.CPU,
					Memory: cr.LowerBound.Memory,
				},
				UpperBound: types.VPAResourceRecommendation{
					Cpu:    cr.UpperBound.CPU,
					Memory: cr.UpperBound.Memory,
				},
				UncappedTarget: types.VPAResourceRecommendation{
					Cpu:    cr.UncappedTarget.CPU,
					Memory: cr.UncappedTarget.Memory,
				},
			})
		}
	}

	return result
}

// 转换 VPAConditions
func (l *VPAGetDetailLogic) convertVPAConditions(conditions []k8stypes.VPACondition) []types.VPACondition {
	if len(conditions) == 0 {
		return nil
	}

	result := make([]types.VPACondition, 0, len(conditions))
	for _, c := range conditions {
		result = append(result, types.VPACondition{
			Type:               c.Type,
			Status:             c.Status,
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime,
		})
	}
	return result
}
