package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterResourceSyncLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClusterResourceSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterResourceSyncLogic {
	return &ClusterResourceSyncLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ClusterResourceSyncLogic) ClusterResourceSync(in *pb.ClusterResourceSyncReq) (*pb.ClusterResourceSyncResp, error) {
	if in.Id == 0 {
		return nil, errorx.Msg("集群资源ID不能为空")
	}

	clusterResource, err := l.svcCtx.OnecClusterResourceModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查找集群资源记录 [ID:%d] 失败: %v", in.Id, err)
		return nil, errorx.Msg("集群资源记录不存在或查询失败")
	}

	cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, clusterResource.ClusterUuid)
	if err != nil {
		l.Errorf("查找集群 [UUID:%s] 失败: %v", clusterResource.ClusterUuid, err)
		return nil, errorx.Msg("关联的集群不存在")
	}

	clusterUuid := cluster.Uuid
	clusterName := cluster.Name
	operator := in.Operator
	svcCtx := l.svcCtx

	go func() {
		ctx := context.Background()
		logger := logx.WithContext(ctx)

		logger.Infof("开始异步同步集群 [%s] 的资源信息", clusterName)

		// 查询该集群下的所有项目集群
		projectClusters, err := svcCtx.OnecProjectClusterModel.SearchNoPage(ctx, "created_at", false, "cluster_uuid = ?", clusterUuid)
		if err != nil {
			logger.Errorf("查询集群 [%s] 下的项目集群失败: %v", clusterName, err)
			return
		}

		var syncCount int
		// 同步每个项目集群的工作空间资源分配
		for _, projectCluster := range projectClusters {
			if err := svcCtx.OnecProjectModel.SyncAllProjectClusters(ctx, projectCluster.Id); err != nil {
				logger.Errorf("同步项目集群 [ID:%d] 的工作空间资源分配失败: %v", projectCluster.Id, err)
				continue
			}
			syncCount++
			logger.Debugf("成功同步项目集群 [ID:%d] 的工作空间资源分配", projectCluster.Id)
		}

		logger.Infof("集群 [%s] 项目集群同步完成，共同步 %d 个", clusterName, syncCount)

		// 同步集群资源统计
		if err := svcCtx.SyncOperator.SyncOneClusterAllResource(ctx, clusterUuid, operator, true); err != nil {
			logger.Errorf("同步集群 [%s] 的资源统计数据失败: %v", clusterName, err)
			return
		}

		logger.Infof("集群 [%s] 资源同步全部完成", clusterName)
	}()

	return &pb.ClusterResourceSyncResp{}, nil
}
