package inspection

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

const inspectionLogExpireSeconds = 24 * 60 * 60

type recordLogPayload struct {
	Time    int64  `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

func inspectionRecordLogKey(recordID uint64) string {
	return fmt.Sprintf("inspection:record:logs:%d", recordID)
}

func appendRecordLog(ctx context.Context, svcCtx *svc.ServiceContext, recordID uint64, level, message string) {
	if svcCtx == nil || svcCtx.Cache == nil || recordID == 0 || message == "" {
		return
	}
	if ctx == nil || ctx.Err() != nil {
		ctx = context.Background()
	}
	if level == "" {
		level = "info"
	}
	payload := recordLogPayload{
		Time:    time.Now().Unix(),
		Level:   level,
		Message: message,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		logx.WithContext(ctx).Errorf("序列化巡检执行日志失败: %v", err)
		return
	}
	key := inspectionRecordLogKey(recordID)
	if _, err := svcCtx.Cache.RpushCtx(ctx, key, string(data)); err != nil {
		logx.WithContext(ctx).Errorf("写入巡检执行日志失败: recordId=%d, err=%v", recordID, err)
		return
	}
	if err := svcCtx.Cache.ExpireCtx(ctx, key, inspectionLogExpireSeconds); err != nil {
		logx.WithContext(ctx).Errorf("设置巡检执行日志过期时间失败: recordId=%d, err=%v", recordID, err)
	}
}

func ClearRecordLog(ctx context.Context, svcCtx *svc.ServiceContext, recordID uint64) {
	if svcCtx == nil || svcCtx.Cache == nil || recordID == 0 {
		return
	}
	if ctx == nil || ctx.Err() != nil {
		ctx = context.Background()
	}
	if _, err := svcCtx.Cache.DelCtx(ctx, inspectionRecordLogKey(recordID)); err != nil {
		logx.WithContext(ctx).Errorf("清理巡检执行日志失败: recordId=%d, err=%v", recordID, err)
	}
}

func logLevelByStatus(status string) string {
	switch status {
	case statusSuccess:
		return "info"
	case statusWarning, statusPartial:
		return "warning"
	case statusCritical, statusFailed:
		return "error"
	default:
		return "info"
	}
}

func statusRank(status string) int {
	switch status {
	case statusCritical, statusFailed:
		return 0
	case statusWarning, statusPartial:
		return 1
	case statusRunning:
		return 2
	case statusSuccess:
		return 3
	default:
		return 4
	}
}
