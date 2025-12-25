package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceTopContainersByCPULogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace Top Containers (CPU)
func NewGetNamespaceTopContainersByCPULogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceTopContainersByCPULogic {
	return &GetNamespaceTopContainersByCPULogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceTopContainersByCPULogic) GetNamespaceTopContainersByCPU(req *types.GetNamespaceTopContainersByCPURequest) (resp *types.GetNamespaceTopContainersByCPUResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	rankings, err := namespace.GetTopContainersByCPU(req.Namespace, req.Limit, timeRange)
	if err != nil {
		l.Errorf("获取 Namespace Top Containers by CPU 失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespaceTopContainersByCPUResponse{
		Data: convertContainerResourceRankingList(rankings),
	}

	l.Infof("获取 Namespace %s Top %d Containers by CPU 成功", req.Namespace, req.Limit)
	return resp, nil
}
