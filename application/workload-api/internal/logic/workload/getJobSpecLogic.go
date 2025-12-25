package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetJobSpecLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetJobSpecLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetJobSpecLogic {
	return &GetJobSpecLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetJobSpecLogic) GetJobSpec(req *types.DefaultIdRequest) (resp *types.JobSpecConfig, err error) {
	// 获取集群客户端和资源详情
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 调用 CronJob operator 获取 Job Spec 配置
	jobSpec, err := client.CronJob().GetJobSpec(versionDetail.Namespace, versionDetail.ResourceName)
	if err != nil {
		l.Errorf("获取 Job Spec 配置失败: %v", err)
		return nil, fmt.Errorf("获取 Job Spec 配置失败")
	}

	// 类型转换：从 k8smanager types (*int32) 转换为 API types (int32)
	resp = &types.JobSpecConfig{
		// 指针转值 - 如果为 nil 则使用默认值 0
		Parallelism:             derefInt32(jobSpec.Parallelism),
		Completions:             derefInt32(jobSpec.Completions),
		BackoffLimit:            derefInt32(jobSpec.BackoffLimit),
		ActiveDeadlineSeconds:   derefInt64(jobSpec.ActiveDeadlineSeconds),
		TTLSecondsAfterFinished: derefInt32(jobSpec.TTLSecondsAfterFinished),
		BackoffLimitPerIndex:    derefInt32(jobSpec.BackoffLimitPerIndex),
		MaxFailedIndexes:        derefInt32(jobSpec.MaxFailedIndexes),

		// 值类型直接赋值
		CompletionMode:       jobSpec.CompletionMode,
		Suspend:              jobSpec.Suspend,
		PodReplacementPolicy: jobSpec.PodReplacementPolicy,
	}

	// 转换 PodFailurePolicy
	if jobSpec.PodFailurePolicy != nil {
		resp.PodFailurePolicy = convertPodFailurePolicyToTypes(jobSpec.PodFailurePolicy)
	}

	return resp, nil
}

// derefInt32 解引用 *int32，如果为 nil 返回 0
func derefInt32(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

// derefInt64 解引用 *int64，如果为 nil 返回 0
func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

// convertPodFailurePolicyToTypes 转换 PodFailurePolicy 类型
func convertPodFailurePolicyToTypes(policy *types2.PodFailurePolicyConfig) *types.PodFailurePolicyConfig {
	if policy == nil {
		return nil
	}

	result := &types.PodFailurePolicyConfig{
		Rules: make([]types.PodFailurePolicyRule, 0, len(policy.Rules)),
	}

	for _, rule := range policy.Rules {
		typeRule := types.PodFailurePolicyRule{
			Action: rule.Action,
		}

		// 转换 OnExitCodes
		if rule.OnExitCodes != nil {
			typeRule.OnExitCodes = &types.PodFailurePolicyOnExitCodesRequirement{
				ContainerName: derefString(rule.OnExitCodes.ContainerName), // 指针转值
				Operator:      rule.OnExitCodes.Operator,
				Values:        rule.OnExitCodes.Values,
			}
		}

		// 转换 OnPodConditions
		if len(rule.OnPodConditions) > 0 {
			typeRule.OnPodConditions = make([]types.PodFailurePolicyOnPodConditionsPattern, 0, len(rule.OnPodConditions))
			for _, condition := range rule.OnPodConditions {
				typeRule.OnPodConditions = append(typeRule.OnPodConditions, types.PodFailurePolicyOnPodConditionsPattern{
					Type:   condition.Type,
					Status: condition.Status,
				})
			}
		}

		result.Rules = append(result.Rules, typeRule)
	}

	return result
}

// derefString 解引用 *string，如果为 nil 返回空字符串
func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
