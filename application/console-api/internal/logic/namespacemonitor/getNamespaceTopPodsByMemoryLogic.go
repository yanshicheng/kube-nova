package namespacemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNamespaceTopPodsByMemoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Namespace Top Pods (Memory)
func NewGetNamespaceTopPodsByMemoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNamespaceTopPodsByMemoryLogic {
	return &GetNamespaceTopPodsByMemoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNamespaceTopPodsByMemoryLogic) GetNamespaceTopPodsByMemory(req *types.GetNamespaceTopPodsByMemoryRequest) (resp *types.GetNamespaceTopPodsByMemoryResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	namespace := client.Namespace()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	rankings, err := namespace.GetTopPodsByMemory(req.Namespace, req.Limit, timeRange)
	if err != nil {
		l.Errorf("获取 Namespace Top Pods by Memory 失败: %v", err)
		return nil, err
	}

	resp = &types.GetNamespaceTopPodsByMemoryResponse{
		Data: convertResourceRankingList(rankings),
	}

	l.Infof("获取 Namespace %s Top %d Pods by Memory 成功", req.Namespace, req.Limit)
	return resp, nil
}
