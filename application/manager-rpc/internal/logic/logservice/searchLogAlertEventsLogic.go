package logservicelogic

import (
	"context"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	logtypes "github.com/yanshicheng/kube-nova/common/logmanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchLogAlertEventsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchLogAlertEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchLogAlertEventsLogic {
	return &SearchLogAlertEventsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SearchLogAlertEventsLogic) SearchLogAlertEvents(in *pb.SearchLogAlertEventsReq) (*pb.SearchLogAlertEventsResp, error) {
	queryStr := ""
	args := make([]any, 0, 4)
	appendCond := func(cond string, arg ...any) {
		if queryStr != "" {
			queryStr += " AND "
		}
		queryStr += cond
		args = append(args, arg...)
	}

	if clusterUUID := strings.TrimSpace(in.ClusterUuid); clusterUUID != "" {
		appendCond("`cluster_uuid` = ?", clusterUUID)
	}
	if in.RuleId > 0 {
		appendCond("`rule_id` = ?", in.RuleId)
	}
	if eventStatus := strings.TrimSpace(in.EventStatus); eventStatus != "" {
		appendCond("`event_status` = ?", eventStatus)
	}
	if notifyStatus := strings.TrimSpace(in.NotifyStatus); notifyStatus != "" {
		appendCond("`notify_status` = ?", notifyStatus)
	}

	page := in.Page
	if page == 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize == 0 {
		pageSize = 20
	}

	list, total, err := l.svcCtx.OnecLogAlertFireEventModel.Search(l.ctx, "id", false, page, pageSize, queryStr, args...)
	if err != nil {
		return nil, errorx.Msg("查询日志告警事件失败")
	}

	resp := &pb.SearchLogAlertEventsResp{
		Data:  make([]*pb.LogAlertEvent, 0, len(list)),
		Total: total,
	}
	for _, item := range list {
		eventResp, buildErr := l.buildEventResponse(item)
		if buildErr != nil {
			logx.Errorf("[SearchLogAlertEvents] 构建事件响应失败: eventId=%d, err=%v", item.Id, buildErr)
			continue
		}
		resp.Data = append(resp.Data, eventResp)
	}
	return resp, nil
}

func (l *SearchLogAlertEventsLogic) buildEventResponse(item *model.OnecLogAlertFireEvent) (*pb.LogAlertEvent, error) {
	payload, err := parseLogAlertEventPayload(item.PayloadJson)
	if err != nil {
		return nil, err
	}

	rule, _ := l.svcCtx.OnecLogAlertRuleModel.FindOne(l.ctx, item.RuleId)

	clusterName := payload.ClusterName
	projectUuid := payload.ProjectUuid
	projectName := payload.ProjectName
	workspaceId := payload.WorkspaceId
	workspaceName := payload.WorkspaceName
	namespace := payload.Namespace
	application := payload.Application
	resourceName := payload.ResourceName
	backendType := payload.BackendType
	ruleName := payload.RuleName

	if rule != nil {
		if clusterName == "" {
			if cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, rule.ClusterUuid); err == nil {
				clusterName = cluster.Name
			}
		}
		if projectUuid == "" {
			projectUuid = rule.ProjectUuid
		}
		if projectName == "" && rule.ProjectUuid != "" {
			if project, err := l.svcCtx.OnecProjectModel.FindOneByUuid(l.ctx, rule.ProjectUuid); err == nil {
				projectName = project.Name
			}
		}
		if workspaceId == 0 {
			workspaceId = rule.WorkspaceId
		}
		if workspaceName == "" && rule.WorkspaceId > 0 {
			if workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, rule.WorkspaceId); err == nil {
				workspaceName = workspace.Name
			}
		}
		if namespace == "" {
			namespace = rule.Namespace
		}
		if application == "" {
			application = rule.Application
		}
		if resourceName == "" {
			resourceName = rule.ResourceName
		}
		if backendType == "" {
			backendType = rule.BackendType
		}
		if ruleName == "" {
			ruleName = rule.Name
		}
	}

	sample := payload.Sample
	if sample == nil && item.HitCount > 0 && rule != nil {
		sample = l.fetchEventSample(rule, item)
	}

	var sampleTimestamp int64
	var sampleMessage, sampleRaw, podName, containerName, host string
	sampleLabels := map[string]string{}
	if sample != nil {
		sampleTimestamp = sample.Timestamp
		sampleMessage = sample.Message
		sampleRaw = sample.Raw
		if sample.Labels != nil {
			sampleLabels = sample.Labels
			podName = firstEventLabel(sample.Labels, "pod_name", "kubernetes.pod_name")
			containerName = firstEventLabel(sample.Labels, "container_name", "kubernetes.container_name")
			host = firstEventLabel(sample.Labels, "node_name", "host", "kubernetes.host")
		}
	}

	nextRetryAt := int64(0)
	if item.NextRetryAt.Valid {
		nextRetryAt = item.NextRetryAt.Time.Unix()
	}

	return &pb.LogAlertEvent{
		Id:               item.Id,
		RuleId:           item.RuleId,
		RuleName:         ruleName,
		BackendType:      backendType,
		ClusterUuid:      item.ClusterUuid,
		ClusterName:      clusterName,
		ProjectUuid:      projectUuid,
		ProjectName:      projectName,
		WorkspaceId:      workspaceId,
		WorkspaceName:    workspaceName,
		Namespace:        namespace,
		Application:      application,
		ResourceName:     resourceName,
		PodName:          podName,
		ContainerName:    containerName,
		Host:             host,
		EventFingerprint: item.EventFingerprint,
		HitCount:         item.HitCount,
		Severity:         item.Severity,
		EventStatus:      item.EventStatus,
		NotifyStatus:     item.NotifyStatus,
		NotifyError:      item.NotifyError,
		RetryCount:       item.RetryCount,
		NextRetryAt:      nextRetryAt,
		SampleTimestamp:  sampleTimestamp,
		SampleMessage:    sampleMessage,
		SampleRaw:        sampleRaw,
		SampleLabels:     sampleLabels,
		CloseReason:      payload.CloseReason,
		ClosedBy:         payload.ClosedBy,
		ClosedAt:         payload.ClosedAt,
		ManualReason:     payload.ManualReason,
		ManualReasonBy:   payload.ManualReasonBy,
		ManualReasonAt:   payload.ManualReasonAt,
		CreatedAt:        item.CreatedAt.Unix(),
		UpdatedAt:        item.UpdatedAt.Unix(),
	}, nil
}

func (l *SearchLogAlertEventsLogic) fetchEventSample(rule *model.OnecLogAlertRule, event *model.OnecLogAlertFireEvent) *logAlertSample {
	app, err := findEventBackendApp(l.ctx, l.svcCtx, rule.ClusterUuid, rule.BackendType)
	if err != nil || app == nil {
		return nil
	}
	client, err := BuildLogClientForEngine(l.ctx, app, l.svcCtx.Config.LogSearch)
	if err != nil {
		return nil
	}
	req := &logtypes.SearchRequest{
		ClusterUuid:   rule.ClusterUuid,
		ProjectUuid:   rule.ProjectUuid,
		NamespaceName: rule.Namespace,
		Namespace:     rule.Namespace,
		Application:   rule.Application,
		ResourceName:  rule.ResourceName,
		QueryText:     eventRuleQuery(rule),
		QueryMode:     eventQueryMode(rule.BackendType),
		SearchMode:    eventSearchMode(rule),
		QueryExpr:     eventExpr(rule),
		LogType:       eventLogType(rule),
		Start:         time.UnixMilli(event.WindowStartMs),
		End:           time.UnixMilli(event.WindowEndMs),
		Limit:         1,
		Direction:     "backward",
	}
	if req.QueryMode == logtypes.QueryModeES {
		req.DataStream = eventESDataStream(eventLogType(rule), l.svcCtx.Config.LogSearch.Elasticsearch.DataStream)
		req.IndexPattern = eventESIndexPattern(eventLogType(rule), l.svcCtx.Config.LogSearch.Elasticsearch.IndexPattern)
	}
	if err := req.Normalize(logtypes.QueryDefaults{QueryMode: req.QueryMode}); err != nil {
		return nil
	}
	resp, err := client.Search(req)
	if err != nil || resp == nil || len(resp.List) == 0 {
		return nil
	}
	return &logAlertSample{
		Timestamp: resp.List[0].Timestamp,
		Message:   resp.List[0].Message,
		Raw:       resp.List[0].Raw,
		Labels:    resp.List[0].Labels,
	}
}

func findEventBackendApp(ctx context.Context, svcCtx *svc.ServiceContext, clusterUUID, backendType string) (*model.OnecClusterApp, error) {
	apps, err := svcCtx.OnecClusterAppModel.SearchNoPage(ctx, "created_at", false, "`cluster_uuid` = ? AND `app_type` = ?", clusterUUID, 2)
	if err != nil || len(apps) == 0 {
		return nil, err
	}
	backendType = strings.ToLower(strings.TrimSpace(backendType))
	for _, item := range apps {
		if strings.EqualFold(strings.TrimSpace(item.AppCode), backendType) {
			return item, nil
		}
	}
	for _, item := range apps {
		if item.IsDefault == 1 {
			return item, nil
		}
	}
	return apps[0], nil
}

func firstEventLabel(labels map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(labels[key]); value != "" {
			return value
		}
	}
	return ""
}

func eventRuleQuery(rule *model.OnecLogAlertRule) string {
	if rule == nil || strings.EqualFold(strings.TrimSpace(rule.ConditionType), "flatline") {
		return ""
	}
	return rule.QueryText
}

func eventQueryMode(backendType string) string {
	if strings.EqualFold(strings.TrimSpace(backendType), "loki") {
		return logtypes.QueryModeLoki
	}
	return logtypes.QueryModeES
}

func eventSearchMode(rule *model.OnecLogAlertRule) string {
	if rule == nil || strings.TrimSpace(rule.SearchMode) == "" {
		return logtypes.SearchModeForm
	}
	return strings.TrimSpace(rule.SearchMode)
}

func eventLogType(rule *model.OnecLogAlertRule) string {
	if rule == nil || strings.TrimSpace(rule.LogType) == "" {
		return "container"
	}
	return strings.TrimSpace(rule.LogType)
}

func eventESDataStream(logType, fallback string) string {
	switch strings.TrimSpace(logType) {
	case "container":
		return "logs-container-default"
	case "system":
		return "logs-system-default"
	default:
		return strings.TrimSpace(fallback)
	}
}

func eventESIndexPattern(logType, fallback string) string {
	switch strings.TrimSpace(logType) {
	case "container":
		return "logs-container-*"
	case "system":
		return "logs-system-*"
	default:
		return strings.TrimSpace(fallback)
	}
}

func eventExpr(rule *model.OnecLogAlertRule) string {
	if rule == nil || !rule.Expr.Valid {
		return ""
	}
	return rule.Expr.String
}
