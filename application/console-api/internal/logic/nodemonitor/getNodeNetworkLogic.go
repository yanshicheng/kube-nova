package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeNetworkLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点网络指标
func NewGetNodeNetworkLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeNetworkLogic {
	return &GetNodeNetworkLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeNetworkLogic) GetNodeNetwork(req *types.GetNodeNetworkRequest) (resp *types.GetNodeNetworkResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	network, err := node.GetNodeNetwork(req.NodeName, timeRange)
	if err != nil {
		l.Errorf("获取节点网络指标失败: Node=%s, Error=%v", req.NodeName, err)
		return nil, err
	}

	resp = &types.GetNodeNetworkResponse{
		Data: convertNodeNetworkMetrics(network),
	}

	l.Infof("获取节点网络指标成功: Node=%s", req.NodeName)
	return resp, nil
}
