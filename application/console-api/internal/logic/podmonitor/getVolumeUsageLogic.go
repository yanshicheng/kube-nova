package podmonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetVolumeUsageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取存储使用情况
func NewGetVolumeUsageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetVolumeUsageLogic {
	return &GetVolumeUsageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetVolumeUsageLogic) GetVolumeUsage(req *types.GetVolumeUsageRequest) (resp *types.GetVolumeUsageResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	pod := client.Pod()

	metrics, err := pod.GetVolumeUsage(req.Namespace, req.PodName)
	if err != nil {
		l.Errorf("获取存储使用情况失败: Namespace=%s, Pod=%s, Error=%v",
			req.Namespace, req.PodName, err)
		return nil, err
	}

	resp = &types.GetVolumeUsageResponse{
		Data: types.PodVolumeMetrics{
			Namespace:     metrics.Namespace,
			PodName:       metrics.PodName,
			Volumes:       make([]types.VolumeUsage, 0),
			TotalCapacity: metrics.TotalCapacity,
			TotalUsed:     metrics.TotalUsed,
			UsagePercent:  metrics.UsagePercent,
		},
	}

	for _, volume := range metrics.Volumes {
		resp.Data.Volumes = append(resp.Data.Volumes, types.VolumeUsage{
			VolumeName:    volume.VolumeName,
			MountPath:     volume.MountPath,
			Device:        volume.Device,
			CapacityBytes: volume.CapacityBytes,
			UsedBytes:     volume.UsedBytes,
			AvailBytes:    volume.AvailBytes,
			UsagePercent:  volume.UsagePercent,
			PVCName:       volume.PVCName,
			StorageClass:  volume.StorageClass,
			AccessMode:    volume.AccessMode,
		})
	}

	l.Infof("获取存储使用情况成功: Pod=%s, VolumeCount=%d, UsagePercent=%.2f%%",
		req.PodName, len(resp.Data.Volumes), metrics.UsagePercent)
	return resp, nil
}
