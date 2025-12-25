package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListPodsMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列出 Pods 指标
func NewListPodsMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPodsMetricsLogic {
	return &ListPodsMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPodsMetricsLogic) ListPodsMetrics(req *types.ListPodsMetricsRequest) (resp *types.ListPodsMetricsResponse, err error) {
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

	// 4. 调用 Pod 操作器获取 Pods 列表
	overviews, err := pod.ListPodsMetrics(req.Namespace, timeRange)
	if err != nil {
		l.Errorf("获取 Pods 列表失败: Namespace=%s, Error=%v", req.Namespace, err)
		return nil, err
	}

	// 5. 转换为 API 响应格式
	resp = &types.ListPodsMetricsResponse{
		Data: make([]types.PodOverview, 0, len(overviews)),
	}

	for _, overview := range overviews {
		podOverview := types.PodOverview{
			Namespace: overview.Namespace,
			PodName:   overview.PodName,
			Status: types.PodStatusSnapshot{
				Timestamp: overview.Status.Timestamp.Unix(),
				Phase:     overview.Status.Phase,
				Ready:     overview.Status.Ready,
				Reason:    overview.Status.Reason,
				Message:   overview.Status.Message,
			},
			CPU: types.CPUUsageSnapshot{
				Timestamp:     overview.CPU.Timestamp.Unix(),
				UsageCores:    overview.CPU.UsageCores,
				UsagePercent:  overview.CPU.UsagePercent,
				RequestCores:  overview.CPU.RequestCores,
				LimitCores:    overview.CPU.LimitCores,
				ThrottledTime: overview.CPU.ThrottledTime,
			},
			Memory: types.MemoryUsageSnapshot{
				Timestamp:       overview.Memory.Timestamp.Unix(),
				UsageBytes:      overview.Memory.UsageBytes,
				UsagePercent:    overview.Memory.UsagePercent,
				RequestBytes:    overview.Memory.RequestBytes,
				LimitBytes:      overview.Memory.LimitBytes,
				WorkingSetBytes: overview.Memory.WorkingSetBytes,
				RSSBytes:        overview.Memory.RSSBytes,
				CacheBytes:      overview.Memory.CacheBytes,
			},
			Network: types.NetworkSnapshot{
				Timestamp:       overview.Network.Timestamp.Unix(),
				ReceiveBytes:    overview.Network.ReceiveBytes,
				TransmitBytes:   overview.Network.TransmitBytes,
				ReceivePackets:  overview.Network.ReceivePackets,
				TransmitPackets: overview.Network.TransmitPackets,
				ReceiveErrors:   overview.Network.ReceiveErrors,
				TransmitErrors:  overview.Network.TransmitErrors,
			},
			Disk: types.DiskSnapshot{
				Timestamp:  overview.Disk.Timestamp.Unix(),
				ReadBytes:  overview.Disk.ReadBytes,
				WriteBytes: overview.Disk.WriteBytes,
				ReadOps:    overview.Disk.ReadOps,
				WriteOps:   overview.Disk.WriteOps,
			},
			RestartCount: overview.RestartCount,
			Age: types.PodAgeMetrics{
				Namespace:     overview.Age.Namespace,
				PodName:       overview.Age.PodName,
				CreationTime:  overview.Age.CreationTime.Unix(),
				Age:           overview.Age.Age,
				AgeSeconds:    overview.Age.AgeSeconds,
				Uptime:        overview.Age.Uptime,
				UptimeSeconds: overview.Age.UptimeSeconds,
			},
			Labels:      overview.Labels,
			Annotations: overview.Annotations,
		}

		// 转换容器状态
		for _, cs := range overview.Status.ContainerStates {
			podOverview.Status.ContainerStates = append(podOverview.Status.ContainerStates, types.ContainerState{
				ContainerName: cs.ContainerName,
				Ready:         cs.Ready,
				RestartCount:  cs.RestartCount,
				State:         cs.State,
				Reason:        cs.Reason,
			})
		}

		resp.Data = append(resp.Data, podOverview)
	}

	l.Infof("获取 Pods 列表成功: Namespace=%s, PodCount=%d", req.Namespace, len(resp.Data))

	return resp, nil
}
