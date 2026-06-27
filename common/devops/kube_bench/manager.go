package kube_bench

import (
	"context"
	"encoding/json"
	"strings"

	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
)

type Manager struct{}

func New() devopstypes.Provider {
	return Manager{}
}

func (Manager) TestConnection(ctx context.Context, req devopstypes.Request) devopstypes.Result {
	_ = ctx
	config := parseConfig(req.Channel.Config)
	metadata := devopstypes.Metadata{
		ProductName:      "kube-bench",
		EngineVersion:    first(config, "engineVersion", "version", "imageVersion"),
		BenchmarkProfile: first(config, "profile", "benchmarkProfile"),
		Capabilities:     map[string]any{"config": true},
	}
	if metadata.EngineVersion == "" {
		metadata.EngineVersion = "待配置版本"
	}
	if metadata.BenchmarkProfile == "" {
		metadata.BenchmarkProfile = "cis"
	}
	return devopstypes.Healthy("kube-bench 配置可用", metadata)
}

func parseConfig(raw string) map[string]any {
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return map[string]any{}
	}
	return data
}

func first(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
