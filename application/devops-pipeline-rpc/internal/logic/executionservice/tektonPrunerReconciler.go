package executionservicelogic

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const tektonPrunerReconcileInterval = 10 * time.Minute
const tektonNativePrunerGlobalConfigMapName = "tekton-pruner-default-spec"
const tektonNativePrunerNamespaceConfigMapName = "tekton-pruner-namespace-spec"
const tektonNativePrunerOwnerLabelKey = "kube-nova.io/pipeline-id"

var tektonPrunerLock sync.Mutex

type tektonPrunerPolicy struct {
	Enabled                     bool              `json:"enabled"`
	ClusterResourceCleanup      *bool             `json:"clusterResourceCleanup"`
	PlatformRecordCleanup       *bool             `json:"platformRecordCleanup"`
	NativePrunerMode            string            `json:"nativePrunerMode"`
	NativePrunerTargetNamespace string            `json:"nativePrunerTargetNamespace"`
	NativePrunerConfigLevel     string            `json:"nativePrunerConfigLevel"`
	ResourceTypes               []string          `json:"resourceTypes"`
	Scope                       string            `json:"scope"`
	HistoryLimit                *int              `json:"historyLimit"`
	SuccessfulHistoryLimit      *int              `json:"successfulHistoryLimit"`
	FailedHistoryLimit          *int              `json:"failedHistoryLimit"`
	TTLSecondsAfterFinished     *int              `json:"ttlSecondsAfterFinished"`
	PlatformRecordRetentionDays *int              `json:"platformRecordRetentionDays"`
	PlatformRecordLimit         *int              `json:"platformRecordLimit"`
	SelectorLabels              []tektonNameValue `json:"selectorLabels"`
	SelectorAnnotations         []tektonNameValue `json:"selectorAnnotations"`
}

type tektonPrunerDagConfig struct {
	PrunerPolicy tektonPrunerPolicy `json:"prunerPolicy"`
}

// StartTektonPrunerReconciler 按 Tekton Pruner 策略清理集群资源和平台运行记录。
func StartTektonPrunerReconciler(svcCtx *svc.ServiceContext) {
	go func() {
		time.Sleep(30 * time.Second)
		ticker := time.NewTicker(tektonPrunerReconcileInterval)
		defer ticker.Stop()
		for {
			reconcileTektonPruner(context.Background(), svcCtx)
			<-ticker.C
		}
	}()
}

func reconcileTektonPruner(ctx context.Context, svcCtx *svc.ServiceContext) {
	if !tektonPrunerLock.TryLock() {
		return
	}
	defer tektonPrunerLock.Unlock()
	if !acquireTektonPrunerLock(ctx, svcCtx) {
		return
	}

	page := uint64(1)
	for {
		pipelines, total, err := svcCtx.PipelineModel.List(ctx, model.DevopsPipelineListFilter{
			EngineType: engineTekton,
			Status:     1,
			Page:       page,
			PageSize:   50,
		})
		if err != nil {
			logx.WithContext(ctx).Errorf("查询 Tekton Pruner 流水线失败: %v", err)
			return
		}
		for _, pipeline := range pipelines {
			reconcileTektonPrunerPipeline(ctx, svcCtx, pipeline)
		}
		if uint64(len(pipelines)) == 0 || page*50 >= total {
			return
		}
		page++
	}
}

func acquireTektonPrunerLock(ctx context.Context, svcCtx *svc.ServiceContext) bool {
	if svcCtx == nil || svcCtx.Cache == nil {
		return true
	}
	sum := sha1.Sum([]byte("tekton-pruner"))
	key := fmt.Sprintf("devops:tekton:pruner:%x", sum)
	ok, err := svcCtx.Cache.SetnxExCtx(ctx, key, "1", int((tektonPrunerReconcileInterval - time.Minute).Seconds()))
	if err != nil {
		logx.WithContext(ctx).Errorf("获取 Tekton Pruner 清理锁失败: %v", err)
		return false
	}
	if !ok {
		logx.WithContext(ctx).Infof("Tekton Pruner 清理已由其他实例处理")
	}
	return ok
}

func reconcileTektonPrunerPipeline(ctx context.Context, svcCtx *svc.ServiceContext, pipeline *model.DevopsPipeline) {
	if pipeline == nil || pipeline.EngineType != engineTekton {
		return
	}
	policy := parseTektonPrunerPolicy(pipeline)
	if !policy.Enabled {
		return
	}
	syncTektonNativePrunerConfig(ctx, svcCtx, pipeline, policy)
	if boolValue(policy.ClusterResourceCleanup, true) {
		pruneTektonClusterResources(ctx, svcCtx, pipeline, policy)
	}
	if boolValue(policy.PlatformRecordCleanup, true) {
		pruneTektonPlatformRecords(ctx, svcCtx, pipeline, policy)
	}
}

func parseTektonPrunerPolicy(pipeline *model.DevopsPipeline) tektonPrunerPolicy {
	var policy tektonPrunerPolicy
	if pipeline == nil {
		return policy
	}
	hasRef := false
	if content := strings.TrimSpace(pipeline.TektonPrunerPolicyRef); content != "" {
		if err := json.Unmarshal([]byte(content), &policy); err == nil {
			hasRef = true
		}
	}
	if !hasRef && strings.TrimSpace(pipeline.TektonDagConfig) != "" {
		var dag tektonPrunerDagConfig
		if err := json.Unmarshal([]byte(pipeline.TektonDagConfig), &dag); err == nil {
			policy = dag.PrunerPolicy
		}
	}
	if strings.TrimSpace(policy.Scope) == "" {
		policy.Scope = "pipeline"
	}
	if len(policy.ResourceTypes) == 0 {
		policy.ResourceTypes = []string{"PipelineRun", "TaskRun"}
	}
	return policy
}

func syncTektonNativePrunerConfig(ctx context.Context, svcCtx *svc.ServiceContext, pipeline *model.DevopsPipeline, policy tektonPrunerPolicy) {
	if pipeline == nil || !shouldApplyTektonNativePrunerConfig(policy) || strings.TrimSpace(pipeline.BuildChannelBindingID) == "" {
		return
	}
	runtime, err := buildRuntimeCached(ctx, svcCtx, pipeline.ProjectID, pipeline.SystemID, pipeline.EnvironmentID, pipeline.BuildChannelBindingID, 0, []string{"SUPER_ADMIN"})
	if err != nil {
		logx.WithContext(ctx).Errorf("解析 Tekton Pruner ConfigMap 运行上下文失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
		return
	}
	if err := syncTektonNativePrunerConfigWithRuntime(ctx, pipeline, policy, runtime); err != nil {
		logx.WithContext(ctx).Errorf("同步 Tekton Pruner ConfigMap 失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
	}
}

func syncTektonNativePrunerConfigWithRuntime(ctx context.Context, pipeline *model.DevopsPipeline, policy tektonPrunerPolicy, runtime *pipelineconfigservice.ResolvePipelineRuntimeResp) error {
	if pipeline == nil || !shouldApplyTektonNativePrunerConfig(policy) {
		return nil
	}
	if runtime == nil {
		return errorx.Msg("Tekton Pruner 运行时配置不完整")
	}
	client, err := devopstekton.NewClient(tektonRequestFromRuntime(runtime))
	if err != nil {
		return err
	}
	cm, err := tektonNativePrunerConfigMap(pipeline, policy)
	if err != nil {
		return err
	}
	return client.ApplyManagedPrunerConfigMap(ctx, cm, tektonNativePrunerOwnerLabelKey, pipeline.ID.Hex())
}

func deleteTektonNativePrunerConfig(ctx context.Context, pipeline *model.DevopsPipeline, runtime *pipelineconfigservice.ResolvePipelineRuntimeResp) error {
	if pipeline == nil {
		return nil
	}
	policy := parseTektonPrunerPolicy(pipeline)
	if !hasTektonNativePrunerConfigTarget(policy) {
		return nil
	}
	client, err := devopstekton.NewClient(tektonRequestFromRuntime(runtime))
	if err != nil {
		return err
	}
	namespace, name := tektonNativePrunerConfigMapTarget(pipeline, policy)
	if namespace == "" || name == "" {
		return nil
	}
	dataKey := "ns-config"
	if strings.TrimSpace(policy.NativePrunerMode) == "globalConfigMap" {
		dataKey = "global-config"
	}
	return client.DeleteManagedPrunerConfigMap(ctx, namespace, name, dataKey, tektonNativePrunerOwnerLabelKey, pipeline.ID.Hex())
}

func shouldApplyTektonNativePrunerConfig(policy tektonPrunerPolicy) bool {
	return policy.Enabled && hasTektonNativePrunerConfigTarget(policy)
}

func hasTektonNativePrunerConfigTarget(policy tektonPrunerPolicy) bool {
	mode := strings.TrimSpace(policy.NativePrunerMode)
	return mode != "" && mode != "disabled"
}

func pruneTektonClusterResources(ctx context.Context, svcCtx *svc.ServiceContext, pipeline *model.DevopsPipeline, policy tektonPrunerPolicy) {
	namespace := strings.TrimSpace(pipeline.TektonNamespace)
	if namespace == "" || strings.TrimSpace(pipeline.BuildChannelBindingID) == "" {
		return
	}
	runtime, err := buildRuntimeCached(ctx, svcCtx, pipeline.ProjectID, pipeline.SystemID, pipeline.EnvironmentID, pipeline.BuildChannelBindingID, 0, []string{"SUPER_ADMIN"})
	if err != nil {
		logx.WithContext(ctx).Errorf("解析 Tekton Pruner 运行上下文失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
		return
	}
	client, err := devopstekton.NewClient(tektonRequestFromRuntime(runtime))
	if err != nil {
		logx.WithContext(ctx).Errorf("创建 Tekton Pruner 客户端失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
		return
	}
	selector := tektonPrunerLabelSelector(pipeline, policy)
	deletedTaskRuns := map[string]struct{}{}
	if hasPrunerResource(policy, "PipelineRun") {
		items, err := client.ListPipelineRunResources(ctx, namespace, selector)
		if err != nil {
			logx.WithContext(ctx).Errorf("查询 Tekton PipelineRun 清理候选失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
			return
		}
		candidates := selectTektonPrunerResources(items, policy)
		for _, item := range candidates {
			if hasPrunerResource(policy, "TaskRun") {
				deleteTektonTaskRunsForPipelineRun(ctx, client, item.Namespace, item.Name, deletedTaskRuns)
			}
			if err := client.DeletePipelineRun(ctx, item.Namespace, item.Name); err != nil {
				logx.WithContext(ctx).Errorf("Pruner 删除 Tekton PipelineRun 失败: namespace=%s name=%s err=%v", item.Namespace, item.Name, err)
			}
		}
	}
	if hasPrunerResource(policy, "TaskRun") {
		pruneTektonTaskRunClusterResources(ctx, client, namespace, selector, policy, deletedTaskRuns)
	}
}

func tektonNativePrunerConfigMap(pipeline *model.DevopsPipeline, policy tektonPrunerPolicy) (*corev1.ConfigMap, error) {
	namespace, name := tektonNativePrunerConfigMapTarget(pipeline, policy)
	if namespace == "" || name == "" {
		return nil, errorx.Msg("Tekton Pruner ConfigMap 目标不完整")
	}
	key := "ns-config"
	labels := map[string]string{
		"app.kubernetes.io/part-of":     "tekton-pruner",
		"pruner.tekton.dev/config-type": "namespace",
		"kube-nova.io/managed-by":       "kube-nova",
	}
	spec := tektonNativePrunerSpec(pipeline, policy)
	if policy.NativePrunerMode == "globalConfigMap" {
		key = "global-config"
		labels["pruner.tekton.dev/config-type"] = "global"
		spec["enforcedConfigLevel"] = nativePrunerConfigLevel(policy)
	}
	raw, err := yaml.Marshal(spec)
	if err != nil {
		return nil, err
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: map[string]string{key: string(raw)},
	}, nil
}

func tektonNativePrunerConfigMapTarget(pipeline *model.DevopsPipeline, policy tektonPrunerPolicy) (string, string) {
	mode := strings.TrimSpace(policy.NativePrunerMode)
	if mode == "" || mode == "disabled" {
		return "", ""
	}
	namespace := strings.TrimSpace(policy.NativePrunerTargetNamespace)
	if mode == "globalConfigMap" {
		if namespace == "" {
			namespace = "tekton-pipelines"
		}
		return namespace, tektonNativePrunerGlobalConfigMapName
	}
	if namespace == "" && pipeline != nil {
		namespace = strings.TrimSpace(pipeline.TektonNamespace)
	}
	return namespace, tektonNativePrunerNamespaceConfigMapName
}

func tektonNativePrunerSpec(pipeline *model.DevopsPipeline, policy tektonPrunerPolicy) map[string]any {
	spec := map[string]any{}
	groups := tektonNativePrunerResourceGroups(pipeline, policy)
	if len(groups) > 0 {
		if hasPrunerResource(policy, "PipelineRun") {
			spec["pipelineRuns"] = groups
		}
		if hasPrunerResource(policy, "TaskRun") {
			spec["taskRuns"] = groups
		}
	}
	return spec
}

func tektonNativePrunerResourceGroups(pipeline *model.DevopsPipeline, policy tektonPrunerPolicy) []map[string]any {
	selector := map[string]any{}
	hasCustomSelector := false
	if labels := tektonNativePrunerNameValues(policy.SelectorLabels); len(labels) > 0 {
		selector["matchLabels"] = labels
		hasCustomSelector = true
	}
	if annotations := tektonNativePrunerNameValues(policy.SelectorAnnotations); len(annotations) > 0 {
		selector["matchAnnotations"] = annotations
		hasCustomSelector = true
	}
	if !hasCustomSelector && policy.Scope == "pipeline" && pipeline != nil && strings.TrimSpace(pipeline.TektonPipelineName) != "" {
		selector["matchLabels"] = map[string]string{
			"tekton.dev/pipeline":           strings.TrimSpace(pipeline.TektonPipelineName),
			tektonNativePrunerOwnerLabelKey: pipeline.ID.Hex(),
		}
	} else {
		ensureTektonNativePrunerOwnerLabel(selector, pipeline)
	}
	if len(selector) == 0 {
		return nil
	}
	group := map[string]any{"selector": []map[string]any{selector}}
	putNonNegativeInt(group, "historyLimit", policy.HistoryLimit)
	putNonNegativeInt(group, "successfulHistoryLimit", policy.SuccessfulHistoryLimit)
	putNonNegativeInt(group, "failedHistoryLimit", policy.FailedHistoryLimit)
	putNonNegativeInt(group, "ttlSecondsAfterFinished", policy.TTLSecondsAfterFinished)
	return []map[string]any{group}
}

func ensureTektonNativePrunerOwnerLabel(selector map[string]any, pipeline *model.DevopsPipeline) {
	if pipeline == nil {
		return
	}
	owner := pipeline.ID.Hex()
	if owner == "" {
		return
	}
	labels, _ := selector["matchLabels"].(map[string]string)
	if labels == nil {
		labels = map[string]string{}
	}
	labels[tektonNativePrunerOwnerLabelKey] = owner
	selector["matchLabels"] = labels
}

func tektonNativePrunerNameValues(items []tektonNameValue) map[string]string {
	result := map[string]string{}
	for _, item := range items {
		key := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(item.Value)
		if key != "" && value != "" {
			result[key] = value
		}
	}
	return result
}

func putNonNegativeInt(target map[string]any, key string, value *int) {
	if value != nil && *value >= 0 {
		target[key] = *value
	}
}

func nativePrunerConfigLevel(policy tektonPrunerPolicy) string {
	level := strings.TrimSpace(policy.NativePrunerConfigLevel)
	if level == "" {
		return "namespace"
	}
	return level
}

func deleteTektonTaskRunsForPipelineRun(ctx context.Context, client *devopstekton.Client, namespace, pipelineRunName string, deleted map[string]struct{}) {
	taskRuns, err := client.ListTaskRunResources(ctx, namespace, pipelineRunName, "")
	if err != nil {
		logx.WithContext(ctx).Errorf("查询 Tekton TaskRun 清理候选失败: namespace=%s pipelineRun=%s err=%v", namespace, pipelineRunName, err)
		return
	}
	for _, taskRun := range taskRuns {
		if err := client.DeleteTaskRun(ctx, taskRun.Namespace, taskRun.Name); err != nil {
			logx.WithContext(ctx).Errorf("Pruner 删除 Tekton TaskRun 失败: namespace=%s name=%s err=%v", taskRun.Namespace, taskRun.Name, err)
			continue
		}
		markDeletedPrunerResource(deleted, taskRun.Namespace, taskRun.Name)
	}
}

func pruneTektonTaskRunClusterResources(ctx context.Context, client *devopstekton.Client, namespace, selector string, policy tektonPrunerPolicy, deleted map[string]struct{}) {
	taskRuns, err := client.ListTaskRunResources(ctx, namespace, "", selector)
	if err != nil {
		logx.WithContext(ctx).Errorf("查询 Tekton TaskRun 独立清理候选失败: namespace=%s selector=%s err=%v", namespace, selector, err)
		return
	}
	for _, taskRun := range selectTektonTaskRunPrunerCandidates(taskRuns, policy, deleted) {
		if err := client.DeleteTaskRun(ctx, taskRun.Namespace, taskRun.Name); err != nil {
			logx.WithContext(ctx).Errorf("Pruner 删除 Tekton TaskRun 失败: namespace=%s name=%s err=%v", taskRun.Namespace, taskRun.Name, err)
			continue
		}
		markDeletedPrunerResource(deleted, taskRun.Namespace, taskRun.Name)
	}
}

func pruneTektonPlatformRecords(ctx context.Context, svcCtx *svc.ServiceContext, pipeline *model.DevopsPipeline, policy tektonPrunerPolicy) {
	filter := model.DevopsPipelineRunPruneFilter{FinalOnly: true}
	if policy.Scope == "namespace" {
		filter.TektonNamespace = strings.TrimSpace(pipeline.TektonNamespace)
	} else {
		filter.PipelineID = pipeline.ID.Hex()
	}
	if filter.PipelineID == "" && filter.TektonNamespace == "" {
		return
	}
	runs, err := svcCtx.PipelineRunModel.ListForPruner(ctx, filter)
	if err != nil {
		logx.WithContext(ctx).Errorf("查询 Tekton 平台运行记录清理候选失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
		return
	}
	ids := selectTektonPrunerRunIDs(runs, policy)
	if len(ids) == 0 {
		return
	}
	if err := svcCtx.RunStageModel.DeleteByRuns(ctx, ids); err != nil {
		logx.WithContext(ctx).Errorf("清理 Tekton 平台阶段记录失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
	}
	if svcCtx.ArtifactModel != nil {
		if err := svcCtx.ArtifactModel.DeleteByRuns(ctx, ids); err != nil {
			logx.WithContext(ctx).Errorf("清理 Tekton 平台产物记录失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
		}
	}
	if _, err := svcCtx.PipelineRunModel.DeleteSoftMany(ctx, ids, "tekton-pruner"); err != nil {
		logx.WithContext(ctx).Errorf("清理 Tekton 平台运行记录失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
	}
}

func selectTektonPrunerResources(items []devopstekton.PrunerResourceInfo, policy tektonPrunerPolicy) []devopstekton.PrunerResourceInfo {
	sort.SliceStable(items, func(i, j int) bool {
		return prunerResourceTime(items[i]).After(prunerResourceTime(items[j]))
	})
	result := make([]devopstekton.PrunerResourceInfo, 0)
	totalCount := 0
	successCount := 0
	failedCount := 0
	for _, item := range items {
		if !matchTektonPrunerAnnotations(item, policy) {
			continue
		}
		status := strings.TrimSpace(item.Status.Status)
		if !isFinalRunStatus(status) {
			continue
		}
		totalCount++
		deleteItem := shouldDeleteByFinishedTTL(item.FinishedAt, policy.TTLSecondsAfterFinished)
		if limitExceeded(policy.HistoryLimit, totalCount) {
			deleteItem = true
		}
		if status == "success" {
			successCount++
			if limitExceeded(policy.SuccessfulHistoryLimit, successCount) {
				deleteItem = true
			}
		} else {
			failedCount++
			if limitExceeded(policy.FailedHistoryLimit, failedCount) {
				deleteItem = true
			}
		}
		if deleteItem {
			result = append(result, item)
		}
	}
	return result
}

func selectTektonTaskRunPrunerCandidates(items []devopstekton.PrunerResourceInfo, policy tektonPrunerPolicy, deleted map[string]struct{}) []devopstekton.PrunerResourceInfo {
	candidates := selectTektonPrunerResources(items, policy)
	if len(candidates) == 0 || len(deleted) == 0 {
		return candidates
	}
	result := make([]devopstekton.PrunerResourceInfo, 0, len(candidates))
	for _, item := range candidates {
		if _, ok := deleted[prunerResourceKey(item.Namespace, item.Name)]; ok {
			continue
		}
		result = append(result, item)
	}
	return result
}

func tektonPrunerLabelSelector(pipeline *model.DevopsPipeline, policy tektonPrunerPolicy) string {
	parts := []string{"kube-nova.io/engine=tekton"}
	if pipeline != nil && policy.Scope != "namespace" {
		parts = append(parts, "kube-nova.io/pipeline-id="+pipeline.ID.Hex())
	}
	for _, item := range policy.SelectorLabels {
		key := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(item.Value)
		if key != "" && value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	return strings.Join(parts, ",")
}

func matchTektonPrunerAnnotations(item devopstekton.PrunerResourceInfo, policy tektonPrunerPolicy) bool {
	for _, selector := range policy.SelectorAnnotations {
		key := strings.TrimSpace(selector.Name)
		value := strings.TrimSpace(selector.Value)
		if key == "" {
			continue
		}
		if item.Annotations == nil || strings.TrimSpace(item.Annotations[key]) != value {
			return false
		}
	}
	return true
}

func selectTektonPrunerRunIDs(runs []*model.DevopsPipelineRun, policy tektonPrunerPolicy) []string {
	sort.SliceStable(runs, func(i, j int) bool {
		return prunerRunTime(runs[i]).After(prunerRunTime(runs[j]))
	})
	result := make([]string, 0)
	totalCount := 0
	for _, run := range runs {
		if run == nil || !isFinalRunStatus(run.Status) {
			continue
		}
		totalCount++
		deleteRun := shouldDeleteByFinishedDays(run.FinishedAt, policy.PlatformRecordRetentionDays)
		if limitExceeded(policy.PlatformRecordLimit, totalCount) {
			deleteRun = true
		}
		if deleteRun {
			result = append(result, run.ID.Hex())
		}
	}
	return result
}

func hasPrunerResource(policy tektonPrunerPolicy, resource string) bool {
	for _, item := range policy.ResourceTypes {
		if strings.EqualFold(strings.TrimSpace(item), resource) {
			return true
		}
	}
	return false
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func limitExceeded(limit *int, count int) bool {
	if limit == nil || *limit < 0 {
		return false
	}
	return count > *limit
}

func shouldDeleteByFinishedTTL(finishedAt time.Time, ttl *int) bool {
	if ttl == nil || *ttl <= 0 || finishedAt.IsZero() {
		return false
	}
	return time.Since(finishedAt) > time.Duration(*ttl)*time.Second
}

func shouldDeleteByFinishedDays(finishedAt time.Time, days *int) bool {
	if days == nil || *days <= 0 || finishedAt.IsZero() {
		return false
	}
	return time.Since(finishedAt) > time.Duration(*days)*24*time.Hour
}

func prunerResourceTime(item devopstekton.PrunerResourceInfo) time.Time {
	if !item.FinishedAt.IsZero() {
		return item.FinishedAt
	}
	return item.CreatedAt
}

func prunerResourceKey(namespace, name string) string {
	return strings.TrimSpace(namespace) + "/" + strings.TrimSpace(name)
}

func markDeletedPrunerResource(deleted map[string]struct{}, namespace, name string) {
	if deleted == nil {
		return
	}
	key := prunerResourceKey(namespace, name)
	if key == "/" {
		return
	}
	deleted[key] = struct{}{}
}

func prunerRunTime(run *model.DevopsPipelineRun) time.Time {
	if run == nil {
		return time.Time{}
	}
	if !run.FinishedAt.IsZero() {
		return run.FinishedAt
	}
	return run.CreateAt
}
