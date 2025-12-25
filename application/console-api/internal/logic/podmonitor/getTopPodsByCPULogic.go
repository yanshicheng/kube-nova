package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetTopPodsByCPULogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 CPU Top Pods
func NewGetTopPodsByCPULogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTopPodsByCPULogic {
	return &GetTopPodsByCPULogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetTopPodsByCPULogic) GetTopPodsByCPU(req *types.GetTopPodsByCPURequest) (resp *types.GetTopPodsByCPUResponse, err error) {
	// 1. 获取 Prometheus 客户端
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	// 2. 获取 Pod 操作器
	pod := client.Pod()

	// 3. 构建时间范围
	timeRange := parseRangeTime(req.Start, req.End, req.Step)

	// 4. 调用 Pod 操作器获取 Top Pods
	rankings, err := pod.GetTopPodsByCPU(req.Namespace, req.Limit, timeRange)
	if err != nil {
		l.Errorf("获取 CPU Top Pods 失败: Namespace=%s, Limit=%d, Error=%v",
			req.Namespace, req.Limit, err)
		return nil, err
	}

	// 5. 转换为 API 响应格式
	resp = &types.GetTopPodsByCPUResponse{
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

	l.Infof("获取 CPU Top Pods 成功: Namespace=%s, Limit=%d, ResultCount=%d",
		req.Namespace, req.Limit, len(resp.Data))

	return resp, nil
}
