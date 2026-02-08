package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8sTypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateJobSpecLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateJobSpecLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateJobSpecLogic {
	return &UpdateJobSpecLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateJobSpecLogic) UpdateJobSpec(req *types.UpdateJobSpecRequest) (resp string, err error) {
	// 获取集群客户端和资源详情
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 获取原 Job Spec 配置
	oldSpec, err := client.CronJob().GetJobSpec(versionDetail.Namespace, versionDetail.ResourceName)
	if err != nil {
		l.Errorf("获取原 Job Spec 配置失败: %v", err)
		// 继续执行
	}

	// 构建更新请求
	updateReq := &k8sTypes.UpdateJobSpecRequest{
		Name:      versionDetail.ResourceName,
		Namespace: versionDetail.Namespace,
	}

	// 只传递非零值实现部分更新
	if req.Parallelism != 0 {
		updateReq.Parallelism = int32Ptr(req.Parallelism)
	}
	if req.Completions != 0 {
		updateReq.Completions = int32Ptr(req.Completions)
	}
	if req.BackoffLimit != 0 {
		updateReq.BackoffLimit = int32Ptr(req.BackoffLimit)
	}
	if req.ActiveDeadlineSeconds != 0 {
		updateReq.ActiveDeadlineSeconds = int64Ptr(req.ActiveDeadlineSeconds)
	}
	if req.TTLSecondsAfterFinished != 0 {
		updateReq.TTLSecondsAfterFinished = int32Ptr(req.TTLSecondsAfterFinished)
	}
	if req.CompletionMode != "" {
		updateReq.CompletionMode = stringPtr(req.CompletionMode)
	}
	updateReq.Suspend = boolPtr(req.Suspend)

	if req.PodReplacementPolicy != "" {
		updateReq.PodReplacementPolicy = stringPtr(req.PodReplacementPolicy)
	}
	if req.BackoffLimitPerIndex != 0 {
		updateReq.BackoffLimitPerIndex = int32Ptr(req.BackoffLimitPerIndex)
	}
	if req.MaxFailedIndexes != 0 {
		updateReq.MaxFailedIndexes = int32Ptr(req.MaxFailedIndexes)
	}

	// 转换 PodFailurePolicy
	if req.PodFailurePolicy != nil {
		updateReq.PodFailurePolicy = convertPodFailurePolicyFromTypes(req.PodFailurePolicy)
	}

	// 调用 CronJob operator 更新 Job Spec 配置
	err = client.CronJob().UpdateJobSpec(updateReq)

	// 生成变更详情
	var changeDetail string
	if oldSpec != nil {
		changeDetail = CompareJobSpec(oldSpec, updateReq)
	} else {
		changeDetail = fmt.Sprintf("Job Spec 配置变更 (无法获取原配置): 并行度: %d, 完成数: %d, 重试次数: %d",
			req.Parallelism, req.Completions, req.BackoffLimit)
	}

	if err != nil {
		l.Errorf("更新 Job Spec 配置失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改Job规格",
			fmt.Sprintf("CronJob %s/%s 修改Job规格配置失败, %s, 错误: %v", versionDetail.Namespace, versionDetail.ResourceName, changeDetail, err), 2)
		return "", fmt.Errorf("更新 Job Spec 配置失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改Job规格",
		fmt.Sprintf("CronJob %s/%s 修改Job规格配置成功, %s", versionDetail.Namespace, versionDetail.ResourceName, changeDetail), 1)
	return "更新成功", nil
}

// int32Ptr 创建 int32 指针
func int32Ptr(v int32) *int32 {
	return &v
}

// int64Ptr 创建 int64 指针
func int64Ptr(v int64) *int64 {
	return &v
}

// stringPtr 创建 string 指针
func stringPtr(v string) *string {
	return &v
}

// boolPtr 创建 bool 指针
func boolPtr(v bool) *bool {
	return &v
}

// convertPodFailurePolicyFromTypes 转换 PodFailurePolicy 类型
func convertPodFailurePolicyFromTypes(policy *types.PodFailurePolicyConfig) *k8sTypes.PodFailurePolicyConfig {
	if policy == nil {
		return nil
	}

	result := &k8sTypes.PodFailurePolicyConfig{
		Rules: make([]k8sTypes.PodFailurePolicyRule, 0, len(policy.Rules)),
	}

	for _, rule := range policy.Rules {
		k8sRule := k8sTypes.PodFailurePolicyRule{
			Action: rule.Action,
		}

		// 转换 OnExitCodes
		if rule.OnExitCodes != nil {
			containerName := rule.OnExitCodes.ContainerName
			k8sRule.OnExitCodes = &k8sTypes.PodFailurePolicyOnExitCodesRequirement{
				ContainerName: &containerName,
				Operator:      rule.OnExitCodes.Operator,
				Values:        rule.OnExitCodes.Values,
			}
		}

		// 转换 OnPodConditions
		if len(rule.OnPodConditions) > 0 {
			k8sRule.OnPodConditions = make([]k8sTypes.PodFailurePolicyOnPodConditionsPattern, 0, len(rule.OnPodConditions))
			for _, condition := range rule.OnPodConditions {
				k8sRule.OnPodConditions = append(k8sRule.OnPodConditions, k8sTypes.PodFailurePolicyOnPodConditionsPattern{
					Type:   condition.Type,
					Status: condition.Status,
				})
			}
		}

		result.Rules = append(result.Rules, k8sRule)
	}

	return result
}
