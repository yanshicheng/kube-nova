package logservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveLogScopeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveLogScopeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveLogScopeLogic {
	return &ResolveLogScopeLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ResolveLogScopeLogic) ResolveLogScope(in *pb.ResolveLogScopeReq) (*pb.ResolveLogScopeResp, error) {
	if in.VersionId > 0 {
		return l.resolveByVersionID(in.VersionId)
	}
	if in.WorkspaceId > 0 {
		return l.resolveByWorkspace(in)
	}
	if in.ProjectUuid == "" && in.Namespace == "" {
		if in.ClusterUuid == "" {
			return nil, errorx.Msg("clusterUuid 不能为空")
		}
		if in.Application != "" || in.ResourceName != "" {
			return nil, errorx.Msg("全局规则不能直接选择服务或版本")
		}
		return &pb.ResolveLogScopeResp{ClusterUuid: in.ClusterUuid}, nil
	}
	if in.ProjectUuid == "" || in.Namespace == "" {
		return nil, errorx.Msg("projectUuid 和 namespace 不能为空")
	}
	project, err := l.svcCtx.OnecProjectModel.FindOneByUuid(l.ctx, in.ProjectUuid)
	if err != nil {
		return nil, errorx.Msg("项目不存在")
	}
	if in.ClusterUuid == "" {
		return nil, errorx.Msg("projectUuid + namespace 不能唯一定位时必须传入 clusterUuid 或 workspaceId")
	}
	projectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOneByClusterUuidProjectId(l.ctx, in.ClusterUuid, project.Id)
	if err != nil {
		return nil, errorx.Msg("项目未绑定该集群")
	}
	workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOneByProjectClusterIdNamespace(l.ctx, projectCluster.Id, in.Namespace)
	if err != nil {
		return nil, errorx.Msg("工作空间不存在")
	}
	resp := &pb.ResolveLogScopeResp{ClusterUuid: workspace.ClusterUuid, ProjectUuid: project.Uuid, ProjectId: project.Id, Namespace: workspace.Namespace, WorkspaceId: workspace.Id, WorkspaceName: workspace.Name}
	if in.Application == "" {
		return resp, nil
	}
	app, err := findApplicationByName(l.ctx, l.svcCtx, workspace.Id, in.Application)
	if err != nil {
		return nil, err
	}
	resp.ApplicationId = app.Id
	resp.ApplicationName = app.NameEn
	resp.ResourceType = app.ResourceType
	if in.ResourceName == "" {
		return resp, nil
	}
	version, err := l.svcCtx.OnecProjectVersion.FindOneByApplicationIdResourceName(l.ctx, app.Id, in.ResourceName)
	if err != nil {
		return nil, errorx.Msg("版本资源不存在")
	}
	resp.VersionId = version.Id
	resp.ResourceName = version.ResourceName
	return resp, nil
}

func (l *ResolveLogScopeLogic) resolveByVersionID(versionID uint64) (*pb.ResolveLogScopeResp, error) {
	detail, err := l.svcCtx.OnecProjectVersion.GetVersionDetailInfo(l.ctx, versionID)
	if err != nil {
		return nil, errorx.Msg("版本不存在")
	}
	projectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, detail.ResourceClusterId)
	if err != nil {
		return nil, errorx.Msg("项目集群不存在")
	}
	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, projectCluster.ProjectId)
	if err != nil {
		return nil, errorx.Msg("项目不存在")
	}
	workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, detail.WorkspaceId)
	if err != nil {
		return nil, errorx.Msg("工作空间不存在")
	}
	app, err := l.svcCtx.OnecProjectApplication.FindOne(l.ctx, detail.ApplicationId)
	if err != nil {
		return nil, errorx.Msg("应用不存在")
	}
	return &pb.ResolveLogScopeResp{ClusterUuid: detail.ClusterUuid, ProjectUuid: project.Uuid, ProjectId: project.Id, Namespace: detail.Namespace, WorkspaceId: workspace.Id, WorkspaceName: workspace.Name, ApplicationId: app.Id, ApplicationName: app.NameEn, ResourceType: detail.ResourceType, VersionId: versionID, ResourceName: detail.ResourceName}, nil
}

func (l *ResolveLogScopeLogic) resolveByWorkspace(in *pb.ResolveLogScopeReq) (*pb.ResolveLogScopeResp, error) {
	workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, in.WorkspaceId)
	if err != nil {
		return nil, errorx.Msg("工作空间不存在")
	}
	projectCluster, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, workspace.ProjectClusterId)
	if err != nil {
		return nil, errorx.Msg("项目集群不存在")
	}
	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, projectCluster.ProjectId)
	if err != nil {
		return nil, errorx.Msg("项目不存在")
	}
	resp := &pb.ResolveLogScopeResp{ClusterUuid: workspace.ClusterUuid, ProjectUuid: project.Uuid, ProjectId: project.Id, Namespace: workspace.Namespace, WorkspaceId: workspace.Id, WorkspaceName: workspace.Name}
	if in.Application == "" {
		return resp, nil
	}
	app, err := findApplicationByName(l.ctx, l.svcCtx, workspace.Id, in.Application)
	if err != nil {
		return nil, err
	}
	resp.ApplicationId = app.Id
	resp.ApplicationName = app.NameEn
	resp.ResourceType = app.ResourceType
	if in.ResourceName == "" {
		return resp, nil
	}
	version, err := l.svcCtx.OnecProjectVersion.FindOneByApplicationIdResourceName(l.ctx, app.Id, in.ResourceName)
	if err != nil {
		return nil, errorx.Msg("版本资源不存在")
	}
	resp.VersionId = version.Id
	resp.ResourceName = version.ResourceName
	return resp, nil
}
