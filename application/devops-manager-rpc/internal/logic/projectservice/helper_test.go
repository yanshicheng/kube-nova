package projectservicelogic

import (
	"encoding/json"
	"testing"
)

func TestNormalizeProjectMetadataJSONUsesAPIVersionForProtocolChannel(t *testing.T) {
	normalized := normalizeProjectMetadataJSON(`{"productName":"Tekton","apiVersion":"tekton.dev/v1"}`)
	var data map[string]any
	if err := json.Unmarshal([]byte(normalized), &data); err != nil {
		t.Fatalf("normalizeProjectMetadataJSON unmarshal failed: %v", err)
	}
	if got := data["version"]; got != "tekton.dev/v1" {
		t.Fatalf("version = %v, want %q", got, "tekton.dev/v1")
	}
}

func TestNormalizeProjectMetadataJSONKeepsGitLabAPIVersionOutOfVersion(t *testing.T) {
	normalized := normalizeProjectMetadataJSON(`{"productName":"GitLab","apiVersion":"v4"}`)
	var data map[string]any
	if err := json.Unmarshal([]byte(normalized), &data); err != nil {
		t.Fatalf("normalizeProjectMetadataJSON unmarshal failed: %v", err)
	}
	if _, ok := data["version"]; ok {
		t.Fatalf("GitLab apiVersion should not be used as product version: %s", normalized)
	}
}
