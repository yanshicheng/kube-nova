package logservicelogic

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/config"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/portalservice"
	elasticsearch "github.com/yanshicheng/kube-nova/common/logmanager/operator/elasticsearch"
	"github.com/yanshicheng/kube-nova/common/logmanager/operator/loki"
	logtypes "github.com/yanshicheng/kube-nova/common/logmanager/types"
)

func buildLogClient(ctx context.Context, app *model.OnecClusterApp, logSearchCfg config.LogSearchConfig) (logtypes.LogClient, error) {
	endpoint := fmt.Sprintf("http://%s:%d", app.AppUrl, app.Port)
	if app.Protocol == "https" {
		endpoint = fmt.Sprintf("https://%s:%d", app.AppUrl, app.Port)
	}
	cfg := &logtypes.LogConfig{
		UUID:     app.ClusterUuid,
		Name:     app.AppName,
		AppCode:  app.AppCode,
		Endpoint: endpoint,
		Username: app.Username,
		Password: app.Password,
		Insecure: app.InsecureSkipVerify == 1,
		Timeout:  10,
		HTTPPool: logtypes.HTTPPoolConfig{
			MaxIdleConns:          logSearchCfg.HTTPPool.MaxIdleConns,
			MaxIdleConnsPerHost:   logSearchCfg.HTTPPool.MaxIdleConnsPerHost,
			MaxConnsPerHost:       logSearchCfg.HTTPPool.MaxConnsPerHost,
			IdleConnTimeout:       logSearchCfg.HTTPPool.IdleConnTimeout,
			ResponseHeaderTimeout: logSearchCfg.HTTPPool.ResponseHeaderTimeout,
			TLSHandshakeTimeout:   logSearchCfg.HTTPPool.TLSHandshakeTimeout,
			ExpectContinueTimeout: logSearchCfg.HTTPPool.ExpectContinueTimeout,
		},
		Elasticsearch: logtypes.ElasticsearchConfig{
			DataStream:   logSearchCfg.Elasticsearch.DataStream,
			IndexPattern: logSearchCfg.Elasticsearch.IndexPattern,
			FieldMapping: logtypes.ElasticsearchFieldMapping{
				Timestamp:     logSearchCfg.Elasticsearch.FieldMapping.Timestamp,
				Message:       logSearchCfg.Elasticsearch.FieldMapping.Message,
				ProjectUuid:   logSearchCfg.Elasticsearch.FieldMapping.ProjectUuid,
				ClusterUuid:   logSearchCfg.Elasticsearch.FieldMapping.ClusterUuid,
				NamespaceName: logSearchCfg.Elasticsearch.FieldMapping.NamespaceName,
				ResourceName:  logSearchCfg.Elasticsearch.FieldMapping.ResourceName,
				PodName:       logSearchCfg.Elasticsearch.FieldMapping.PodName,
				ContainerName: logSearchCfg.Elasticsearch.FieldMapping.ContainerName,
				PodIp:         logSearchCfg.Elasticsearch.FieldMapping.PodIp,
				Host:          logSearchCfg.Elasticsearch.FieldMapping.Host,
				SourceType:    logSearchCfg.Elasticsearch.FieldMapping.SourceType,
				LogType:       logSearchCfg.Elasticsearch.FieldMapping.LogType,
				Level:         logSearchCfg.Elasticsearch.FieldMapping.Level,
			},
		},
	}
	if app.Token.Valid {
		cfg.Token = app.Token.String
	}
	code := strings.ToLower(strings.TrimSpace(app.AppCode))
	switch {
	case code == "loki":
		return loki.NewClient(ctx, cfg)
	case code == "es", code == "elasticsearch", strings.Contains(code, "elastic"):
		return elasticsearch.NewClient(ctx, cfg)
	default:
		return nil, fmt.Errorf("不支持的日志后端: %s", app.AppCode)
	}
}

func BuildLogClientForEngine(ctx context.Context, app *model.OnecClusterApp, logSearchCfg config.LogSearchConfig) (logtypes.LogClient, error) {
	return buildLogClient(ctx, app, logSearchCfg)
}

func buildAlertSyncRequest(rule *model.OnecLogAlertRule, webhookURL, webhookToken string) *logtypes.AlertSyncRequest {
	return &logtypes.AlertSyncRequest{
		RuleID:        rule.Id,
		ClusterUuid:   rule.ClusterUuid,
		ProjectUuid:   rule.ProjectUuid,
		Namespace:     rule.Namespace,
		Application:   rule.Application,
		ResourceName:  rule.ResourceName,
		Name:          rule.Name,
		Description:   rule.Description,
		QueryText:     rule.QueryText,
		ConditionType: rule.ConditionType,
		Threshold:     rule.Threshold,
		Window:        rule.Window,
		Severity:      rule.Severity,
		WebhookURL:    webhookURL,
		WebhookToken:  webhookToken,
	}
}

func getWebhookCallback(ctx context.Context, svcCtx *svc.ServiceContext) (string, string, error) {
	resp, err := svcCtx.PortalRpc.GetKubeNovaPlatformUrl(ctx, &portalservice.GetPlatformUrlReq{})
	if err != nil {
		return "", "", err
	}
	if resp.Url == "" {
		return "", "", fmt.Errorf("平台地址为空")
	}
	return buildLogAlertWebhookURL(resp.Url), svcCtx.Config.Webhook.Token, nil
}
