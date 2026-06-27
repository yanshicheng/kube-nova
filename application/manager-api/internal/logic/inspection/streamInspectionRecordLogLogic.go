// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package inspection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	inspectionLogStreamTick        = 2 * time.Second
	inspectionLogStreamHeartbeat   = 15 * time.Second
	inspectionLogStreamMaxDuration = 2 * time.Hour
	inspectionLogStreamMaxErrors   = 30
)

type StreamInspectionRecordLogLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	w      http.ResponseWriter
}

// 巡检执行日志SSE
func NewStreamInspectionRecordLogLogic(ctx context.Context, svcCtx *svc.ServiceContext, w http.ResponseWriter) *StreamInspectionRecordLogLogic {
	return &StreamInspectionRecordLogLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
		w:      w,
	}
}

func (l *StreamInspectionRecordLogLogic) StreamInspectionRecordLog(req *types.InspectionReportRequest) error {
	if req.RecordId == 0 {
		return errorx.Msg("巡检记录ID不能为空")
	}
	report, err := l.svcCtx.ManagerRpc.InspectionReportGet(l.ctx, &pb.InspectionReportReq{RecordId: req.RecordId})
	if err != nil {
		return err
	}
	flusher, ok := l.w.(http.Flusher)
	if !ok {
		return errorx.Msg("当前连接不支持SSE")
	}

	l.w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	l.w.Header().Set("Cache-Control", "no-cache")
	l.w.Header().Set("Connection", "keep-alive")
	l.w.Header().Set("X-Accel-Buffering", "no")
	if _, err := fmt.Fprint(l.w, "retry: 3000\n\n"); err != nil {
		return nil
	}
	flusher.Flush()

	sent := 0
	ticker := time.NewTicker(inspectionLogStreamTick)
	defer ticker.Stop()
	heartbeat := time.NewTicker(inspectionLogStreamHeartbeat)
	defer heartbeat.Stop()
	maxTimer := time.NewTimer(inspectionLogStreamMaxDuration)
	defer maxTimer.Stop()
	reportErrorCount := 0

	for {
		if err := l.flushLogs(req.RecordId, &sent, flusher); err != nil {
			if l.writeEvent("log", map[string]any{
				"time":    time.Now().Unix(),
				"level":   "warning",
				"message": "读取巡检执行日志失败: " + err.Error(),
			}, flusher) != nil {
				return nil
			}
		}
		if report != nil && report.Record != nil && report.Record.Status != "running" {
			if l.writeEvent("done", map[string]any{
				"recordId":     req.RecordId,
				"status":       report.Record.Status,
				"healthLevel":  report.Record.HealthLevel,
				"finishedAt":   report.Record.FinishedAt,
				"message":      "任务执行完成，请查看报告",
				"errorMessage": report.Record.ErrorMessage,
			}, flusher) != nil {
				return nil
			}
			return nil
		}

		select {
		case <-l.ctx.Done():
			return nil
		case <-heartbeat.C:
			if l.writeEvent("ping", map[string]any{"time": time.Now().Unix()}, flusher) != nil {
				return nil
			}
		case <-maxTimer.C:
			_ = l.writeEvent("done", map[string]any{
				"recordId": req.RecordId,
				"status":   "running",
				"message":  "日志连接已超过最大时长，请刷新报告中心查看最新状态",
			}, flusher)
			return nil
		case <-ticker.C:
			latest, err := l.svcCtx.ManagerRpc.InspectionReportGet(l.ctx, &pb.InspectionReportReq{RecordId: req.RecordId})
			if err != nil {
				reportErrorCount++
				if reportErrorCount == 1 || reportErrorCount%5 == 0 {
					if l.writeEvent("log", map[string]any{
						"time":    time.Now().Unix(),
						"level":   "warning",
						"message": "同步巡检状态失败，系统将继续重试: " + err.Error(),
					}, flusher) != nil {
						return nil
					}
				}
				if reportErrorCount >= inspectionLogStreamMaxErrors {
					_ = l.writeEvent("done", map[string]any{
						"recordId": req.RecordId,
						"status":   "unknown",
						"message":  "日志连接连续同步失败，请刷新报告中心查看最新状态",
					}, flusher)
					return nil
				}
				continue
			}
			reportErrorCount = 0
			report = latest
		}
	}
}

func (l *StreamInspectionRecordLogLogic) flushLogs(recordID uint64, sent *int, flusher http.Flusher) error {
	if l.svcCtx.Cache == nil {
		return nil
	}
	key := fmt.Sprintf("inspection:record:logs:%d", recordID)
	items, err := l.svcCtx.Cache.LrangeCtx(l.ctx, key, *sent, -1)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	for _, item := range items {
		if err := l.writeRawEvent("log", item, flusher); err != nil {
			return err
		}
	}
	*sent += len(items)
	return nil
}

func (l *StreamInspectionRecordLogLogic) writeEvent(event string, payload any, flusher http.Flusher) error {
	data, err := json.Marshal(payload)
	if err != nil {
		l.Errorf("序列化SSE事件失败: %v", err)
		return err
	}
	return l.writeRawEvent(event, string(data), flusher)
}

func (l *StreamInspectionRecordLogLogic) writeRawEvent(event, data string, flusher http.Flusher) error {
	if _, err := fmt.Fprintf(l.w, "event: %s\n", event); err != nil {
		if !errorsIsClosed(err) {
			l.Errorf("写入SSE事件失败: %v", err)
		}
		return err
	}
	if _, err := fmt.Fprintf(l.w, "data: %s\n\n", data); err != nil {
		if !errorsIsClosed(err) {
			l.Errorf("写入SSE数据失败: %v", err)
		}
		return err
	}
	flusher.Flush()
	return nil
}

func errorsIsClosed(err error) bool {
	return err == nil || errors.Is(err, context.Canceled) || errors.Is(err, io.ErrClosedPipe)
}
