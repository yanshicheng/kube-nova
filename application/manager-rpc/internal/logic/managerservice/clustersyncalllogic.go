package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterSyncAllLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClusterSyncAllLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterSyncAllLogic {
	return &ClusterSyncAllLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ClusterSyncAllLogic) ClusterSyncAll(in *pb.ClusterSyncAllReq) (*pb.ClusterSyncAllResp, error) {
	go func() {
		err := l.svcCtx.SyncOperator.SyncAll(context.Background(), in.Operator, true)
		if err != nil {
			return
		}
	}()
	return &pb.ClusterSyncAllResp{}, nil
}
