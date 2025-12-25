package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceTopPodsByCPULogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace Top Pods (CPU)
func NewGetNamespaceTopPodsByCPULogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceTopPodsByCPULogic {
	return &GetNamespaceTopPodsByCPULogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceTopPodsByCPULogic) GetNamespaceTopPodsByCPU(req *types.GetNamespaceTopPodsByCPURequest) (resp *types.GetNamespaceTopPodsByCPUResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	rankings, err := namespace.GetTopPodsByCPU(req.Namespace, req.Limit, timeRange)
	if err != nil {
		l.Errorf("获取 Namespace Top Pods by CPU 失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespaceTopPodsByCPUResponse{
		Data: convertResourceRankingList(rankings),
	}

	l.Infof("获取 Namespace %s Top %d Pods by CPU 成功", req.Namespace, req.Limit)
	return resp, nil
}
