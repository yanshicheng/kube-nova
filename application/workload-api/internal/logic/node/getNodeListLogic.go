package node

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetNodeListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeListLogic {
	return &GetNodeListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeListLogic) GetNodeList(req *types.SearchClusterNodeRequest) (resp *types.SearchClusterNodeResponse, err error) {

	// 2. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 3. 获取节点列表
	nodeOperator := client.Node()
	nodeList, err := nodeOperator.List(&types2.NodeListRequest{
		Page:       req.Page,
		PageSize:   req.PageSize,
		IsAsc:      req.IsAsc,
		OrderField: req.OrderField,
	})
	if err != nil {
		l.Errorf("获取节点列表失败: %v", err)
		return nil, fmt.Errorf("获取节点列表失败")
	}

	// 4. 转换为响应格式
	items := make([]types.ClusterNodeInfo, 0, len(nodeList.Items))
	for _, node := range nodeList.Items {
		items = append(items, types.ClusterNodeInfo{
			ClusterUuid:   req.ClusterUuid,
			NodeName:      node.NodeName,
			NodeIp:        node.NodeIp,
			NodeStatus:    node.Status,
			CpuUsge:       0, // 需要从监控系统获取
			MemoryUsge:    0, // 需要从监控系统获取
			PodTotal:      node.Pods,
			PodUsge:       node.PodsUsage,
			CreatedAt:     node.JoinAt,
			NodeRole:      node.Roles,
			Architecture:  node.Architecture,
			Unschedulable: node.Unschedulable,
		})
	}

	// 5. 返回结果（实际应用中可能需要分页处理）
	resp = &types.SearchClusterNodeResponse{
		Items: items,
		Total: nodeList.Total,
	}

	return resp, nil
}
