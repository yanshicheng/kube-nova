package canary

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8stypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CanaryGetDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Canary 详情
func NewCanaryGetDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CanaryGetDetailLogic {
	return &CanaryGetDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CanaryGetDetailLogic) CanaryGetDetail(req *types.CanaryNameRequest) (resp *types.CanaryDetail, err error) {
	workload, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{
		Id: req.WorkloadId,
	})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败: %v", err)
	}
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workload.Data.ClusterUuid)
	canaryOperator := client.Flagger()
	detail, err := canaryOperator.GetDetail(workload.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Canary 详情失败: %v", err)
		return nil, fmt.Errorf("获取 Canary 详情失败: %v", err)
	}

	// 转换类型
	resp = l.convertCanaryDetail(detail)

	l.Infof("获取 Canary 详情成功: %s", req.Name)
	return resp, nil
}

// 转换 CanaryDetail
func (l *CanaryGetDetailLogic) convertCanaryDetail(detail *k8stypes.CanaryDetail) *types.CanaryDetail {
	result := &types.CanaryDetail{
		Name:              detail.Name,
		Namespace:         detail.Namespace,
		TargetRef:         l.convertTargetRefInfo(detail.TargetRef),
		ProgressDeadline:  detail.ProgressDeadline,
		Status:            detail.Status,
		CanaryWeight:      detail.CanaryWeight,
		FailedChecks:      detail.FailedChecks,
		Phase:             detail.Phase,
		LastTransition:    detail.LastTransition,
		Age:               detail.Age,
		CreationTimestamp: detail.CreationTimestamp,
		Analysis:          l.convertCanaryAnalysis(detail.Analysis),
		Service:           l.convertCanaryService(detail.Service),
	}

	// 处理可选字段
	if len(detail.Labels) > 0 {
		result.Labels = detail.Labels
	}

	if len(detail.Annotations) > 0 {
		result.Annotations = detail.Annotations
	}

	return result
}

// 转换 TargetRefInfo
func (l *CanaryGetDetailLogic) convertTargetRefInfo(ref k8stypes.TargetRefInfo) types.TargetRefInfo {
	return types.TargetRefInfo{
		Kind:       ref.Kind,
		Name:       ref.Name,
		ApiVersion: ref.APIVersion,
	}
}

// 转换 CanaryAnalysis
func (l *CanaryGetDetailLogic) convertCanaryAnalysis(analysis k8stypes.CanaryAnalysis) types.CanaryAnalysis {
	result := types.CanaryAnalysis{
		Interval:   analysis.Interval,
		Threshold:  analysis.Threshold,
		MaxWeight:  analysis.MaxWeight,
		StepWeight: analysis.StepWeight,
		Iterations: analysis.Iterations,
	}

	// 转换 Metrics
	if len(analysis.Metrics) > 0 {
		result.Metrics = l.convertMetrics(analysis.Metrics)
	}

	// 转换 Webhooks
	if len(analysis.Webhooks) > 0 {
		result.Webhooks = l.convertWebhooks(analysis.Webhooks)
	}

	// 转换 Match
	if len(analysis.Match) > 0 {
		result.Match = l.convertMatchInfo(analysis.Match)
	}

	return result
}

// 转换 Metrics
func (l *CanaryGetDetailLogic) convertMetrics(metrics []k8stypes.MetricInfo) []types.MetricInfo {
	if len(metrics) == 0 {
		return nil
	}

	result := make([]types.MetricInfo, 0, len(metrics))
	for _, m := range metrics {
		metric := types.MetricInfo{
			Name:     m.Name,
			Interval: m.Interval,
			Query:    m.Query,
		}

		// 处理 ThresholdRange（指针类型）
		if m.ThresholdRange != nil {
			metric.ThresholdRange = types.ThresholdRange{}
			if m.ThresholdRange.Min != nil {
				metric.ThresholdRange.Min = *m.ThresholdRange.Min
			}
			if m.ThresholdRange.Max != nil {
				metric.ThresholdRange.Max = *m.ThresholdRange.Max
			}
		}

		// 处理 TemplateRef（指针类型）
		if m.TemplateRef != nil {
			metric.TemplateRef = types.TemplateRef{
				Name:      m.TemplateRef.Name,
				Namespace: m.TemplateRef.Namespace,
			}
		}

		result = append(result, metric)
	}
	return result
}

// 转换 Webhooks
func (l *CanaryGetDetailLogic) convertWebhooks(webhooks []k8stypes.WebhookInfo) []types.WebhookInfo {
	if len(webhooks) == 0 {
		return nil
	}

	result := make([]types.WebhookInfo, 0, len(webhooks))
	for _, w := range webhooks {
		webhook := types.WebhookInfo{
			Name:     w.Name,
			Type:     w.Type,
			Url:      w.URL,
			Timeout:  w.Timeout,
			Metadata: w.Metadata,
		}
		result = append(result, webhook)
	}
	return result
}

// 转换 MatchInfo
func (l *CanaryGetDetailLogic) convertMatchInfo(matches []k8stypes.MatchInfo) []types.MatchInfo {
	if len(matches) == 0 {
		return nil
	}

	result := make([]types.MatchInfo, 0, len(matches))
	for _, m := range matches {
		match := types.MatchInfo{
			Headers: make(map[string]types.StringMatch),
		}

		// 转换 Headers
		for key, sm := range m.Headers {
			match.Headers[key] = types.StringMatch{
				Exact:  sm.Exact,
				Prefix: sm.Prefix,
				Suffix: sm.Suffix,
				Regex:  sm.Regex,
			}
		}

		result = append(result, match)
	}
	return result
}

// 转换 CanaryService
func (l *CanaryGetDetailLogic) convertCanaryService(service k8stypes.CanaryService) types.CanaryService {
	result := types.CanaryService{
		Port:       service.Port,
		TargetPort: service.TargetPort,
		Name:       service.Name,
		PortName:   service.PortName,
		Gateways:   service.Gateways,
		Hosts:      service.Hosts,
	}

	// 处理 TrafficPolicy
	if service.TrafficPolicy != nil {
		result.TrafficPolicy = types.TrafficPolicy{}

		// 处理 TLS
		if service.TrafficPolicy.TLS != nil {
			result.TrafficPolicy.Tls = types.TLSPolicy{
				Mode: service.TrafficPolicy.TLS.Mode,
			}
		}
	}

	return result
}
