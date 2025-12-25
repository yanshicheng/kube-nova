package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectWorkspaceSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectWorkspaceSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectWorkspaceSearchLogic {
	return &ProjectWorkspaceSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectWorkspaceSearch 搜索工作空间
// ProjectWorkspaceSearch 搜索工作空间列表
func (l *ProjectWorkspaceSearchLogic) ProjectWorkspaceSearch(in *pb.SearchOnecProjectWorkspaceReq) (*pb.SearchOnecProjectWorkspaceResp, error) {

	if in.ProjectClusterId == 0 {
		l.Errorf("项目集群ID不能为空")
		return nil, errorx.Msg("项目集群ID不能为空")
	}

	// 构建查询条件
	queryStr := "project_cluster_id = ?"
	args := []interface{}{in.ProjectClusterId}

	if in.Name != "" {
		queryStr += " AND name LIKE ?"
		args = append(args, "%"+in.Name+"%")
	}

	if in.Namespace != "" {
		queryStr += " AND namespace = ?"
		args = append(args, in.Namespace)
	}

	// 执行搜索
	workspaces, err := l.svcCtx.OnecProjectWorkspaceModel.SearchNoPage(l.ctx, "", true, queryStr, args...)
	if err != nil {
		l.Errorf("搜索工作空间失败，错误: %v", err)
		return nil, errorx.Msg("搜索工作空间失败")
	}

	// 转换结果
	var data []*pb.OnecProjectWorkspace
	for _, ws := range workspaces {
		// 查询集群名称
		cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, ws.ClusterUuid)
		clusterName := ""
		if err == nil {
			clusterName = cluster.Name
		}
		data = append(data, convertWorkspaceToProto(ws, clusterName))
	}

	return &pb.SearchOnecProjectWorkspaceResp{
		Data: data,
	}, nil
}
