package node

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeAnnotationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetNodeAnnotationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeAnnotationsLogic {
	return &GetNodeAnnotationsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeAnnotationsLogic) GetNodeAnnotations(req *types.NodeIdRequest) (resp []types.NodeAnnotationItem, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.NodeAnnotationsList(l.ctx, &pb.ClusterNodeAnnotationsListReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取节点注解列表失败: %v", err)
		return nil, errorx.Msg("获取节点注解列表失败")
	}

	var items []types.NodeAnnotationItem
	for _, annotation := range rpcResp.Annotations {
		items = append(items, types.NodeAnnotationItem{
			Key:      annotation.Key,
			Value:    annotation.Value,
			IsDelete: annotation.IsDelete,
		})
	}

	return items, nil
}
