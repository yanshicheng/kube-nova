package executionservicelogic

import (
	"strings"
	"testing"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestRenderTektonTriggerResourcesIncludesWebhookLabelsInterceptorAndIngress(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:                 bson.NewObjectID(),
		Code:               "app-build",
		TektonPipelineName: "app-build",
	}
	policy := tektonTriggerConfig{
		Enabled:             true,
		APIVersion:          "triggers.tekton.dev/v1beta1",
		Provider:            "github",
		EventListenerName:   "app-build-el",
		TriggerName:         "app-build-trigger",
		TriggerBindingName:  "app-build-binding",
		TriggerTemplateName: "app-build-template",
		ServiceType:         "ClusterIP",
		IngressHost:         "hooks.example.com",
		WebhookPath:         "/hook/app-build",
		IngressClassName:    "nginx",
		TLSSecretName:       "hooks-tls",
		SecretName:          "github-webhook",
		SecretKey:           "token",
		EventTypes:          []string{"push"},
		BindingParams:       []tektonTriggerParam{{Name: "revision", Value: "$(body.head_commit.id)"}},
	}

	contents, err := renderTektonTriggerResources(pipeline, tektonBindingConfig{Namespace: "devops-demo"}, policy)
	if err != nil {
		t.Fatalf("renderTektonTriggerResources error: %v", err)
	}
	out := strings.Join(contents, "\n---\n")
	assertContains(t, out, "kube-nova.io/trigger-type: webhook")
	assertContains(t, out, "interceptors:")
	assertContains(t, out, "name: github")
	assertContains(t, out, "secretName: github-webhook")
	assertContains(t, out, "kind: Ingress")
	assertContains(t, out, "ingressClassName: nginx")
	assertContains(t, out, "secretName: hooks-tls")
	assertContains(t, out, "name: el-app-build-el")
	assertContains(t, out, "path: /hook/app-build")
}

func TestDefaultPipelineRunParamsUsesCurrentOrDefaultValue(t *testing.T) {
	params := defaultPipelineRunParams(&model.DevopsPipeline{
		Params: []model.PipelineParam{
			{Code: "branch", CurrentValue: "release", DefaultValue: "main"},
			{Code: "env", DefaultValue: "prod"},
			{Code: "empty"},
		},
	})

	if params["branch"] != "release" {
		t.Fatalf("branch should use current value, got %q", params["branch"])
	}
	if params["env"] != "prod" {
		t.Fatalf("env should use default value, got %q", params["env"])
	}
	if params["empty"] != "" {
		t.Fatalf("empty should be kept as empty string, got %q", params["empty"])
	}
}

func TestParseTektonTriggerConfigFallsBackToDagConfig(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "dag-trigger",
		TektonDagConfig: `{
			"triggerConfig": {
				"enabled": true,
				"eventListenerName": "dag-trigger-el",
				"triggerName": "dag-trigger",
				"triggerBindingName": "dag-trigger-binding",
				"triggerTemplateName": "dag-trigger-template",
				"serviceType": "NodePort"
			}
		}`,
	}

	policy, err := parseTektonTriggerConfig(pipeline)
	if err != nil {
		t.Fatalf("parseTektonTriggerConfig error: %v", err)
	}
	if !policy.Enabled || policy.EventListenerName != "dag-trigger-el" || policy.ServiceType != "NodePort" {
		t.Fatalf("unexpected policy from dag config: %#v", policy)
	}
}

func TestHasTektonTriggerConfigUsesDagTriggerConfig(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:   bson.NewObjectID(),
		Code: "dag-trigger",
		TektonDagConfig: `{
			"triggerConfig": {
				"enabled": true,
				"eventListenerName": "dag-trigger-el"
			}
		}`,
	}

	if !hasTektonTriggerConfig(pipeline) {
		t.Fatalf("dag triggerConfig should be treated as trigger config")
	}
	if tektonTriggerConfigSignature(pipeline) == "" {
		t.Fatalf("dag triggerConfig signature should not be empty")
	}
}

func TestHasTektonTriggerConfigIgnoresMissingDagTriggerConfig(t *testing.T) {
	pipeline := &model.DevopsPipeline{
		ID:              bson.NewObjectID(),
		Code:            "dag-trigger",
		TektonDagConfig: `{"nodes":[]}`,
	}

	if hasTektonTriggerConfig(pipeline) {
		t.Fatalf("missing dag triggerConfig should not be treated as trigger config")
	}
}
