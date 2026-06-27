package executionservicelogic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"

	"github.com/zeromicro/go-zero/core/logx"
)

const pipelineRunReconcileInterval = 30 * time.Second

// StartPipelineRunReconciler 补偿服务重启后遗留的运行状态。
func StartPipelineRunReconciler(svcCtx *svc.ServiceContext) {
	go func() {
		time.Sleep(5 * time.Second)
		ticker := time.NewTicker(pipelineRunReconcileInterval)
		defer ticker.Stop()
		for {
			reconcileActivePipelineRuns(context.Background(), svcCtx)
			<-ticker.C
		}
	}()
}

func reconcileActivePipelineRuns(ctx context.Context, svcCtx *svc.ServiceContext) {
	discoverTektonWebhookPipelineRuns(ctx, svcCtx)
	for _, status := range activeRunStatuses() {
		items, _, err := svcCtx.PipelineRunModel.List(ctx, model.DevopsPipelineRunListFilter{
			Status:   status,
			Page:     1,
			PageSize: 50,
			Limit:    50,
		})
		if err != nil {
			logx.WithContext(ctx).Errorf("补偿流水线运行状态失败: status=%s err=%v", status, err)
			continue
		}
		for _, run := range items {
			reconcilePipelineRun(ctx, svcCtx, run)
		}
	}
}

func discoverTektonWebhookPipelineRuns(ctx context.Context, svcCtx *svc.ServiceContext) {
	items, _, err := svcCtx.PipelineModel.List(ctx, model.DevopsPipelineListFilter{
		EngineType: engineTekton,
		Status:     1,
		Page:       1,
		PageSize:   100,
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("发现 Tekton Webhook 运行记录失败: %v", err)
		return
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		pipeline, err := svcCtx.PipelineModel.FindOne(ctx, item.ID.Hex())
		if err != nil {
			logx.WithContext(ctx).Errorf("读取 Tekton Webhook 流水线失败: pipelineId=%s err=%v", item.ID.Hex(), err)
			continue
		}
		discoverTektonWebhookPipelineRunsForPipeline(ctx, svcCtx, pipeline)
	}
}

func discoverTektonWebhookPipelineRunsForPipeline(ctx context.Context, svcCtx *svc.ServiceContext, pipeline *model.DevopsPipeline) {
	if pipeline == nil || pipeline.EngineType != engineTekton {
		return
	}
	policy, err := parseTektonTriggerConfig(pipeline)
	if err != nil {
		logx.WithContext(ctx).Errorf("解析 Tekton Webhook 配置失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
		return
	}
	if !policy.Enabled {
		return
	}
	namespace := strings.TrimSpace(pipeline.TektonNamespace)
	if namespace == "" || strings.TrimSpace(pipeline.BuildChannelBindingID) == "" {
		return
	}
	runtime, err := buildRuntimeCached(ctx, svcCtx, pipeline.ProjectID, pipeline.SystemID, pipeline.EnvironmentID, pipeline.BuildChannelBindingID, 0, []string{"SUPER_ADMIN"})
	if err != nil {
		logx.WithContext(ctx).Errorf("解析 Tekton Webhook 运行上下文失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
		return
	}
	client, err := devopstekton.NewClient(tektonRequestFromRuntime(runtime))
	if err != nil {
		logx.WithContext(ctx).Errorf("创建 Tekton Webhook 客户端失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
		return
	}
	selector := "kube-nova.io/engine=tekton,kube-nova.io/trigger-type=webhook,kube-nova.io/pipeline-id=" + pipeline.ID.Hex()
	runs, err := client.ListPipelineRunResources(ctx, namespace, selector)
	if err != nil {
		logx.WithContext(ctx).Errorf("查询 Tekton Webhook PipelineRun 失败: pipelineId=%s err=%v", pipeline.ID.Hex(), err)
		return
	}
	for _, remote := range runs {
		if strings.TrimSpace(remote.Name) == "" {
			continue
		}
		if _, err := svcCtx.PipelineRunModel.FindByTektonPipelineRun(ctx, remote.Namespace, remote.Name); err == nil {
			continue
		} else if !errors.Is(err, model.ErrNotFound) {
			logx.WithContext(ctx).Errorf("查询 Tekton Webhook 平台运行记录失败: namespace=%s name=%s err=%v", remote.Namespace, remote.Name, err)
			continue
		}
		run, err := createDiscoveredTektonWebhookRun(ctx, svcCtx, pipeline, remote)
		if err != nil {
			logx.WithContext(ctx).Errorf("创建 Tekton Webhook 平台运行记录失败: namespace=%s name=%s err=%v", remote.Namespace, remote.Name, err)
			continue
		}
		if err := syncTektonRunSnapshot(ctx, svcCtx, run, 0, []string{"SUPER_ADMIN"}, "tekton-webhook"); err != nil {
			logx.WithContext(ctx).Errorf("同步 Tekton Webhook 运行状态失败: runId=%s err=%v", run.ID.Hex(), err)
		}
	}
}

func createDiscoveredTektonWebhookRun(ctx context.Context, svcCtx *svc.ServiceContext, pipeline *model.DevopsPipeline, remote devopstekton.PrunerResourceInfo) (*model.DevopsPipelineRun, error) {
	params := discoveredTektonWebhookParams(pipeline, remote)
	paramBytes, _ := json.Marshal(params)
	env := map[string]string{
		"PROJECT_CODE":      pipeline.ProjectCode,
		"SYSTEM_CODE":       pipeline.SystemCode,
		"SYSTEM_NAME":       pipeline.SystemName,
		"PIPELINE_CODE":     pipeline.Code,
		"PIPELINE_NAME":     pipeline.Name,
		"PIPELINE_ENV":      pipeline.EnvironmentCode,
		"PIPELINE_ENV_NAME": pipeline.EnvironmentName,
	}
	if len(remote.Workspaces) > 0 {
		env["TEKTON_WORKSPACES"] = strings.Join(remote.Workspaces, ",")
	}
	envBytes, _ := json.Marshal(env)
	status := strings.TrimSpace(remote.Status.Status)
	if status == "" {
		status = "running"
	}
	startedAt := remote.Status.StartedAt
	if startedAt.IsZero() {
		startedAt = remote.CreatedAt
	}
	run := &model.DevopsPipelineRun{
		ProjectID:                     pipeline.ProjectID,
		ProjectName:                   pipeline.ProjectName,
		ProjectCode:                   pipeline.ProjectCode,
		SystemID:                      pipeline.SystemID,
		SystemName:                    pipeline.SystemName,
		SystemCode:                    pipeline.SystemCode,
		EnvironmentID:                 pipeline.EnvironmentID,
		EnvironmentName:               pipeline.EnvironmentName,
		EnvironmentCode:               pipeline.EnvironmentCode,
		PipelineID:                    pipeline.ID.Hex(),
		PipelineName:                  pipeline.Name,
		PipelineCode:                  pipeline.Code,
		EngineType:                    pipeline.EngineType,
		TemplateID:                    pipeline.TemplateID,
		BuildChannelBindingID:         pipeline.BuildChannelBindingID,
		JenkinsJobName:                pipeline.TektonPipelineName,
		JenkinsJobFullName:            pipeline.TektonNamespace + "/" + pipeline.TektonPipelineName,
		TektonNamespace:               remote.Namespace,
		TektonPipelineName:            pipeline.TektonPipelineName,
		TektonPipelineRunName:         remote.Name,
		PipelineSnapshot:              remote.Yaml,
		TektonPipelineYamlSnapshot:    pipeline.TektonPipelineYamlSnapshot,
		TektonPipelineRunYamlSnapshot: remote.Yaml,
		TriggerType:                   "webhook",
		TriggerUsername:               "Tekton Trigger",
		Status:                        status,
		ParamsSnapshot:                string(paramBytes),
		EnvSnapshot:                   string(envBytes),
		StartedAt:                     startedAt,
		FinishedAt:                    remote.Status.FinishedAt,
		CreatedBy:                     "tekton-webhook",
		UpdatedBy:                     "tekton-webhook",
	}
	if run.TektonPipelineName == "" {
		run.TektonPipelineName = tektonPipelineName(pipeline.Code)
	}
	if isFinalRunStatus(status) {
		if run.FinishedAt.IsZero() {
			run.FinishedAt = time.Now()
		}
		if strings.TrimSpace(remote.Status.Message) != "" && status != "success" {
			run.ErrorMessage = remote.Status.Message
		}
	} else {
		run.FinishedAt = time.Time{}
	}
	if !run.StartedAt.IsZero() {
		end := time.Now()
		if !run.FinishedAt.IsZero() {
			end = run.FinishedAt
		}
		run.DurationSeconds = int64(end.Sub(run.StartedAt).Seconds())
		if run.DurationSeconds < 0 {
			run.DurationSeconds = 0
		}
	}
	if err := svcCtx.PipelineRunModel.Insert(ctx, run); err != nil {
		return nil, err
	}
	if err := createRunStagesForPipeline(ctx, svcCtx, run.ID.Hex(), pipeline, nil); err != nil {
		return nil, err
	}
	if err := createScanPlanSnapshot(ctx, svcCtx, run, pipeline, params, "tekton-webhook"); err != nil {
		logx.WithContext(ctx).Errorf("创建 Tekton Webhook 扫描计划快照失败: runId=%s err=%v", run.ID.Hex(), err)
	}
	return run, nil
}

func discoveredTektonWebhookParams(pipeline *model.DevopsPipeline, remote devopstekton.PrunerResourceInfo) map[string]string {
	result := defaultPipelineRunParams(pipeline)
	for key, value := range remote.Params {
		name := strings.TrimSpace(key)
		if name != "" {
			result[name] = value
		}
	}
	return result
}

func defaultPipelineRunParams(pipeline *model.DevopsPipeline) map[string]string {
	result := make(map[string]string)
	if pipeline == nil {
		return result
	}
	for _, item := range pipeline.Params {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		result[code] = strings.TrimSpace(firstNotBlank(item.CurrentValue, item.DefaultValue))
	}
	return result
}

func reconcilePipelineRun(ctx context.Context, svcCtx *svc.ServiceContext, run *model.DevopsPipelineRun) {
	if run == nil || isFinalRunStatus(run.Status) {
		return
	}
	if run.EngineType == engineTekton {
		if err := syncTektonRunSnapshot(ctx, svcCtx, run, 0, []string{"SUPER_ADMIN"}, "system"); err != nil {
			logx.WithContext(ctx).Errorf("补偿 Tekton 运行状态失败: runId=%s err=%v", run.ID.Hex(), err)
		}
		return
	}
	if strings.TrimSpace(run.JenkinsQueueID) == "" && run.JenkinsBuildNumber <= 0 {
		if time.Since(run.StartedAt) > 10*time.Minute {
			_ = markRunFinal(ctx, svcCtx, run, "failed", "运行记录缺少 Jenkins 队列编号，请重新运行流水线", "system")
		}
		return
	}
	runtime, err := buildRuntime(ctx, svcCtx, run.ProjectID, run.SystemID, run.EnvironmentID, run.BuildChannelBindingID, 0, []string{"SUPER_ADMIN"})
	if err != nil {
		logx.WithContext(ctx).Errorf("补偿流水线运行上下文失败: runId=%s err=%v", run.ID.Hex(), err)
		return
	}
	manager := jenkinsManagerFromRuntime(runtime)
	ready, err := ensureRunBuildNumber(ctx, svcCtx, run, manager, "system")
	if err != nil {
		logx.WithContext(ctx).Errorf("补偿 Jenkins 构建号失败: runId=%s err=%v", run.ID.Hex(), err)
		return
	}
	if ready {
		if err := syncRunStatusFromJenkins(ctx, svcCtx, run, manager, "system"); err != nil {
			logx.WithContext(ctx).Errorf("补偿 Jenkins 构建状态失败: runId=%s buildNumber=%d err=%v", run.ID.Hex(), run.JenkinsBuildNumber, err)
		}
	}
}
