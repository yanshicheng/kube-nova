package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetTopPodsByMemoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取内存 Top Pods
func NewGetTopPodsByMemoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTopPodsByMemoryLogic {
	return &GetTopPodsByMemoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetTopPodsByMemoryLogic) GetTopPodsByMemory(req *types.GetTopPodsByMemoryRequest) (resp *types.GetTopPodsByMemoryResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	rankings, err := pod.GetTopPodsByMemory(req.Namespace, req.Limit, timeRange)
	if err != nil {
		l.Errorf("获取内存 Top Pods 失败: Error=%v", err)
		return nil, err
	}

	resp = &types.GetTopPodsByMemoryResponse{
		Data: make([]types.PodResourceRanking, 0, len(rankings)),
	}

	for _, ranking := range rankings {
		resp.Data = append(resp.Data, types.PodResourceRanking{
			Namespace: ranking.Namespace,
			PodName:   ranking.PodName,
			Value:     ranking.Value,
			Unit:      ranking.Unit,
		})
	}

	l.Infof("获取内存 Top Pods 成功: ResultCount=%d", len(resp.Data))
	return resp, nil
}
