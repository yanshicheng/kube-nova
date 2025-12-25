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

type GetClusterNsDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetClusterNsDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterNsDetailLogic {
	return &GetClusterNsDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 通过 clusterUuid 和 namespace 获取详情信息
func (l *GetClusterNsDetailLogic) GetClusterNsDetail(in *pb.GetClusterNsDetailReq) (*pb.GetClusterNsDetailResp, error) {
	// 1. 参数校验
	if in.ClusterUuid == "" {
		return nil, errorx.Msg("clusterUuid 不能为空")
	}
	if in.Namespace == "" {
		return nil, errorx.Msg("namespace 不能为空")
	}

	// 2. 通过 clusterUuid 获取集群信息
	cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, in.ClusterUuid)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("集群不存在")
		}
		l.Logger.Errorf("查询集群失败: %v", err)
		return nil, errorx.Msg("查询集群失败")
	}

	// 3. 通过 cluster_uuid 和 namespace 查找工作空间
	workspaces, err := l.svcCtx.OnecProjectWorkspaceModel.SearchNoPage(
		l.ctx,
		"",
		true,
		"`cluster_uuid` = ? AND `namespace` = ?",
		in.ClusterUuid,
		in.Namespace,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("工作空间不存在")
		}
		l.Logger.Errorf("查询工作空间失败: %v", err)
		return nil, errorx.Msg("查询工作空间失败")
	}
	if len(workspaces) == 0 {
		return nil, errorx.Msg("工作空间不存在")
	}
	workspace := workspaces[0]

	// 4. 通过 project_cluster_id 获取项目集群关系
	projectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, workspace.ProjectClusterId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("项目集群关系不存在")
		}
		l.Logger.Errorf("查询项目集群关系失败: %v", err)
		return nil, errorx.Msg("查询项目集群关系失败")
	}

	// 5. 通过 project_id 获取项目信息
	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, projectCluster.ProjectId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("项目不存在")
		}
		l.Logger.Errorf("查询项目失败: %v", err)
		return nil, errorx.Msg("查询项目失败")
	}

	// 6. 组装返回结果
	return &pb.GetClusterNsDetailResp{
		ClusterId:       cluster.Id,
		ClusterUuid:     cluster.Uuid,
		ClusterName:     cluster.Name,
		ProjectId:       project.Id,
		ProjectNameCn:   project.Name,
		ProjectDetails:  project.Description,
		ProjectUuid:     project.Uuid,
		Namespace:       workspace.Namespace,
		WorkspaceId:     workspace.Id,
		WorkspaceNameCn: workspace.Name,
	}, nil
}
