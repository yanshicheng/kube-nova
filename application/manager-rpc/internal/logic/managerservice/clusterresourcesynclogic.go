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

// ClusterResourceSync 同步指定集群资源的统计信息
func (l *ClusterResourceSyncLogic) ClusterResourceSync(in *pb.ClusterResourceSyncReq) (*pb.ClusterResourceSyncResp, error) {
	if in.Id == 0 {
		return nil, errorx.Msg("集群资源ID不能为空")
	}

	l.Infof("开始同步集群资源 [ID:%d] 的统计信息", in.Id)

	// 验证集群资源记录是否存在
	clusterResource, err := l.svcCtx.OnecClusterResourceModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查找集群资源记录 [ID:%d] 失败: %v", in.Id, err)
		return nil, errorx.Msg("集群资源记录不存在或查询失败")
	}

	// 查找对应的集群信息
	cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, clusterResource.ClusterUuid)
	if err != nil {
		l.Errorf("查找集群 [UUID:%s] 失败: %v", clusterResource.ClusterUuid, err)
		return nil, errorx.Msg("关联的集群不存在")
	}

	// 查询该集群下的所有项目集群
	projectClusters, err := l.svcCtx.OnecProjectClusterModel.SearchNoPage(l.ctx, "created_at", false, "cluster_uuid = ?", cluster.Uuid)
	if err != nil {
		l.Errorf("查询集群 [%s] 下的项目集群失败: %v", cluster.Name, err)
		return nil, errorx.Msg("查询项目集群失败")
	}

	var syncCount int

	// 同步每个项目集群的工作空间资源分配
	for _, projectCluster := range projectClusters {
		if err := l.svcCtx.OnecProjectModel.SyncAllProjectClusters(l.ctx, projectCluster.Id); err != nil {
			l.Errorf("同步项目集群 [ID:%d] 的工作空间资源分配失败: %v", projectCluster.Id, err)
			continue
		}
		syncCount++
		l.Debugf("成功同步项目集群 [ID:%d] 的工作空间资源分配", projectCluster.Id)
	}

	l.Infof("集群 [%s] 资源同步完成，共同步 %d 个项目集群", cluster.Name, syncCount)
	l.Infof("找到集群 [%s:%s]，开始同步资源统计", cluster.Name, cluster.Uuid)
	// 同步集群信息
	if err := l.svcCtx.SyncOperator.SyncOneClusterAllResource(l.ctx, cluster.Uuid, in.Operator, true); err != nil {
		l.Errorf("同步集群 [%s] 的资源统计数据失败: %v", cluster.Name, err)
		return nil, errorx.Msg("同步集群资源统计失败")
	}

	l.Infof("成功同步集群 [%s] 的项目集群统计数据", cluster.Name)

	return &pb.ClusterResourceSyncResp{}, nil
}
