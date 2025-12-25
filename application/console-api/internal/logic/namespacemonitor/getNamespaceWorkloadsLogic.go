package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceWorkloadsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace 工作负载统计
func NewGetNamespaceWorkloadsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceWorkloadsLogic {
	return &GetNamespaceWorkloadsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceWorkloadsLogic) GetNamespaceWorkloads(req *types.GetNamespaceWorkloadsRequest) (resp *types.GetNamespaceWorkloadsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	metrics, err := namespace.GetNamespaceWorkloads(req.Namespace, timeRange)
	if err != nil {
		l.Errorf("获取 Namespace Workloads 失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespaceWorkloadsResponse{
		Data: convertNamespaceWorkloadMetrics(metrics),
	}

	l.Infof("获取 Namespace %s Workloads 成功", req.Namespace)
	return resp, nil
}
