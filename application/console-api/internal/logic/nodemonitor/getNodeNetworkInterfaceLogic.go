package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeNetworkInterfaceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点指定网络接口指标
func NewGetNodeNetworkInterfaceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeNetworkInterfaceLogic {
	return &GetNodeNetworkInterfaceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeNetworkInterfaceLogic) GetNodeNetworkInterface(req *types.GetNodeNetworkInterfaceRequest) (resp *types.GetNodeNetworkInterfaceResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	iface, err := node.GetNodeNetworkInterface(req.NodeName, req.InterfaceName, timeRange)
	if err != nil {
		l.Errorf("获取节点网络接口指标失败: Node=%s, Interface=%s, Error=%v",
			req.NodeName, req.InterfaceName, err)
		return nil, err
	}

	resp = &types.GetNodeNetworkInterfaceResponse{
		Data: convertNodeNetworkInterfaceMetrics(iface),
	}

	l.Infof("获取节点网络接口指标成功: Node=%s, Interface=%s", req.NodeName, req.InterfaceName)
	return resp, nil
}
