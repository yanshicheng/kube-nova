package channelcheck

import (
	"encoding/json"
	"testing"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
)

func TestProviderForIncludesGitHTTPChannels(t *testing.T) {
	for _, channelType := range []string{"github", "gitee"} {
		if providerFor(channelType) == nil {
			t.Fatalf("providerFor(%q) should return provider", channelType)
		}
	}
	if providerFor("svn") != nil {
		t.Fatal("svn should keep protocol fallback instead of Git HTTP provider")
	}
}

func TestChannelMetadataUsesToolEngineVersion(t *testing.T) {
	metadata := channelMetadata(&model.DevopsChannel{
		ChannelType: "spotbugs",
		Config:      `{"engineVersion":"4.8.6"}`,
	}, map[string]any{"config": true})

	raw := mustMetadataJSON(t, metadata)
	if got := raw["version"]; got != "4.8.6" {
		t.Fatalf("version = %v, want %q", got, "4.8.6")
	}
	if got := raw["productName"]; got != "SpotBugs" {
		t.Fatalf("productName = %v, want %q", got, "SpotBugs")
	}
}

func TestChannelMetadataUsesAPIVersionForProtocolChannel(t *testing.T) {
	metadata := channelMetadata(&model.DevopsChannel{ChannelType: "github"}, nil)

	raw := mustMetadataJSON(t, metadata)
	if got := raw["version"]; got != "v3" {
		t.Fatalf("version = %v, want %q", got, "v3")
	}
}

func mustMetadataJSON(t *testing.T, metadata devopstypes.Metadata) map[string]any {
	t.Helper()
	var raw map[string]any
	if err := json.Unmarshal([]byte(devopstypes.MetadataJSON(metadata)), &raw); err != nil {
		t.Fatalf("unmarshal metadata failed: %v", err)
	}
	return raw
}
