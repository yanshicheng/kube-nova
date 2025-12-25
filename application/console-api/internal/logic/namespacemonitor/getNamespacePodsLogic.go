package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespacePodsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace Pod 统计
func NewGetNamespacePodsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespacePodsLogic {
	return &GetNamespacePodsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespacePodsLogic) GetNamespacePods(req *types.GetNamespacePodsRequest) (resp *types.GetNamespacePodsResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	metrics, err := namespace.GetNamespacePods(req.Namespace, timeRange)
	if err != nil {
		l.Errorf("获取 Namespace Pods 失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespacePodsResponse{
		Data: convertNamespacePodStatistics(metrics),
	}

	l.Infof("获取 Namespace %s Pods 成功", req.Namespace)
	return resp, nil
}
