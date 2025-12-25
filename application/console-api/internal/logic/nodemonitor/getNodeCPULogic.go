package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeCPULogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点 CPU 指标
func NewGetNodeCPULogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeCPULogic {
	return &GetNodeCPULogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeCPULogic) GetNodeCPU(req *types.GetNodeCPURequest) (resp *types.GetNodeCPUResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	cpu, err := node.GetNodeCPU(req.NodeName, timeRange)
	if err != nil {
		l.Errorf("获取节点 CPU 指标失败: Node=%s, Error=%v", req.NodeName, err)
		return nil, err
	}

	resp = &types.GetNodeCPUResponse{
		Data: convertNodeCPUMetrics(cpu),
	}

	l.Infof("获取节点 CPU 指标成功: Node=%s", req.NodeName)
	return resp, nil
}
