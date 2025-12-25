package node

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetNodeDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeDetailLogic {
	return &GetNodeDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeDetailLogic) GetNodeDetail(req *types.DefaultNodeNameRequest) (resp *types.ClusterNodeDetail, err error) {

	// 2. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 3. 获取节点详情 - 使用新的 GetNodeDetail 方法
	nodeOperator := client.Node()
	nodeInfo, err := nodeOperator.GetNodeDetail(req.NodeName)
	if err != nil {
		l.Errorf("获取节点详情失败: %v", err)
		return nil, fmt.Errorf("获取节点详情失败")
	}

	// 4. 转换为响应格式
	resp = &types.ClusterNodeDetail{
		NodeName:        nodeInfo.NodeName,
		ClusterUuid:     req.ClusterUuid,
		NodeUuid:        nodeInfo.NodeUuid,
		Name:            nodeInfo.NodeName,
		Hostname:        nodeInfo.HostName,
		Roles:           nodeInfo.Roles,
		OsImage:         nodeInfo.OsImage,
		NodeIp:          nodeInfo.NodeIp,
		KernelVersion:   nodeInfo.KernelVersion,
		OperatingSystem: nodeInfo.OperatingSystem,
		Architecture:    nodeInfo.Architecture,
		Cpu:             nodeInfo.Cpu,
		Memory:          nodeInfo.Memory,
		Pods:            nodeInfo.Pods,
		IsGpu:           nodeInfo.IsGpu,
		Runtime:         nodeInfo.Runtime,
		JoinAt:          nodeInfo.JoinAt,
		Unschedulable:   nodeInfo.Unschedulable,
		KubeletVersion:  nodeInfo.KubeletVersion,
		Status:          nodeInfo.Status,
		PodCidr:         nodeInfo.PodCidr,
		PodCidrs:        nodeInfo.PodCidrs,
	}

	return resp, nil
}
