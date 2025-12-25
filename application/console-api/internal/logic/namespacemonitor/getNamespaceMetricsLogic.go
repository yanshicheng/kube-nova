package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace 综合指标
func NewGetNamespaceMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceMetricsLogic {
	return &GetNamespaceMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceMetricsLogic) GetNamespaceMetrics(req *types.GetNamespaceMetricsRequest) (resp *types.GetNamespaceMetricsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	metrics, err := namespace.GetNamespaceMetrics(req.Namespace, timeRange)
	if err != nil {
		l.Errorf("获取 Namespace 综合指标失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespaceMetricsResponse{
		Data: convertNamespaceMetrics(metrics),
	}

	l.Infof("获取 Namespace %s 综合指标成功", req.Namespace)
	return resp, nil
}
