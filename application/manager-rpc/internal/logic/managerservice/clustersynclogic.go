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
	cluster, err := l.svcCtx.OnecClusterModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("集群不存在 [id=%d]", in.Id)
			return nil, errorx.Msg("集群不存在")
		}
		l.Errorf("查询集群失败: %v", err)
		return nil, errorx.Msg("查询集群失败")
	}

	clusterUuid := cluster.Uuid
	updatedBy := in.UpdatedBy
	svcCtx := l.svcCtx

	go func() {
		ctx := context.Background()
		logger := logx.WithContext(ctx)

		logger.Infof("开始异步同步集群 [uuid=%s]", clusterUuid)

		err := svcCtx.SyncOperator.SyncOneCLuster(ctx, clusterUuid, updatedBy, true)
		if err != nil {
			logger.Errorf("同步集群失败 [uuid=%s]: %v", clusterUuid, err)
			return
		}

		logger.Infof("集群同步完成 [uuid=%s]", clusterUuid)
	}()

	return &pb.SyncClusterResp{}, nil
}
