package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type VersionDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVersionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VersionDetailLogic {
	return &VersionDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取版本detailinfo
func (l *VersionDetailLogic) VersionDetail(in *pb.GetOnecProjectVersionDetailReq) (*pb.GetOnecProjectVersionDetailResp, error) {
	// 查询版本是否存在
	version, err := l.svcCtx.OnecProjectVersion.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查询版本失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("查询版本失败")
	}

	// 通过 version 查询 application
	application, err := l.svcCtx.OnecProjectApplication.FindOne(l.ctx, version.ApplicationId)
	if err != nil {
		l.Errorf("查询应用失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("查询应用失败")
	}

	// 通过 application 查询  workspace
	workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, application.WorkspaceId)
	if err != nil {
		l.Errorf("查询工作空间失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("查询工作空间失败")
	}
	// 通过 workspace 获取 cluster 和
	cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, workspace.ClusterUuid)
	if err != nil {
		l.Errorf("查询集群失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("查询集群失败")
	}

	// 查询项目
	// 查询项目集群关联
	projectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, workspace.ProjectClusterId)
	if err != nil {
		l.Errorf("查询项目集群关联失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("查询项目集群关联失败")
	}
	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, projectCluster.ProjectId)
	if err != nil {
		l.Errorf("查询项目失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("查询项目失败")
	}

	// 将 Labels 字符串转换为 map
	return &pb.GetOnecProjectVersionDetailResp{
		ClusterUuid:       cluster.Uuid,
		ClusterName:       cluster.Name,
		ResourceClusterId: projectCluster.Id,
		ProjectUuid:       project.Uuid,
		ProjectDetails:    project.Description,
		ProjectNameCn:     project.Name,
		WorkspaceNameCn:   workspace.Name,
		WorkspaceId:       workspace.Id,
		Namespace:         workspace.Namespace,
		Version:           version.Version,
		VersionId:         version.Id,
		ResourceName:      version.ResourceName,
		ResourceType:      application.ResourceType,
		ApplicationId:     application.Id,
		ApplicationNameCn: application.NameCn,
		ApplicationNameEn: application.NameEn,
	}, nil
}
