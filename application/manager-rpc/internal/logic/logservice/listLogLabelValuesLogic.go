package logservicelogic

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	logtypes "github.com/yanshicheng/kube-nova/common/logmanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListLogLabelValuesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListLogLabelValuesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLogLabelValuesLogic {
	return &ListLogLabelValuesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListLogLabelValuesLogic) ListLogLabelValues(in *pb.ListLogLabelValuesReq) (*pb.ListLogLabelValuesResp, error) {
	values, err := listLogLabelValues(in)
	if err != nil {
		return nil, err
	}

	dynamicValues, err := l.listDynamicLabelValues(in)
	if err != nil {
		l.Errorf("动态查询 label values 失败，降级使用默认值: %v", err)
	}

	merged := mergeLabelValues(values, dynamicValues)
	return &pb.ListLogLabelValuesResp{Values: merged}, nil
}

func mergeLabelValues(staticValues, dynamicValues []string) []string {
	seen := make(map[string]struct{}, len(staticValues)+len(dynamicValues))
	merged := make([]string, 0, len(staticValues)+len(dynamicValues))
	appendValues := func(values []string) {
		for _, value := range values {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			merged = append(merged, trimmed)
		}
	}
	appendValues(staticValues)
	appendValues(dynamicValues)
	sort.Strings(merged)
	return merged
}

func (l *ListLogLabelValuesLogic) listDynamicLabelValues(in *pb.ListLogLabelValuesReq) ([]string, error) {
	clusterUUID := strings.TrimSpace(in.GetClusterUuid())
	labelKey := strings.TrimSpace(in.GetLabelKey())
	if clusterUUID == "" || labelKey == "" {
		return []string{}, nil
	}

	sourceType := strings.ToLower(strings.TrimSpace(in.GetSourceType()))
	if sourceType != logSourceTypeContainer && sourceType != logSourceTypeSystem {
		return []string{}, nil
	}

	requestedMode := resolveRequestedQueryMode(in.GetQueryMode())
	if requestedMode == logtypes.QueryModePlatform {
		return []string{}, nil
	}
	if l.svcCtx == nil || l.svcCtx.OnecClusterAppModel == nil {
		return []string{}, nil
	}

	queryLogic := NewQueryLogsLogic(l.ctx, l.svcCtx)
	app, err := queryLogic.lookupLogBackendApp(clusterUUID, requestedMode)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return []string{}, nil
	}

	client, err := buildLogClient(l.ctx, app, l.svcCtx.Config.LogSearch)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	searchInput := &pb.QueryLogsReq{
		ClusterUuid: clusterUUID,
		Namespace:   strings.TrimSpace(in.GetNamespace()),
		Hosts:       in.GetHosts(),
		QueryMode:   requestedMode,
		StartTime:   now.Add(-2 * time.Hour).UnixMilli(),
		EndTime:     now.UnixMilli(),
		Limit:       300,
		Direction:   "backward",
		LogType:     sourceType,
		Mode:        logtypes.SearchModeForm,
	}
	if len(in.GetHosts()) > 0 {
		searchInput.Host = strings.TrimSpace(in.GetHosts()[0])
	}
	if sourceType == logSourceTypeContainer {
		if resourceName := strings.TrimSpace(in.GetResourceName()); resourceName != "" {
			searchInput.PodName = resourceName + "*"
		}
	}

	searchReq, err := buildSearchRequest(searchInput, sourceType, client.GetBackendType(), searchDefaults{
		QueryMode:    l.svcCtx.Config.LogSearch.QueryMode,
		DataStream:   resolveElasticsearchDataStream(client.GetBackendType(), sourceType, l.svcCtx.Config.LogSearch.Elasticsearch.DataStream),
		IndexPattern: resolveElasticsearchIndexPattern(client.GetBackendType(), sourceType, l.svcCtx.Config.LogSearch.Elasticsearch.IndexPattern),
	})
	if err != nil {
		return nil, err
	}

	searchResp, err := client.Search(searchReq)
	if err != nil {
		return nil, err
	}
	if searchResp == nil || len(searchResp.List) == 0 {
		return []string{}, nil
	}

	candidateKeys := l.mappedLabelKeyCandidates(labelKey, client.GetBackendType())
	seen := make(map[string]struct{}, len(searchResp.List))
	values := make([]string, 0, len(searchResp.List))
	for _, record := range searchResp.List {
		if len(record.Labels) == 0 {
			continue
		}
		for _, key := range candidateKeys {
			value := strings.TrimSpace(record.Labels[key])
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			values = append(values, value)
		}
	}
	sort.Strings(values)
	return values, nil
}

func (l *ListLogLabelValuesLogic) mappedLabelKeyCandidates(labelKey, backendType string) []string {
	seen := map[string]struct{}{}
	candidates := make([]string, 0, 2)
	appendCandidate := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}

	labelKey = strings.TrimSpace(labelKey)
	appendCandidate(labelKey)

	if backendType != "elasticsearch" {
		return candidates
	}

	fieldMapping := l.svcCtx.Config.LogSearch.Elasticsearch.FieldMapping
	switch labelKey {
	case "project_uuid":
		appendCandidate(fieldMapping.ProjectUuid)
	case "cluster_uuid":
		appendCandidate(fieldMapping.ClusterUuid)
	case "namespace_name":
		appendCandidate(fieldMapping.NamespaceName)
	case "resource_name":
		appendCandidate(fieldMapping.ResourceName)
	case "pod_name":
		appendCandidate(fieldMapping.PodName)
	case "container_name":
		appendCandidate(fieldMapping.ContainerName)
	case "pod_ip":
		appendCandidate(fieldMapping.PodIp)
	case "node_name", "host":
		appendCandidate(fieldMapping.Host)
		appendCandidate("node_name")
		appendCandidate("host")
	case "source_type":
		appendCandidate(fieldMapping.SourceType)
	case "log_type":
		appendCandidate(fieldMapping.LogType)
	case "level":
		appendCandidate(fieldMapping.Level)
	}
	return candidates
}
