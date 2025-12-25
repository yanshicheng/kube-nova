package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClusterDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterDeleteLogic {
	return &ClusterDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 删除集群
func (l *ClusterDeleteLogic) ClusterDelete(in *pb.DeleteClusterReq) (*pb.DeleteClusterResp, error) {
	// todo: add your logic here and delete this line

	return &pb.DeleteClusterResp{}, nil
}
