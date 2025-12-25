package operator

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/yanshicheng/kube-nova/common/prometheusmanager/types"
)

type PrometheusClientImpl struct {
	*BaseOperator
	uuid     string
	name     string
	endpoint string
}

func NewPrometheusClient(ctx context.Context, config *types.PrometheusConfig) (types.PrometheusClient, error) {
	if config.UUID == "" || config.Endpoint == "" {
		return nil, fmt.Errorf("配置无效: uuid、endpoint 不能为空")
	}

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 30
	}

	base, err := NewBaseOperator(ctx, config.Endpoint, config.Username, config.Password, config.Insecure, timeout)
	if err != nil {
		return nil, fmt.Errorf("创建基础操作器失败: %w", err)
	}

	return &PrometheusClientImpl{
		BaseOperator: base,
		uuid:         config.UUID,
		name:         config.Name,
		endpoint:     config.Endpoint,
	}, nil
}

func (p *PrometheusClientImpl) GetUUID() string {
	return p.uuid
}

func (p *PrometheusClientImpl) GetName() string {
	return p.name
}

func (p *PrometheusClientImpl) GetEndpoint() string {
	return p.endpoint
}

// Query 即时查询
func (p *PrometheusClientImpl) Query(query string, timestamp *time.Time) ([]types.InstantQueryResult, error) {

	params := map[string]string{
		"query": query,
	}

	if timestamp != nil {
		params["time"] = p.formatTimestamp(*timestamp)
	}

	var response struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Value  []interface{}     `json:"value"` // [timestamp, value]
			} `json:"result"`
		} `json:"data"`
		Error string `json:"error,omitempty"`
	}

	if err := p.doRequest("GET", "/api/v1/query", params, nil, &response); err != nil {
		return nil, err
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("查询失败: %s", response.Error)
	}

	results := make([]types.InstantQueryResult, 0, len(response.Data.Result))
	for _, item := range response.Data.Result {
		if len(item.Value) != 2 {
			continue
		}

		timestamp := int64(item.Value[0].(float64))
		valueStr := item.Value[1].(string)
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			p.log.Errorf("解析值失败: %v", err)
			continue
		}

		results = append(results, types.InstantQueryResult{
			Metric: item.Metric,
			Value:  value,
			Time:   time.Unix(timestamp, 0),
		})
	}

	return results, nil
}

// QueryRange 范围查询
func (p *PrometheusClientImpl) QueryRange(query string, start, end time.Time, step string) ([]types.RangeQueryResult, error) {

	params := map[string]string{
		"query": query,
		"start": p.formatTimestamp(start),
		"end":   p.formatTimestamp(end),
		"step":  step,
	}

	var response struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Values [][]interface{}   `json:"values"` // [[timestamp, value], ...]
			} `json:"result"`
		} `json:"data"`
		Error string `json:"error,omitempty"`
	}

	if err := p.doRequest("GET", "/api/v1/query_range", params, nil, &response); err != nil {
		return nil, err
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("查询失败: %s", response.Error)
	}

	results := make([]types.RangeQueryResult, 0, len(response.Data.Result))
	for _, item := range response.Data.Result {
		values := make([]types.MetricValue, 0, len(item.Values))

		for _, v := range item.Values {
			if len(v) != 2 {
				continue
			}

			timestamp := int64(v[0].(float64))
			valueStr := v[1].(string)
			value, err := strconv.ParseFloat(valueStr, 64)
			if err != nil {
				p.log.Errorf("解析值失败: %v", err)
				continue
			}

			values = append(values, types.MetricValue{
				Timestamp: time.Unix(timestamp, 0),
				Value:     value,
			})
		}

		results = append(results, types.RangeQueryResult{
			Metric: item.Metric,
			Values: values,
		})
	}

	return results, nil
}

// Pod 返回 Pod 操作器
func (p *PrometheusClientImpl) Pod() types.PodOperator {
	return NewPodOperator(p.ctx, p.BaseOperator)
}

// Node 节点操作器
func (p *PrometheusClientImpl) Node() types.NodeOperator {
	return NewNodeOperator(p.ctx, p.BaseOperator)
}

// Namespace 命名空间操作器
func (p *PrometheusClientImpl) Namespace() types.NamespaceOperator {
	return NewNamespaceOperator(p.ctx, p.BaseOperator)
}

// cluster 集群操作器
func (p *PrometheusClientImpl) Cluster() types.ClusterOperator {
	return NewClusterOperator(p.ctx, p.BaseOperator)
}

// ingress
func (p *PrometheusClientImpl) Ingress() types.IngressOperator {
	return NewIngressOperator(p.ctx, p.BaseOperator)
}

// flagger
func (p *PrometheusClientImpl) Flagger() types.FlaggerOperator {
	return NewFlaggerOperator(p.ctx, p.BaseOperator)
}

// Ping 健康检查
func (p *PrometheusClientImpl) Ping() error {
	p.log.Info("测试连接")

	url := p.endpoint + "/-/healthy"
	req, err := http.NewRequestWithContext(p.ctx, "GET", url, nil)
	if err != nil {
		p.log.Errorf("创建 Ping 请求失败: %v", err)
		return fmt.Errorf("创建 Ping 请求失败")
	}

	if p.username != "" && p.password != "" {
		req.SetBasicAuth(p.username, p.password)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		p.log.Errorf("Ping 请求失败: %v", err)
		return fmt.Errorf("ping 请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		p.log.Errorf("Ping 失败: status=%d", resp.StatusCode)
		return fmt.Errorf("ping 失败: status=%d", resp.StatusCode)
	}

	p.log.Info("连接测试成功")
	return nil
}

// Close 关闭客户端
func (p *PrometheusClientImpl) Close() error {
	p.log.Info("关闭客户端")
	p.client.CloseIdleConnections()
	return nil
}
