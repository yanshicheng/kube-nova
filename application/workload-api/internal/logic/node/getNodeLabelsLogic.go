package node

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeLabelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetNodeLabelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeLabelsLogic {
	return &GetNodeLabelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeLabelsLogic) GetNodeLabels(req *types.DefaultNodeNameRequest) (resp []types.NodeLabelItem, err error) {

	// 2. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 3. 获取节点标签
	nodeOperator := client.Node()
	labels, err := nodeOperator.GetLabels(req.NodeName)
	if err != nil {
		l.Errorf("获取节点标签失败: %v", err)
		return nil, fmt.Errorf("获取节点标签失败")
	}

	// 4. 转换为响应格式，并标记系统标签不可删除
	resp = make([]types.NodeLabelItem, 0, len(labels))
	for _, label := range labels {
		resp = append(resp, types.NodeLabelItem{
			Key:      label.Key,
			Value:    label.Value,
			IsDelete: label.IsDelete,
		})
	}

	return resp, nil
}
