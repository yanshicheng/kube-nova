package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectClusterGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectClusterGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectClusterGetByIdLogic {
	return &ProjectClusterGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectClusterGetById 根据ID获取项目集群配额
func (l *ProjectClusterGetByIdLogic) ProjectClusterGetById(in *pb.GetOnecProjectClusterByIdReq) (*pb.GetOnecProjectClusterByIdResp, error) {

	projectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("查询项目集群配额失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("项目集群配额不存在")
	}

	// 查询集群名称
	cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, projectCluster.ClusterUuid)
	clusterName := ""
	if err == nil {
		clusterName = cluster.Name
	}

	return &pb.GetOnecProjectClusterByIdResp{
		Data: convertProjectClusterToProto(projectCluster, clusterName),
	}, nil
}
