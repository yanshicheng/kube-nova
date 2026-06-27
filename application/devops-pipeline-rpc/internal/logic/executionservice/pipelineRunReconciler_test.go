package executionservicelogic

import (
	"testing"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
)

func TestDiscoveredTektonWebhookParamsUsesRemoteParams(t *testing.T) {
	params := discoveredTektonWebhookParams(&model.DevopsPipeline{
		Params: []model.PipelineParam{
			{Code: "branch", DefaultValue: "main"},
			{Code: "image", DefaultValue: "old"},
		},
	}, devopstekton.PrunerResourceInfo{
		Params: map[string]string{
			"branch": "release",
			"image":  "registry.example.com/app:1",
		},
	})

	if params["branch"] != "release" {
		t.Fatalf("branch = %q, want release", params["branch"])
	}
	if params["image"] != "registry.example.com/app:1" {
		t.Fatalf("image = %q, want remote image", params["image"])
	}
}
