package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeAllocatableLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点可分配资源
func NewGetNodeAllocatableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeAllocatableLogic {
	return &GetNodeAllocatableLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeAllocatableLogic) GetNodeAllocatable(req *types.GetNodeAllocatableRequest) (resp *types.GetNodeAllocatableResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()

	allocatable, err := node.GetNodeAllocatable(req.NodeName)
	if err != nil {
		l.Errorf("获取节点可分配资源失败: Node=%s, Error=%v", req.NodeName, err)
		return nil, err
	}

	resp = &types.GetNodeAllocatableResponse{
		Data: types.NodeResourceQuantity{
			CPUCores:         allocatable.CPUCores,
			MemoryBytes:      allocatable.MemoryBytes,
			Pods:             allocatable.Pods,
			EphemeralStorage: allocatable.EphemeralStorage,
		},
	}

	l.Infof("获取节点可分配资源成功: Node=%s", req.NodeName)
	return resp, nil
}
