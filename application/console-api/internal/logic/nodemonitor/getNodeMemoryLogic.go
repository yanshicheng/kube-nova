package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/logic/utils"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeMemoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点内存指标
func NewGetNodeMemoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeMemoryLogic {
	return &GetNodeMemoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeMemoryLogic) GetNodeMemory(req *types.GetNodeMemoryRequest) (resp *types.GetNodeMemoryResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()
	timeRange := utils.ParseTimeRange(req.Start, req.End, "")

	memory, err := node.GetNodeMemory(req.NodeName, timeRange)
	if err != nil {
		l.Errorf("获取节点内存指标失败: Node=%s, Error=%v", req.NodeName, err)
		return nil, err
	}

	resp = &types.GetNodeMemoryResponse{
		Data: convertNodeMemoryMetrics(memory),
	}

	l.Infof("获取节点内存指标成功: Node=%s", req.NodeName)
	return resp, nil
}
