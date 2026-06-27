package logservicelogic

import (
	"sort"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

const (
	logSourceTypeContainer = "container"
	logSourceTypeSystem    = "system"
)

var (
	containerLokiLabelKeys = []string{"app", "caller", "cluster_name", "cluster_uuid", "container_name", "duration", "level", "log_type", "namespace_name", "node_name", "pod_ip", "pod_name", "project_uuid", "remote_host", "source_host", "span", "trace"}
	containerESLabelKeys   = []string{"app", "caller", "cluster_name", "cluster_uuid", "container_name", "duration", "level", "log_type", "namespace_name", "node_name", "pod_ip", "pod_name", "project_uuid", "remote_host", "source_host", "span", "trace"}
	systemLokiLabelKeys    = []string{"app", "cluster_name", "cluster_uuid", "level", "log_type", "node_name", "source_host", "source_type"}
	systemESLabelKeys      = []string{"app", "cluster_name", "cluster_uuid", "level", "log_type", "node_name", "source_host", "source_type"}
)

func listLogLabelKeys(sourceType, queryMode string) ([]string, error) {
	sourceType = strings.ToLower(strings.TrimSpace(sourceType))
	queryMode = strings.ToLower(strings.TrimSpace(queryMode))

	switch queryMode {
	case "loki":
		switch sourceType {
		case logSourceTypeContainer:
			return append([]string(nil), containerLokiLabelKeys...), nil
		case logSourceTypeSystem:
			return append([]string(nil), systemLokiLabelKeys...), nil
		}
	case "es":
		switch sourceType {
		case logSourceTypeContainer:
			return append([]string(nil), containerESLabelKeys...), nil
		case logSourceTypeSystem:
			return append([]string(nil), systemESLabelKeys...), nil
		}
	case "platform":
		return nil, errorx.Msg("当前模式不支持 labels 元数据")
	}

	return nil, errorx.Msg("当前模式不支持 labels 元数据")
}

func listLogLabelValues(in *pb.ListLogLabelValuesReq) ([]string, error) {
	keys, err := listLogLabelKeys(in.SourceType, in.QueryMode)
	if err != nil {
		return nil, err
	}
	labelKey := strings.TrimSpace(in.LabelKey)
	if labelKey == "" {
		return []string{}, nil
	}
	allowed := false
	for _, key := range keys {
		if key == labelKey {
			allowed = true
			break
		}
	}
	if !allowed {
		return []string{}, nil
	}

	values := []string{}
	switch strings.ToLower(strings.TrimSpace(in.SourceType)) {
	case logSourceTypeContainer:
		switch labelKey {
		case "app":
			values = append(values, strings.TrimSpace(in.Application))
		case "cluster_uuid":
			values = append(values, strings.TrimSpace(in.ClusterUuid))
		case "namespace_name":
			values = append(values, strings.TrimSpace(in.Namespace))
		case "node_name", "host":
			values = append(values, in.Hosts...)
		case "log_type":
			values = append(values, logSourceTypeContainer)
		case "level":
			values = append(values, "info", "warn", "error")
		}
	case logSourceTypeSystem:
		switch labelKey {
		case "cluster_uuid":
			values = append(values, strings.TrimSpace(in.ClusterUuid))
		case "node_name", "host":
			values = append(values, in.Hosts...)
		case "log_type":
			values = append(values, logSourceTypeSystem)
		case "level":
			values = append(values, "info", "warning", "error")
		}
	}

	filtered := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		filtered = append(filtered, value)
	}
	sort.Strings(filtered)
	return filtered, nil
}
