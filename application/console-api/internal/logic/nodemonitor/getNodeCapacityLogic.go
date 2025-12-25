package nodemonitor

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeCapacityLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取节点容量
func NewGetNodeCapacityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeCapacityLogic {
	return &GetNodeCapacityLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeCapacityLogic) GetNodeCapacity(req *types.GetNodeCapacityRequest) (resp *types.GetNodeCapacityResponse, err error) {
	client, err := l.svcCtx.PrometheusManager.Get(req.ClusterUuid)
	if err != nil {
		l.Errorf("获取 Prometheus 客户端失败: %v", err)
		return nil, err
	}

	node := client.Node()

	capacity, err := node.GetNodeCapacity(req.NodeName)
	if err != nil {
		l.Errorf("获取节点容量失败: Node=%s, Error=%v", req.NodeName, err)
		return nil, err
	}

	resp = &types.GetNodeCapacityResponse{
		Data: types.NodeResourceQuantity{
			CPUCores:         capacity.CPUCores,
			MemoryBytes:      capacity.MemoryBytes,
			Pods:             capacity.Pods,
			EphemeralStorage: capacity.EphemeralStorage,
		},
	}

	l.Infof("获取节点容量成功: Node=%s", req.NodeName)
	return resp, nil
}
