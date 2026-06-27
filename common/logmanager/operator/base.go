package operator

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	logtypes "github.com/yanshicheng/kube-nova/common/logmanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type BaseOperator struct {
	log      logx.Logger
	ctx      context.Context
	endpoint string
	username string
	password string
	token    string
	client   *http.Client
}

func NewBaseOperator(ctx context.Context, endpoint, username, password, token string, insecure bool, timeout int, pool logtypes.HTTPPoolConfig) (*BaseOperator, error) {
	if timeout <= 0 {
		timeout = 30
	}
	maxIdleConns := pool.MaxIdleConns
	if maxIdleConns <= 0 {
		maxIdleConns = 100
	}
	maxIdleConnsPerHost := pool.MaxIdleConnsPerHost
	if maxIdleConnsPerHost <= 0 {
		maxIdleConnsPerHost = 20
	}
	maxConnsPerHost := pool.MaxConnsPerHost
	if maxConnsPerHost <= 0 {
		maxConnsPerHost = 50
	}
	idleConnTimeout := pool.IdleConnTimeout
	if idleConnTimeout <= 0 {
		idleConnTimeout = 90 * time.Second
	}
	responseHeaderTimeout := pool.ResponseHeaderTimeout
	if responseHeaderTimeout <= 0 {
		responseHeaderTimeout = 15 * time.Second
	}
	tlsHandshakeTimeout := pool.TLSHandshakeTimeout
	if tlsHandshakeTimeout <= 0 {
		tlsHandshakeTimeout = 10 * time.Second
	}
	expectContinueTimeout := pool.ExpectContinueTimeout
	if expectContinueTimeout <= 0 {
		expectContinueTimeout = 1 * time.Second
	}
	transport := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: insecure},
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		MaxConnsPerHost:       maxConnsPerHost,
		IdleConnTimeout:       idleConnTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
	}
	return &BaseOperator{
		log:      logx.WithContext(ctx),
		ctx:      ctx,
		endpoint: strings.TrimRight(endpoint, "/"),
		username: username,
		password: password,
		token:    token,
		client:   &http.Client{Transport: transport, Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

func (b *BaseOperator) newRequest(method, path string, params map[string]string, body io.Reader, contentType string) (*http.Request, error) {
	requestURL := b.endpoint + path
	if len(params) > 0 {
		query := url.Values{}
		for k, v := range params {
			if v != "" {
				query.Add(k, v)
			}
		}
		requestURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(b.ctx, method, requestURL, body)
	if err != nil {
		return nil, err
	}
	if b.username != "" && b.password != "" {
		req.SetBasicAuth(b.username, b.password)
	}
	if b.token != "" {
		req.Header.Set("Authorization", "Bearer "+b.token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

func (b *BaseOperator) DoJSONRequest(method, path string, params map[string]string, body any, result any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("序列化请求失败: %v", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := b.newRequest(method, path, params, reader, "application/json")
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API 错误: status=%d, body=%s", resp.StatusCode, string(respBody))
	}
	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("解析响应失败: %v", err)
		}
	}
	return nil
}

func (b *BaseOperator) DoTextRequest(method, path string, params map[string]string, body string, contentType string) ([]byte, int, error) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := b.newRequest(method, path, params, reader, contentType)
	if err != nil {
		return nil, 0, fmt.Errorf("创建请求失败: %v", err)
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("读取响应失败: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return data, resp.StatusCode, fmt.Errorf("API 错误: status=%d, body=%s", resp.StatusCode, string(data))
	}
	return data, resp.StatusCode, nil
}
