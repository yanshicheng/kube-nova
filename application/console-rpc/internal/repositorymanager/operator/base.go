package operator

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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

func NewBaseOperator(ctx context.Context, endpoint, username, password string, insecure bool, caCert string) (*BaseOperator, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecure,
	}

	// 添加 CA 证书
	if caCert != "" && !insecure {
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM([]byte(caCert)) {
			return nil, fmt.Errorf("解析 CA 证书失败")
		}
		tlsConfig.RootCAs = certPool
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
			Timeout:   30 * time.Second,
		},
	}, nil
}

// ResponseWithHeaders 包含响应数据和响应头
type ResponseWithHeaders struct {
	Data    interface{}
	Headers http.Header
}

// doRequest 发送 HTTP 请求
func (b *BaseOperator) doRequest(method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			b.log.Errorf("序列化请求失败: %v", err)
			return fmt.Errorf("序列化请求失败")
		}
		reqBody = bytes.NewReader(data)
	}

	requestURL := b.endpoint + path
	req, err := http.NewRequestWithContext(b.ctx, method, requestURL, reqBody)
	if err != nil {
		b.log.Errorf("创建请求失败: %v", err)
		return fmt.Errorf("创建请求失败")
	}

	req.SetBasicAuth(b.username, b.password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	b.log.Debugf("发起请求: %s %s", method, requestURL)

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

	// 记录响应头中的重要信息
	if totalCount := resp.Header.Get("X-Total-Count"); totalCount != "" {
		b.log.Debugf("响应总数: %s", totalCount)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b.log.Errorf("Harbor API 错误: status=%d, body=%s", resp.StatusCode, string(respBody))
		return fmt.Errorf("Harbor API 错误: status=%d", resp.StatusCode)
	}

	if result != nil && len(respBody) > 0 {
		// 检查响应 Content-Type
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			b.log.Errorf("期望 JSON 响应但收到 Content-Type: %s, URL: %s, body: %s", contentType, requestURL, string(respBody))
			// 如果返回 HTML，可能是认证失败或路径错误
			if strings.Contains(contentType, "text/html") {
				return fmt.Errorf("收到 HTML 响应而非 JSON，可能是认证失败或 API 路径错误")
			}
			return fmt.Errorf("期望 JSON 响应但收到: %s", contentType)
		}

		if err := json.Unmarshal(respBody, result); err != nil {
			b.log.Errorf("解析响应失败: %v, body=%s", err, string(respBody))
			return fmt.Errorf("解析响应失败")
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

// getTotalCountFromHeader 从响应头获取总数
func (b *BaseOperator) getTotalCountFromHeader(headers *http.Header) int {
	if headers == nil {
		return 0
	}

	totalCountStr := headers.Get("X-Total-Count")
	if totalCountStr == "" {
		return 0
	}

	totalCount, err := strconv.Atoi(totalCountStr)
	if err != nil {
		b.log.Errorf("解析 X-Total-Count 失败: %v", err)
		return 0
	}

	return totalCount
}

// doRequestWithHeaders 发送请求并返回原始响应（支持自定义头）
func (b *BaseOperator) doRequestWithHeaders(method, path string, body interface{}, headers map[string]string) ([]byte, error) {
	url := b.endpoint + path

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求失败: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
		b.log.Infof("请求 Body: %s", string(jsonData))
	}

	req, err := http.NewRequestWithContext(b.ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置认证
	req.SetBasicAuth(b.username, b.password)

	// 设置自定义头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 如果没有设置 Content-Type，默认设置为 application/json
	if _, ok := headers["Content-Type"]; !ok && body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	b.log.Infof("发送请求: %s %s", method, url)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if len(respBody) > 1000 {
		b.log.Infof("响应 Body (前1000字符): %s", string(respBody[:1000]))
	} else {
		b.log.Infof("响应 Body: %s", string(respBody))
	}

	// 检查状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Harbor API 错误: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
