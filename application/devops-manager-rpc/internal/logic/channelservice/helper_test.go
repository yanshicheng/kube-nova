package channelservicelogic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

func TestNormalizeChannelTypeMappingFieldsRejectsLegacyURLFields(t *testing.T) {
	_, err := normalizeChannelTypeMappingFields(`{"fields":[{"field":"dynamic.imageUrl","name":"镜像地址","kind":"address"}]}`)
	if err == nil {
		t.Fatalf("normalizeChannelTypeMappingFields should reject legacy url fields")
	}
	if !strings.Contains(err.Error(), "旧 dynamic.*Url 协议") {
		t.Fatalf("normalizeChannelTypeMappingFields error = %v", err)
	}
}

func TestNormalizeChannelTypeMappingFieldsForcesManualInput(t *testing.T) {
	normalized, err := normalizeChannelTypeMappingFields(`{"fields":[{"field":"dynamic.image","name":"镜像","kind":"dynamic","allowManualInput":false}]}`)
	if err != nil {
		t.Fatalf("normalizeChannelTypeMappingFields error: %v", err)
	}
	field, ok := channelvars.FindVariableField(normalized, channelvars.FieldDynamicImage)
	if !ok {
		t.Fatalf("normalized mapping fields should contain %s", channelvars.FieldDynamicImage)
	}
	if !field.AllowManualInput {
		t.Fatalf("normalized dynamic field should allow manual input")
	}
}

func TestNormalizeMetadataJSONUsesAPIVersionForProtocolChannel(t *testing.T) {
	normalized := normalizeMetadataJSON(`{"productName":"GitHub","apiVersion":"v3"}`)
	var data map[string]any
	if err := json.Unmarshal([]byte(normalized), &data); err != nil {
		t.Fatalf("normalizeMetadataJSON unmarshal failed: %v", err)
	}
	if got := data["version"]; got != "v3" {
		t.Fatalf("version = %v, want %q", got, "v3")
	}
}

func TestNormalizeMetadataJSONKeepsGitLabAPIVersionOutOfVersion(t *testing.T) {
	normalized := normalizeMetadataJSON(`{"productName":"GitLab","apiVersion":"v4"}`)
	var data map[string]any
	if err := json.Unmarshal([]byte(normalized), &data); err != nil {
		t.Fatalf("normalizeMetadataJSON unmarshal failed: %v", err)
	}
	if _, ok := data["version"]; ok {
		t.Fatalf("GitLab apiVersion should not be used as product version: %s", normalized)
	}
}
