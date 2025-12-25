package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CompareNamespacesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 对比多个 Namespace
func NewCompareNamespacesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompareNamespacesLogic {
	return &CompareNamespacesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CompareNamespacesLogic) CompareNamespaces(req *types.CompareNamespacesRequest) (resp *types.CompareNamespacesResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	comparison, err := namespace.CompareNamespaces(req.Namespaces, timeRange)
	if err != nil {
		l.Errorf("对比 Namespaces 失败: %v", err)
		return nil, err
	}

	resp = &types.CompareNamespacesResponse{
		Data: convertNamespaceComparison(comparison),
	}

	l.Infof("对比 %d 个 Namespaces 成功", len(req.Namespaces))
	return resp, nil
}
