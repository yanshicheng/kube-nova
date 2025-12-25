package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeAllocatedLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点已分配资源
func NewGetNodeAllocatedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeAllocatedLogic {
	return &GetNodeAllocatedLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeAllocatedLogic) GetNodeAllocated(req *types.GetNodeAllocatedRequest) (resp *types.GetNodeAllocatedResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()

	allocated, err := node.GetNodeAllocated(req.NodeName)
	if err != nil {
		l.Errorf("获取节点已分配资源失败: Node=%s, Error=%v", req.NodeName, err)
		return nil, err
	}

	resp = &types.GetNodeAllocatedResponse{
		Data: types.NodeResourceQuantity{
			CPUCores:         allocated.CPUCores,
			MemoryBytes:      allocated.MemoryBytes,
			Pods:             allocated.Pods,
			EphemeralStorage: allocated.EphemeralStorage,
		},
	}

	l.Infof("获取节点已分配资源成功: Node=%s", req.NodeName)
	return resp, nil
}
