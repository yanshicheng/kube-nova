package operator

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/common/prometheusmanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type BaseOperator struct {
	log      logx.Logger
	ctx      context.Context
	endpoint string
	username string
	password string
	client   *http.Client
}

func NewBaseOperator(ctx context.Context, endpoint, username, password string, insecure bool, timeout int) (*BaseOperator, error) {
	if timeout <= 0 {
		timeout = 30
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecure,
	}

	transport := &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	return &BaseOperator{
		log:      logx.WithContext(ctx),
		ctx:      ctx,
		endpoint: strings.TrimRight(endpoint, "/"),
		username: username,
		password: password,
		client: &http.Client{
			Transport: transport,
			Timeout:   time.Duration(timeout) * time.Second,
		},
	}, nil
}

// doRequest 发送 HTTP 请求
func (b *BaseOperator) doRequest(method, path string, params map[string]string, body interface{}, result interface{}) error {
	requestURL := b.endpoint + path

	// 构建查询参数
	if len(params) > 0 {
		query := url.Values{}
		for k, v := range params {
			if v != "" {
				query.Add(k, v)
			}
		}
		requestURL += "?" + query.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			b.log.Errorf("序列化请求失败: %v", err)
			return fmt.Errorf("序列化请求失败")
		}
		reqBody = strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(b.ctx, method, requestURL, reqBody)
	if err != nil {
		b.log.Errorf("创建请求失败: %v", err)
		return fmt.Errorf("创建请求失败")
	}

	// 设置认证
	if b.username != "" && b.password != "" {
		req.SetBasicAuth(b.username, b.password)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		b.log.Errorf("请求失败: %v", err)
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		b.log.Errorf("读取响应失败: %v", err)
		return fmt.Errorf("读取响应失败")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b.log.Errorf("Prometheus API 错误: status=%d, body=%s", resp.StatusCode, string(respBody))
		return fmt.Errorf("Prometheus API 错误: status=%d", resp.StatusCode)
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			b.log.Errorf("解析响应失败: %v, body=%s", err, string(respBody))
			return fmt.Errorf("解析响应失败: %v", err)
		}
	}

	return nil
}

// buildQuery 构建查询字符串
func (b *BaseOperator) buildQuery(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	query := url.Values{}
	for k, v := range params {
		if v != "" {
			query.Add(k, v)
		}
	}
	return "?" + query.Encode()
}

// bytesToGB 字节转换为 GB
func bytesToGB(bytes int64) float64 {
	return float64(bytes) / 1024 / 1024 / 1024
}

// truncateString 截断字符串
func (b *BaseOperator) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (b *BaseOperator) formatTimestamp(t time.Time) string {
	// 返回带3位小数的Unix时间戳
	return fmt.Sprintf("%.3f", float64(t.UnixNano())/1e9)
}

// calculateStep 自动计算合理的 step
// 目标：返回 200-500 个数据点
func (b *BaseOperator) calculateStep(start, end time.Time) string {
	duration := end.Sub(start)

	// 根据计算结果，向上取整到合理的步长
	switch {
	case duration < 5*time.Minute:
		return "1s" // 5min / 1s = 300 点
	case duration < 10*time.Minute:
		return "2s" // 10min / 2s = 300 点
	case duration < 30*time.Minute:
		return "5s" // 30min / 5s = 360 点
	case duration < 1*time.Hour:
		return "15s" // 1h / 15s = 240 点
	case duration < 3*time.Hour:
		return "30s" // 3h / 30s = 360 点
	case duration < 6*time.Hour:
		return "1m" // 6h / 1m = 360 点
	case duration < 12*time.Hour:
		return "2m" // 12h / 2m = 360 点
	case duration < 24*time.Hour:
		return "5m" // 24h / 5m = 288 点
	case duration < 3*24*time.Hour:
		return "10m" // 3d / 10m = 432 点
	case duration < 7*24*time.Hour:
		return "30m" // 7d / 30m = 336 点
	case duration < 30*24*time.Hour:
		return "2h" // 30d / 2h = 360 点
	default:
		return "6h" // >30d / 6h = 合理点数
	}
}

// calculateRateWindow 根据 timeRange 计算合适的 rate 窗口
// 原则：Window = Step × 4（Prometheus 最佳实践）
func (b *BaseOperator) calculateRateWindow(timeRange *types.TimeRange) string {
	if timeRange == nil || timeRange.Start.IsZero() || timeRange.End.IsZero() {
		return "5m" // 默认值
	}

	duration := timeRange.End.Sub(timeRange.Start)

	// 根据 duration 计算 step，然后 window = step × 4
	switch {
	case duration < 5*time.Minute:
		// step=1s → window=4s
		return "5s" // 稍微放大，确保有足够数据
	case duration < 10*time.Minute:
		// step=2s → window=8s
		return "10s"
	case duration < 30*time.Minute:
		// step=5s → window=20s
		return "20s"
	case duration < 1*time.Hour:
		// step=15s → window=1m
		return "1m"
	case duration < 3*time.Hour:
		// step=30s → window=2m
		return "2m"
	case duration < 6*time.Hour:
		// step=1m → window=4m
		return "4m"
	case duration < 12*time.Hour:
		// step=2m → window=8m
		return "8m"
	case duration < 24*time.Hour:
		// step=5m → window=20m
		return "20m"
	case duration < 3*24*time.Hour:
		// step=10m → window=40m
		return "40m"
	case duration < 7*24*time.Hour:
		// step=30m → window=2h
		return "2h"
	case duration < 30*24*time.Hour:
		// step=2h → window=8h
		return "8h"
	default:
		// step=6h → window=24h
		return "24h"
	}
}
