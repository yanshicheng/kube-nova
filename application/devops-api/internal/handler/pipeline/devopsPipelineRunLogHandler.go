// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pipeline

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/mcuadros/go-defaults"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/logic/pipeline"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/verify"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 查询流水线日志
func DevopsPipelineRunLogHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DevopsPipelineRunLogRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		// 设置默认值
		defaults.SetDefaults(&req)
		// validator验证
		if err := svcCtx.Validator.Validate.StructCtx(r.Context(), &req); err != nil {
			strErr := verify.RemoveTopSaStr(err.(validator.ValidationErrors), svcCtx.Validator.Translator)
			httpx.ErrorCtx(r.Context(), w, errorx.New(40020, strErr))
			return
		}
		if strings.EqualFold(r.URL.Query().Get("stream"), "true") {
			streamDevopsPipelineRunLog(svcCtx, w, r, req)
			return
		}
		l := pipeline.NewDevopsPipelineRunLogLogic(r.Context(), svcCtx)
		resp, err := l.DevopsPipelineRunLog(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

func streamDevopsPipelineRunLog(svcCtx *svc.ServiceContext, w http.ResponseWriter, r *http.Request, req types.DevopsPipelineRunLogRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.ErrorCtx(r.Context(), w, errorx.New(50000, "当前服务不支持日志流"))
		return
	}

	header := w.Header()
	header.Set("Content-Type", "text/event-stream; charset=utf-8")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")

	send := func(event string, data any) bool {
		payload, err := json.Marshal(data)
		if err != nil {
			payload, _ = json.Marshal(map[string]string{"message": err.Error()})
			event = "error"
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	if !send("open", map[string]string{"status": "connected"}) {
		return
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	// go-zero 接口有全局超时，日志流主动短连接续连，避免被超时中间件截断。
	deadline := time.NewTimer(25 * time.Second)
	defer deadline.Stop()
	lastLogContent := ""
	lastStagePayload := ""
	lastInputPayload := ""
	// 跟踪流水线完成状态，连续 N 轮未变化且全部阶段结束才发送 end 事件
	var lastStageResp *types.ListDevopsPipelineRunStageResponse
	var lastLogResp *types.DevopsPipelineRunLogResponse
	doneStableCount := 0
	const doneStableThreshold = 3

	fetchAndSend := func() bool {
		logLogic := pipeline.NewDevopsPipelineRunLogLogic(r.Context(), svcCtx)
		logResp, err := logLogic.DevopsPipelineRunLog(&req)
		lastLogResp = logResp
		if err != nil {
			return send("log-error", map[string]string{"message": err.Error()})
		}
		logResp.Changed = logResp.Content != lastLogContent
		if logResp.Changed {
			lastLogContent = logResp.Content
		}
		if !send("log", logResp) {
			return false
		}

		stageLogic := pipeline.NewDevopsPipelineRunStageListLogic(r.Context(), svcCtx)
		stageResp, err := stageLogic.DevopsPipelineRunStageList(&types.ListDevopsPipelineRunStageRequest{RunId: req.RunId})
		lastStageResp = stageResp
		if err != nil {
			return send("log-error", map[string]string{"message": err.Error()})
		}
		if stagePayload, marshalErr := json.Marshal(stageResp); marshalErr == nil {
			if string(stagePayload) != lastStagePayload {
				lastStagePayload = string(stagePayload)
				if !send("stages", stageResp) {
					return false
				}
			}
		} else if !send("stages", stageResp) {
			return false
		}

		if pipelineRunStageNeedInput(stageResp) || lastInputPayload != "" {
			inputLogic := pipeline.NewDevopsPipelineRunInputListLogic(r.Context(), svcCtx)
			inputResp, err := inputLogic.DevopsPipelineRunInputList(&types.ListDevopsPipelineRunInputRequest{RunId: req.RunId})
			if err == nil {
				if inputPayload, marshalErr := json.Marshal(inputResp); marshalErr == nil {
					if string(inputPayload) != lastInputPayload {
						lastInputPayload = string(inputPayload)
						if !send("input", inputResp) {
							return false
						}
					}
				} else if !send("input", inputResp) {
					return false
				}
			}
		}

		return true
	}

	sendEndIfDone := func() bool {
		if lastStageResp == nil || lastLogResp == nil {
			return false
		}
		if !pipelineRunStageStreamDone(lastStageResp) {
			doneStableCount = 0
			return false
		}
		if lastLogResp.Changed {
			doneStableCount = 0
			return false
		}
		doneStableCount++
		if doneStableCount >= doneStableThreshold {
			return true
		}
		return false
	}

	for {
		if !fetchAndSend() {
			return
		}
		if sendEndIfDone() {
			_ = send("end", map[string]string{"status": "completed"})
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if !fetchAndSend() {
				return
			}
			if sendEndIfDone() {
				_ = send("end", map[string]string{"status": "completed"})
				return
			}
		case <-heartbeat.C:
			if !send("ping", map[string]int64{"time": time.Now().Unix()}) {
				return
			}
		case <-deadline.C:
			_ = send("reconnect", map[string]string{"status": "continue"})
			return
		}
	}
}

func pipelineRunStageNeedInput(resp *types.ListDevopsPipelineRunStageResponse) bool {
	if resp == nil {
		return false
	}
	for _, item := range resp.Items {
		if strings.EqualFold(strings.TrimSpace(item.Status), "paused") {
			return true
		}
	}
	return false
}

func pipelineRunStageStreamDone(resp *types.ListDevopsPipelineRunStageResponse) bool {
	if resp == nil || len(resp.Items) == 0 {
		return false
	}
	for _, item := range resp.Items {
		switch strings.ToLower(strings.TrimSpace(item.Status)) {
		case "success", "failed", "failure", "unstable", "aborted", "skipped", "finished", "not_built", "notbuilt":
			continue
		default:
			return false
		}
	}
	return true
}
