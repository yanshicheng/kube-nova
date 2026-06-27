package executionservicelogic

import (
	"strings"
	"testing"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
)

func TestValidateTektonPolicyConfigsRejectsInvalidSchedule(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		EngineType: engineTekton,
		TektonTriggerConfig: `{
			"scheduleConfig": {
				"enabled": true,
				"cron": "bad cron",
				"timezone": "Asia/Shanghai"
			}
		}`,
	}

	err := validateTektonPolicyConfigs(pipeline)
	if err == nil || !strings.Contains(err.Error(), "Cron") {
		t.Fatalf("invalid schedule should be rejected, got %v", err)
	}
}

func TestValidateTektonPolicyConfigsRejectsInvalidPruner(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		EngineType: engineTekton,
		TektonPrunerPolicyRef: `{
			"enabled": true,
			"resourceTypes": ["PipelineRun"],
			"historyLimit": -1
		}`,
	}

	err := validateTektonPolicyConfigs(pipeline)
	if err == nil || !strings.Contains(err.Error(), "historyLimit") {
		t.Fatalf("invalid pruner should be rejected, got %v", err)
	}
}

func TestValidateTektonPolicyConfigsRejectsInvalidNativePrunerNamespace(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		EngineType: engineTekton,
		TektonPrunerPolicyRef: `{
			"enabled": true,
			"nativePrunerMode": "namespaceConfigMap",
			"nativePrunerTargetNamespace": "Bad_Namespace",
			"nativePrunerConfigLevel": "namespace",
			"resourceTypes": ["PipelineRun"]
		}`,
	}

	err := validateTektonPolicyConfigs(pipeline)
	if err == nil || !strings.Contains(err.Error(), "Namespace") {
		t.Fatalf("invalid native pruner namespace should be rejected, got %v", err)
	}
}

func TestValidateTektonPolicyConfigsRejectsInvalidNativePrunerMode(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		EngineType: engineTekton,
		TektonPrunerPolicyRef: `{
			"enabled": true,
			"nativePrunerMode": "tektonPruner",
			"resourceTypes": ["PipelineRun"]
		}`,
	}

	err := validateTektonPolicyConfigs(pipeline)
	if err == nil || !strings.Contains(err.Error(), "模式") {
		t.Fatalf("invalid native pruner mode should be rejected, got %v", err)
	}
}

func TestValidateTektonPolicyConfigsRejectsInvalidNativePrunerConfigLevel(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		EngineType: engineTekton,
		TektonPrunerPolicyRef: `{
			"enabled": true,
			"nativePrunerMode": "globalConfigMap",
			"nativePrunerConfigLevel": "cluster",
			"resourceTypes": ["PipelineRun"]
		}`,
	}

	err := validateTektonPolicyConfigs(pipeline)
	if err == nil || !strings.Contains(err.Error(), "配置级别") {
		t.Fatalf("invalid native pruner config level should be rejected, got %v", err)
	}
}

func TestValidateTektonPolicyConfigsRejectsInvalidTrigger(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		Code:       "build",
		EngineType: engineTekton,
		TektonTriggerConfig: `{
			"enabled": true,
			"eventListenerName": "build-el",
			"triggerName": "build-trigger",
			"triggerBindingName": "build-binding",
			"triggerTemplateName": "build-template",
			"interceptorType": "cel"
		}`,
	}

	err := validateTektonPolicyConfigs(pipeline)
	if err == nil || !strings.Contains(err.Error(), "CEL") {
		t.Fatalf("invalid trigger should be rejected, got %v", err)
	}
}

func TestValidateTektonPolicyConfigsRejectsInvalidRunPolicyYaml(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		Code:       "build",
		EngineType: engineTekton,
		TektonRunPolicy: `{
			"podTemplateRaw": "- bad",
			"taskRunSpecsRaw": "- pipelineTaskName: build"
		}`,
	}

	err := validateTektonPolicyConfigs(pipeline)
	if err == nil || !strings.Contains(err.Error(), "podTemplate") {
		t.Fatalf("invalid run policy YAML should be rejected, got %v", err)
	}
}

func TestValidateTektonPolicyConfigsRejectsInvalidRunPolicyNameValue(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		Code:       "build",
		EngineType: engineTekton,
		TektonRunPolicy: `{
			"labels": [{"name": "team"}]
		}`,
	}

	err := validateTektonPolicyConfigs(pipeline)
	if err == nil || !strings.Contains(err.Error(), "名称和值") {
		t.Fatalf("invalid run policy name value should be rejected, got %v", err)
	}
}

func TestValidateTektonPolicyConfigsDoesNotValidateJenkinsRunPolicy(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		Code:       "build",
		EngineType: engineJenkins,
		TektonRunPolicy: `{
			"podTemplateRaw": "- bad"
		}`,
	}

	if err := validateTektonPolicyConfigs(pipeline); err != nil {
		t.Fatalf("jenkins pipeline should not validate tekton run policy: %v", err)
	}
}

func TestValidateTektonPolicyConfigsRejectsInvalidDagTrigger(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		Code:       "build",
		EngineType: engineTekton,
		TektonDagConfig: `{
			"triggerConfig": {
				"enabled": true,
				"eventListenerName": "build-el",
				"triggerName": "build-trigger",
				"triggerBindingName": "build-binding",
				"triggerTemplateName": "build-template",
				"interceptorType": "cel"
			}
		}`,
	}

	err := validateTektonPolicyConfigs(pipeline)
	if err == nil || !strings.Contains(err.Error(), "CEL") {
		t.Fatalf("invalid dag trigger should be rejected, got %v", err)
	}
}

func TestShouldApplyTektonNativePrunerConfigRequiresEnabled(t *testing.T) {
	if shouldApplyTektonNativePrunerConfig(tektonPrunerPolicy{Enabled: false, NativePrunerMode: "namespaceConfigMap"}) {
		t.Fatalf("disabled native pruner should not be applied")
	}
	if !shouldApplyTektonNativePrunerConfig(tektonPrunerPolicy{Enabled: true, NativePrunerMode: "namespaceConfigMap"}) {
		t.Fatalf("enabled native pruner should be applied")
	}
}

func TestValidateTektonPolicyConfigsAllowsValidAdvancedPolicies(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		Code:       "build",
		EngineType: engineTekton,
		TektonTriggerConfig: `{
			"enabled": true,
			"eventListenerName": "build-el",
			"triggerName": "build-trigger",
			"triggerBindingName": "build-binding",
			"triggerTemplateName": "build-template",
			"serviceType": "ClusterIP",
			"bindingParams": [{"name": "branch", "value": "$(body.branch)"}],
			"scheduleConfig": {
				"enabled": true,
				"cron": "0 2 * * *",
				"timezone": "Asia/Shanghai",
				"concurrencyPolicy": "skip"
			}
		}`,
		TektonPrunerPolicyRef: `{
			"enabled": true,
			"nativePrunerMode": "namespaceConfigMap",
			"nativePrunerTargetNamespace": "tekton-pipelines",
			"nativePrunerConfigLevel": "namespace",
			"resourceTypes": ["PipelineRun", "TaskRun"],
			"scope": "pipeline",
			"historyLimit": 10
		}`,
	}

	if err := validateTektonPolicyConfigs(pipeline); err != nil {
		t.Fatalf("valid policies should pass: %v", err)
	}
}
