package managerservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterSyncLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClusterSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterSyncLogic {
	return &ClusterSyncLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ClusterSyncLogic) ClusterSync(in *pb.SyncClusterReq) (*pb.SyncClusterResp, error) {

	// 1. 查询集群信息
	cluster, err := l.svcCtx.OnecClusterModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("集群不存在 [id=%d]", in.Id)
			return nil, errorx.Msg("集群不存在")
		}
		l.Errorf("查询集群失败: %v", err)
		return nil, errorx.Msg("查询集群失败")
	}
	err = l.svcCtx.SyncOperator.SyncOneCLuster(l.ctx, cluster.Uuid, in.UpdatedBy, true)
	if err != nil {
		l.Errorf("同步集群失败: %v", err)
		return nil, errorx.Msg("同步集群失败")
	}
	return &pb.SyncClusterResp{}, nil
}
