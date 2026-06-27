package types

import "testing"

func TestSearchRequestNormalizeNamespaceCompatibility(t *testing.T) {
	req := SearchRequest{
		Namespace: "legacy-ns",
		Keyword:   "error",
	}

	if err := req.Normalize(QueryDefaults{}); err != nil {
		t.Fatalf("Normalize() returned error: %v", err)
	}
	if req.NamespaceName != "legacy-ns" {
		t.Fatalf("expected NamespaceName to inherit legacy namespace, got %q", req.NamespaceName)
	}
	if req.Namespace != "legacy-ns" {
		t.Fatalf("expected Namespace to be preserved, got %q", req.Namespace)
	}
	if req.QueryText != "error" {
		t.Fatalf("expected QueryText to inherit legacy keyword, got %q", req.QueryText)
	}
}

func TestSearchRequestNormalizeQueryMode(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantError bool
	}{
		{name: "default platform", input: "", want: QueryModePlatform},
		{name: "platform", input: "platform", want: QueryModePlatform},
		{name: "loki", input: "loki", want: QueryModeLoki},
		{name: "es", input: "es", want: QueryModeES},
		{name: "invalid simple alias", input: "simple", wantError: true},
		{name: "invalid raw alias", input: "raw", wantError: true},
		{name: "invalid elasticsearch alias", input: "elasticsearch", wantError: true},
		{name: "invalid", input: "splunk", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := SearchRequest{QueryMode: tt.input}
			err := req.Normalize(QueryDefaults{})
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Normalize() returned error: %v", err)
			}
			if req.QueryMode != tt.want {
				t.Fatalf("expected QueryMode %q, got %q", tt.want, req.QueryMode)
			}
		})
	}
}

func TestSearchRequestNormalizeWildcardQueryText(t *testing.T) {
	req := SearchRequest{QueryText: "*"}
	if err := req.Normalize(QueryDefaults{}); err != nil {
		t.Fatalf("Normalize() returned error: %v", err)
	}
	if req.QueryText != "" || req.Keyword != "" {
		t.Fatalf("expected wildcard query text to normalize to empty, got queryText=%q keyword=%q", req.QueryText, req.Keyword)
	}
}

func TestSearchRequestNormalizeDefaultsAndTrim(t *testing.T) {
	req := SearchRequest{
		ClusterUuid:   " cluster-1 ",
		ProjectUuid:   " project-1 ",
		NamespaceName: " ns-1 ",
		Application:   " app-1 ",
		ResourceName:  " deploy-1 ",
		PodName:       " pod-1 ",
		ContainerName: " container-1 ",
		PodIp:         " 10.0.0.1 ",
		LogType:       " container ",
		Level:         " error ",
		Command:       " | json | pod_ip=\"10.0.0.2\" ",
		DataStream:    " logs-default ",
		IndexPattern:  " logs-* ",
		Direction:     " backward ",
		NextToken:     " token-1 ",
	}

	err := req.Normalize(QueryDefaults{QueryMode: QueryModeES})
	if err != nil {
		t.Fatalf("Normalize() returned error: %v", err)
	}
	if req.QueryMode != QueryModeES {
		t.Fatalf("expected QueryMode %q, got %q", QueryModeES, req.QueryMode)
	}
	if req.DataStream != "logs-default" {
		t.Fatalf("expected DataStream to stay trimmed, got %q", req.DataStream)
	}
	if req.IndexPattern != "logs-*" {
		t.Fatalf("expected IndexPattern to stay trimmed, got %q", req.IndexPattern)
	}
	if req.ClusterUuid != "cluster-1" || req.ProjectUuid != "project-1" || req.NamespaceName != "ns-1" {
		t.Fatalf("expected trimmed scope fields, got cluster=%q project=%q namespace=%q", req.ClusterUuid, req.ProjectUuid, req.NamespaceName)
	}
	if req.PodIp != "10.0.0.1" || req.LogType != "container" || req.Level != "error" {
		t.Fatalf("expected trimmed filters, got podIp=%q logType=%q level=%q", req.PodIp, req.LogType, req.Level)
	}
	if req.Command != "| json | pod_ip=\"10.0.0.2\"" {
		t.Fatalf("expected Command to stay trimmed, got %q", req.Command)
	}
}

func TestQueryDefaultsNormalizeTrim(t *testing.T) {
	defaults, err := QueryDefaults{QueryMode: " es "}.Normalize()
	if err != nil {
		t.Fatalf("Normalize() returned error: %v", err)
	}
	if defaults.QueryMode != QueryModeES {
		t.Fatalf("expected QueryMode %q, got %q", QueryModeES, defaults.QueryMode)
	}
}

func TestNormalizeElasticsearchConfigTrim(t *testing.T) {
	cfg := NormalizeElasticsearchConfig(ElasticsearchConfig{
		DataStream:   " logs-default ",
		IndexPattern: " logs-* ",
		FieldMapping: ElasticsearchFieldMapping{
			Timestamp:     " @timestamp ",
			Message:       " message ",
			ProjectUuid:   " project_uuid ",
			ClusterUuid:   " cluster_uuid ",
			NamespaceName: " namespace_name ",
			ResourceName:  " resource_name ",
			PodName:       " pod_name ",
			ContainerName: " container_name ",
			PodIp:         " pod_ip ",
			LogType:       " log_type ",
			Level:         " level ",
		},
	})
	if cfg.DataStream != "logs-default" || cfg.IndexPattern != "logs-*" {
		t.Fatalf("expected trimmed data stream config, got dataStream=%q indexPattern=%q", cfg.DataStream, cfg.IndexPattern)
	}
	if cfg.FieldMapping.Timestamp != "@timestamp" || cfg.FieldMapping.NamespaceName != "namespace_name" {
		t.Fatalf("expected trimmed field mappings, got timestamp=%q namespaceName=%q", cfg.FieldMapping.Timestamp, cfg.FieldMapping.NamespaceName)
	}
	if cfg.FieldMapping.Level != "level" {
		t.Fatalf("expected level mapping to be trimmed, got %q", cfg.FieldMapping.Level)
	}
}
