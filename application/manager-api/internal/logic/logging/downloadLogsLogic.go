package logging

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type DownloadLogsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	w      http.ResponseWriter
}

func NewDownloadLogsLogic(ctx context.Context, svcCtx *svc.ServiceContext, w http.ResponseWriter) *DownloadLogsLogic {
	return &DownloadLogsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx, w: w}
}

func (l *DownloadLogsLogic) DownloadLogs(req *types.LogDownloadRequest) error {
	start, err := parseRFC3339Time(req.Start)
	if err != nil {
		return err
	}
	end, err := parseRFC3339Time(req.End)
	if err != nil {
		return err
	}
	if req.Message != "" {
		if req.QueryText == "" {
			req.QueryText = req.Message
		}
		if req.Keyword == "" {
			req.Keyword = req.Message
		}
	}

	rpcResp, err := l.svcCtx.LogRpc.QueryLogs(l.ctx, &logservice.QueryLogsReq{
		ClusterUuid:   req.ClusterUuid,
		Namespace:     req.Namespace,
		PodName:       req.PodName,
		ContainerName: req.ContainerName,
		LogType:       firstNonEmpty(req.SourceType, "container"),
		PodIp:         req.PodIp,
		Hosts:         req.Hosts,
		Level:         req.Level,
		QueryMode:     req.QueryMode,
		QueryText:     req.QueryText,
		Keyword:       req.Keyword,
		StartTime:     start.UnixMilli(),
		EndTime:       end.UnixMilli(),
		Limit:         req.Limit,
		Direction:     req.Direction,
	})
	if err != nil {
		return err
	}

	namespaceName := req.Namespace
	if namespaceName == "" {
		namespaceName = "logs"
	}
	filename := fmt.Sprintf("logs-%s-%s.log", namespaceName, time.Now().Format("20060102-150405"))
	l.w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	l.w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	for _, item := range rpcResp.List {
		if _, err := l.w.Write([]byte(item.Message + "\n")); err != nil {
			return err
		}
	}
	return nil
}
