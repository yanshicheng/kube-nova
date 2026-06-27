package types

import (
	"encoding/json"
	"testing"
)

func TestMetadataJSONUsesAPIVersionForProtocolChannels(t *testing.T) {
	raw := MetadataJSON(Metadata{ProductName: "GitHub", APIVersion: "v3"})

	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("MetadataJSON unmarshal failed: %v", err)
	}
	if got := data["version"]; got != "v3" {
		t.Fatalf("version = %v, want %q", got, "v3")
	}
}

func TestMetadataJSONDoesNotUseAPIVersionForGitLab(t *testing.T) {
	raw := MetadataJSON(Metadata{ProductName: "GitLab", APIVersion: "v4"})

	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("MetadataJSON unmarshal failed: %v", err)
	}
	if _, ok := data["version"]; ok {
		t.Fatalf("GitLab apiVersion should not be used as product version: %s", raw)
	}
}
