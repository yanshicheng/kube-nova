package config

import (
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

func TestConfigLoadLogSearchElasticsearch(t *testing.T) {
	yamlContent := []byte(`Name: manager.rpc
ListenOn: 0.0.0.0:30011
Timeout: 30000
Mysql:
  DataSource: root:password@tcp(127.0.0.1:3306)/kube_nova?charset=utf8mb4&parseTime=True&loc=Local
  MaxOpenConns: 10
  MaxIdleConns: 5
  ConnMaxLifetime: 30m
Cache:
  Host: 127.0.0.1:6379
  Type: node
DBCache:
  - Host: 127.0.0.1:6379
    Type: node
PortalRpc:
  Endpoints:
    - 127.0.0.1:30010
LogSearch:
  QueryMode: platform
  Elasticsearch:
    DataStream: logs-kube-nova-default
    IndexPattern: logs-kube-nova-*
    FieldMapping:
      Timestamp: "@timestamp"
      Message: message
      ProjectUuid: kubernetes.labels.project_uuid
      NamespaceName: kubernetes.namespace_name
      ResourceName: kubernetes.labels.resource_name
      PodName: kubernetes.pod_name
      ContainerName: kubernetes.container_name
      PodIp: kubernetes.pod_ip
      LogType: log_type
      Level: level
`)

	var cfg Config
	if err := conf.LoadFromYamlBytes(yamlContent, &cfg); err != nil {
		t.Fatalf("LoadFromYamlBytes() returned error: %v", err)
	}
	if cfg.LogSearch.QueryMode != "platform" {
		t.Fatalf("expected QueryMode to be loaded, got %q", cfg.LogSearch.QueryMode)
	}
	if cfg.LogSearch.Elasticsearch.DataStream != "logs-kube-nova-default" {
		t.Fatalf("expected DataStream to be loaded, got %q", cfg.LogSearch.Elasticsearch.DataStream)
	}
	if cfg.LogSearch.Elasticsearch.IndexPattern != "logs-kube-nova-*" {
		t.Fatalf("expected IndexPattern to be loaded, got %q", cfg.LogSearch.Elasticsearch.IndexPattern)
	}
	if cfg.LogSearch.Elasticsearch.FieldMapping.NamespaceName != "kubernetes.namespace_name" {
		t.Fatalf("expected NamespaceName field mapping, got %q", cfg.LogSearch.Elasticsearch.FieldMapping.NamespaceName)
	}
	if cfg.LogSearch.Elasticsearch.FieldMapping.Level != "level" {
		t.Fatalf("expected Level field mapping, got %q", cfg.LogSearch.Elasticsearch.FieldMapping.Level)
	}
}
