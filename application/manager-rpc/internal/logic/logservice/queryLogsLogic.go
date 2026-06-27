package logservicelogic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	logtypes "github.com/yanshicheng/kube-nova/common/logmanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type QueryLogsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

var (
	esContainerDataStream  = "logs-container-default"
	esSystemDataStream     = "logs-system-default"
	esContainerIndexStream = "logs-container-*"
	esSystemIndexStream    = "logs-system-*"
)

func NewQueryLogsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryLogsLogic {
	return &QueryLogsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func buildQueryLogsResp(searchResp *logtypes.SearchResponse) *pb.QueryLogsResp {
	resp := &pb.QueryLogsResp{
		BackendType:  searchResp.BackendType,
		NextToken:    searchResp.NextToken,
		QueryMode:    searchResp.QueryMode,
		DataStream:   searchResp.DataStream,
		IndexPattern: searchResp.IndexPattern,
		List:         make([]*pb.LogQueryRecord, 0, len(searchResp.List)),
	}
	for _, item := range searchResp.List {
		resp.List = append(resp.List, &pb.LogQueryRecord{
			Timestamp:   item.Timestamp,
			Message:     item.Message,
			BackendType: item.BackendType,
			Labels:      item.Labels,
			Raw:         item.Raw,
		})
	}
	return resp
}

func normalizeLogType(logType string) (string, error) {
	switch strings.TrimSpace(logType) {
	case "container", "system":
		return strings.TrimSpace(logType), nil
	case "":
		return "", errorx.Msg("logType 不能为空")
	default:
		return "", errorx.Msg("logType 仅支持 container 或 system")
	}
}

func normalizeSearchMode(mode string) (string, error) {
	normalized, err := logtypes.NormalizeSearchMode(mode)
	if err != nil {
		return "", errorx.Msg("mode 仅支持 form 或 code")
	}
	return normalized, nil
}

func resolveQueryMode(searchMode, requestedMode, backendType, defaultMode string) string {
	if searchMode == logtypes.SearchModeCode {
		switch backendType {
		case "loki":
			return logtypes.QueryModeLoki
		case "elasticsearch":
			return logtypes.QueryModeES
		default:
			return defaultMode
		}
	}
	if trimmed := strings.TrimSpace(requestedMode); trimmed != "" {
		return trimmed
	}
	return defaultMode
}

func mergeHosts(host string, hosts []string) (string, []string) {
	trimmedHost := strings.TrimSpace(host)
	merged := make([]string, 0, len(hosts)+1)
	seen := make(map[string]struct{}, len(hosts)+1)
	appendHost := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		merged = append(merged, value)
	}
	if trimmedHost != "" {
		appendHost(trimmedHost)
	}
	for _, item := range hosts {
		appendHost(item)
	}
	if trimmedHost == "" && len(merged) == 1 {
		trimmedHost = merged[0]
	}
	return trimmedHost, merged
}

func buildSearchRequest(in *pb.QueryLogsReq, logType, backendType string, defaults searchDefaults) (*logtypes.SearchRequest, error) {
	searchMode, err := normalizeSearchMode(in.GetMode())
	if err != nil {
		return nil, err
	}
	if searchMode == logtypes.SearchModeCode && strings.TrimSpace(in.GetExpr()) == "" {
		return nil, errorx.Msg("code 模式下 expr 不能为空")
	}
	host, hosts := mergeHosts(in.GetHost(), in.GetHosts())
	searchReq := &logtypes.SearchRequest{
		ClusterUuid:   in.GetClusterUuid(),
		ProjectUuid:   in.GetProjectUuid(),
		NamespaceName: in.GetNamespace(),
		Namespace:     in.GetNamespace(),
		Application:   in.GetApplication(),
		ResourceName:  in.GetResourceName(),
		PodName:       in.GetPodName(),
		ContainerName: in.GetContainerName(),
		PodIp:         in.GetPodIp(),
		Hosts:         hosts,
		Host:          host,
		SourceType:    in.GetSourceType(),
		LogType:       logType,
		Level:         in.GetLevel(),
		QueryText:     in.GetQueryText(),
		Command:       in.GetCommand(),
		Keyword:       in.GetKeyword(),
		QueryMode:     resolveQueryMode(searchMode, in.GetQueryMode(), backendType, defaults.QueryMode),
		QueryExpr:     in.GetExpr(),
		SearchMode:    searchMode,
		DataStream:    defaults.DataStream,
		IndexPattern:  defaults.IndexPattern,
		Start:         time.UnixMilli(in.GetStartTime()),
		End:           time.UnixMilli(in.GetEndTime()),
		Limit:         in.GetLimit(),
		Direction:     in.GetDirection(),
		NextToken:     in.GetNextToken(),
	}
	if err := searchReq.Normalize(logtypes.QueryDefaults{QueryMode: defaults.QueryMode}); err != nil {
		return nil, err
	}
	return searchReq, nil
}

type searchDefaults struct {
	QueryMode    string
	DataStream   string
	IndexPattern string
}

func appBackendType(app *model.OnecClusterApp) string {
	if app == nil {
		return ""
	}
	code := strings.ToLower(strings.TrimSpace(app.AppCode))
	switch {
	case code == "loki":
		return logtypes.QueryModeLoki
	case code == "elasticsearch", code == "es", strings.Contains(code, "elastic"):
		return logtypes.QueryModeES
	default:
		return ""
	}
}

func resolveRequestedQueryMode(mode string) string {
	normalized, err := logtypes.NormalizeQueryMode(mode)
	if err != nil {
		return logtypes.QueryModePlatform
	}
	return normalized
}

func resolveLogBackendApp(apps []*model.OnecClusterApp, requestedMode string) *model.OnecClusterApp {
	if len(apps) == 0 {
		return nil
	}
	requestedMode = resolveRequestedQueryMode(requestedMode)
	if requestedMode == logtypes.QueryModeLoki || requestedMode == logtypes.QueryModeES {
		for _, app := range apps {
			if appBackendType(app) == requestedMode {
				return app
			}
		}
		return nil
	}
	for _, app := range apps {
		if app == nil {
			continue
		}
		if app.IsDefault == 1 && appBackendType(app) != "" {
			return app
		}
	}
	for _, app := range apps {
		if appBackendType(app) != "" {
			return app
		}
	}
	return nil
}

func (l *QueryLogsLogic) lookupLogBackendApp(clusterUUID, requestedMode string) (*model.OnecClusterApp, error) {
	apps, err := l.svcCtx.OnecClusterAppModel.SearchNoPage(l.ctx, "created_at", false, "`cluster_uuid` = ? AND `app_type` = ?", clusterUUID, 2)
	if err != nil {
		return nil, err
	}
	return resolveLogBackendApp(apps, requestedMode), nil
}

func resolveElasticsearchDataStream(backendType, logType, fallback string) string {
	if backendType != "elasticsearch" {
		return strings.TrimSpace(fallback)
	}
	switch strings.TrimSpace(logType) {
	case "container":
		return esContainerDataStream
	case "system":
		return esSystemDataStream
	default:
		return strings.TrimSpace(fallback)
	}
}

func resolveElasticsearchIndexPattern(backendType, logType, fallback string) string {
	if backendType != "elasticsearch" {
		return strings.TrimSpace(fallback)
	}
	switch strings.TrimSpace(logType) {
	case "container":
		return esContainerIndexStream
	case "system":
		return esSystemIndexStream
	default:
		return strings.TrimSpace(fallback)
	}
}

func (l *QueryLogsLogic) QueryLogs(in *pb.QueryLogsReq) (*pb.QueryLogsResp, error) {
	if strings.TrimSpace(in.GetClusterUuid()) == "" {
		return nil, errorx.Msg("clusterUuid 不能为空")
	}
	logType, err := normalizeLogType(in.GetLogType())
	if err != nil {
		return nil, err
	}
	if in.GetStartTime() <= 0 || in.GetEndTime() <= 0 {
		return nil, errorx.Msg("start 和 end 不能为空")
	}
	if in.GetStartTime() > in.GetEndTime() {
		return nil, errorx.Msg("start 不能大于 end")
	}

	app, err := l.lookupLogBackendApp(in.GetClusterUuid(), in.GetQueryMode())
	if err != nil {
		return nil, errorx.Msg("查询日志组件配置失败")
	}
	if app == nil {
		return nil, errorx.Msg("未找到日志组件配置")
	}

	client, err := buildLogClient(l.ctx, app, l.svcCtx.Config.LogSearch)
	if err != nil {
		return nil, err
	}
	searchReq, err := buildSearchRequest(in, logType, client.GetBackendType(), searchDefaults{
		QueryMode:    l.svcCtx.Config.LogSearch.QueryMode,
		DataStream:   resolveElasticsearchDataStream(client.GetBackendType(), logType, l.svcCtx.Config.LogSearch.Elasticsearch.DataStream),
		IndexPattern: resolveElasticsearchIndexPattern(client.GetBackendType(), logType, l.svcCtx.Config.LogSearch.Elasticsearch.IndexPattern),
	})
	if err != nil {
		return nil, errorx.Msg(fmt.Sprintf("查询参数错误: %v", err))
	}

	searchResp, err := client.Search(searchReq)
	if err != nil {
		return nil, err
	}
	return buildQueryLogsResp(searchResp), nil
}
