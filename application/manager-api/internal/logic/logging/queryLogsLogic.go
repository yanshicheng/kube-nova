package logging

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type QueryLogsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewQueryLogsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryLogsLogic {
	return &QueryLogsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *QueryLogsLogic) QueryLogs(req *types.LogQueryRequest) (resp *types.LogQueryResponse, err error) {
	start, err := parseLogTime(req.Start)
	if err != nil {
		return nil, err
	}
	end, err := parseLogTime(req.End)
	if err != nil {
		return nil, err
	}
	if start.After(end) {
		return nil, fmt.Errorf("开始时间不能大于结束时间")
	}

	logType := firstNonEmpty(req.LogType, req.LegacyLogType)
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "form"
	}
	queryText := strings.TrimSpace(req.QueryText)
	keyword := strings.TrimSpace(req.Keyword)
	if req.Message != "" {
		if queryText == "" {
			queryText = req.Message
		}
		if keyword == "" {
			keyword = req.Message
		}
	}
	expr := strings.TrimSpace(req.Expr)
	if mode != "code" && expr == "" && shouldPromoteFormQueryToCode(req.QueryMode, queryText) {
		mode = "code"
		expr = queryText
	}
	if mode == "code" {
		if expr == "" {
			return nil, fmt.Errorf("code 模式下 expr 不能为空")
		}
		if queryText == "" {
			queryText = expr
		}
		if keyword == "" {
			keyword = expr
		}
	}
	rpcResp, err := l.svcCtx.LogRpc.QueryLogs(l.ctx, &logservice.QueryLogsReq{
		ClusterUuid:   req.ClusterUuid,
		Namespace:     req.Namespace,
		Application:   req.Application,
		ResourceName:  req.ResourceName,
		PodName:       req.PodName,
		ContainerName: req.ContainerName,
		PodIp:         req.PodIp,
		Hosts:         req.Hosts,
		Level:         req.Level,
		QueryMode:     req.QueryMode,
		QueryText:     queryText,
		Command:       req.Command,
		Keyword:       keyword,
		StartTime:     start.UnixMilli(),
		EndTime:       end.UnixMilli(),
		Limit:         req.Limit,
		Direction:     req.Direction,
		NextToken:     req.NextToken,
		LogType:       logType,
		ProjectUuid:   req.ProjectUuid,
		Host:          req.Host,
		SourceType:    req.SourceType,
		Mode:          mode,
		Expr:          expr,
	})
	if err != nil {
		return nil, err
	}

	list := make([]types.LogRecord, 0, len(rpcResp.List))
	for _, item := range rpcResp.List {
		list = append(list, types.LogRecord{
			Timestamp:   item.Timestamp,
			Message:     item.Message,
			BackendType: item.BackendType,
			Labels:      item.Labels,
			Raw:         item.Raw,
		})
	}
	return &types.LogQueryResponse{
		BackendType:  rpcResp.BackendType,
		QueryMode:    rpcResp.QueryMode,
		DataStream:   rpcResp.DataStream,
		IndexPattern: rpcResp.IndexPattern,
		NextToken:    rpcResp.NextToken,
		List:         list,
	}, nil
}

func shouldPromoteFormQueryToCode(queryMode, queryText string) bool {
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(queryMode)) {
	case "loki":
		return strings.HasPrefix(queryText, "{") ||
			strings.Contains(queryText, "| json") ||
			strings.Contains(queryText, "|=") ||
			strings.Contains(queryText, "|~")
	case "es":
		return strings.HasPrefix(queryText, "log_type:") ||
			strings.HasPrefix(queryText, "cluster_uuid:") ||
			strings.Contains(queryText, " AND ") ||
			strings.Contains(queryText, " OR ") ||
			strings.Contains(queryText, " NOT ")
	default:
		return false
	}
}
