package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeDiskDevicesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点磁盘设备列表
func NewGetNodeDiskDevicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeDiskDevicesLogic {
	return &GetNodeDiskDevicesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeDiskDevicesLogic) GetNodeDiskDevices(req *types.GetNodeDiskDevicesRequest) (resp *types.GetNodeDiskDevicesResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	devices, err := node.GetNodeDiskDevices(req.NodeName, timeRange)
	if err != nil {
		l.Errorf("获取节点磁盘设备列表失败: Node=%s, Error=%v", req.NodeName, err)
		return nil, err
	}

	resp = &types.GetNodeDiskDevicesResponse{}
	for _, dev := range devices {
		resp.Data = append(resp.Data, convertNodeDiskDeviceMetrics(&dev))
	}

	l.Infof("获取节点磁盘设备列表成功: Node=%s, Count=%d", req.NodeName, len(devices))
	return resp, nil
}
