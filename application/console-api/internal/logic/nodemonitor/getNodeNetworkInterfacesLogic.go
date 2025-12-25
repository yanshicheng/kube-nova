package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeNetworkInterfacesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点网络接口列表
func NewGetNodeNetworkInterfacesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeNetworkInterfacesLogic {
	return &GetNodeNetworkInterfacesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeNetworkInterfacesLogic) GetNodeNetworkInterfaces(req *types.GetNodeNetworkInterfacesRequest) (resp *types.GetNodeNetworkInterfacesResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	interfaces, err := node.GetNodeNetworkInterfaces(req.NodeName, timeRange)
	if err != nil {
		l.Errorf("获取节点网络接口列表失败: Node=%s, Error=%v", req.NodeName, err)
		return nil, err
	}

	resp = &types.GetNodeNetworkInterfacesResponse{}
	for _, iface := range interfaces {
		resp.Data = append(resp.Data, convertNodeNetworkInterfaceMetrics(&iface))
	}

	l.Infof("获取节点网络接口列表成功: Node=%s, Count=%d", req.NodeName, len(interfaces))
	return resp, nil
}
