package node

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeTaintsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetNodeTaintsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeTaintsLogic {
	return &GetNodeTaintsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeTaintsLogic) GetNodeTaints(req *types.NodeIdRequest) (resp []types.NodeTaint, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.NodeTaintList(l.ctx, &pb.ClusterNodeTaintsListReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取节点污点列表失败: %v", err)
		return nil, errorx.Msg("获取节点污点列表失败")
	}

	var items []types.NodeTaint
	for _, taint := range rpcResp.Taints {
		items = append(items, types.NodeTaint{
			Key:      taint.Key,
			Value:    taint.Value,
			Effect:   taint.Effect,
			Time:     taint.Time,
			IsDelete: taint.IsDelete,
		})
	}

	return items, nil
}
