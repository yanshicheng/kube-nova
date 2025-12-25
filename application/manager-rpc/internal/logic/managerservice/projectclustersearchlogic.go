package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectClusterSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectClusterSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectClusterSearchLogic {
	return &ProjectClusterSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectClusterSearch 搜索项目集群配额
// ProjectClusterSearch 搜索项目集群配额列表
func (l *ProjectClusterSearchLogic) ProjectClusterSearch(in *pb.SearchOnecProjectClusterReq) (*pb.SearchOnecProjectClusterResp, error) {

	if in.ProjectId == 0 {
		l.Logger.Errorf("项目ID不能为空")
		return nil, errorx.Msg("项目ID不能为空")
	}

	// 构建查询条件
	queryStr := "project_id = ?"
	args := []interface{}{in.ProjectId}

	if in.ClusterUuid != "" {
		queryStr += " AND cluster_uuid = ?"
		args = append(args, in.ClusterUuid)
	}

	// 执行搜索
	projectClusters, err := l.svcCtx.OnecProjectClusterModel.SearchNoPage(l.ctx, "", true, queryStr, args...)
	if err != nil {
		l.Logger.Errorf("搜索项目集群配额失败，错误: %v", err)
		return nil, errorx.Msg("搜索项目集群配额失败")
	}

	// 转换结果
	var data []*pb.OnecProjectCluster
	for _, pc := range projectClusters {
		// 查询集群名称
		cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, pc.ClusterUuid)
		clusterName := ""
		if err == nil {
			clusterName = cluster.Name
		}
		data = append(data, convertProjectClusterToProto(pc, clusterName))
	}

	l.Logger.Infof("搜索项目集群配额成功，数量: %d", len(data))
	return &pb.SearchOnecProjectClusterResp{
		Data: data,
	}, nil
}
