package node

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
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

func (l *GetNodeLabelsLogic) GetNodeLabels(req *types.NodeIdRequest) (resp []types.NodeLabelItem, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.NodeLabelsList(l.ctx, &pb.ClusterNodeLabelsListReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取节点标签列表失败: %v", err)
		return nil, fmt.Errorf("获取节点标签列表失败")
	}

	var items []types.NodeLabelItem
	for _, label := range rpcResp.Labels {
		items = append(items, types.NodeLabelItem{
			Key:      label.Key,
			Value:    label.Value,
			IsDelete: label.IsDelete,
		})
	}

	return items, nil
}
