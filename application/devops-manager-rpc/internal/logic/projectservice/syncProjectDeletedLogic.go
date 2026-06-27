package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SyncProjectDeletedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSyncProjectDeletedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SyncProjectDeletedLogic {
	return &SyncProjectDeletedLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SyncProjectDeletedLogic) SyncProjectDeleted(in *pb.DevopsSyncProjectDeletedReq) (*pb.EmptyResp, error) {
	project, err := l.svcCtx.ProjectModel.FindOneByPortalUuid(l.ctx, in.PortalProjectUuid)
	if err != nil {
		l.Infof("DevOps 项目不存在，跳过删除同步，portalProjectUuid: %s", in.PortalProjectUuid)
		return &pb.EmptyResp{}, nil
	}

	projectId := project.ID.Hex()
	updatedBy := in.UpdatedBy
	if updatedBy == "" {
		updatedBy = "system"
	}

	// 软删除 devops_project
	if err := l.svcCtx.ProjectModel.DeleteSoft(l.ctx, projectId, updatedBy); err != nil {
		l.Errorf("软删除 DevOps 项目失败: %v", err)
		return nil, err
	}

	// 级联软删除 project_member
	if err := l.svcCtx.ProjectMemberModel.DeleteSoftByProject(l.ctx, projectId, updatedBy); err != nil {
		l.Errorf("级联删除项目成员失败: %v", err)
	}

	// 级联软删除 project_channel_binding
	if err := l.svcCtx.ProjectChannelModel.DeleteSoftByProject(l.ctx, projectId, updatedBy); err != nil {
		l.Errorf("级联删除渠道绑定失败: %v", err)
	}

	// 级联软删除 project_config
	if err := l.svcCtx.ProjectConfigModel.DeleteSoftByProject(l.ctx, projectId, updatedBy); err != nil {
		l.Errorf("级联删除项目配置失败: %v", err)
	}

	// 级联软删除 project_maven_config
	if err := l.svcCtx.ProjectMavenModel.DeleteSoftByProject(l.ctx, projectId, updatedBy); err != nil {
		l.Errorf("级联删除 Maven 配置失败: %v", err)
	}

	l.Infof("同步删除 DevOps 项目成功，portalProjectUuid: %s, projectId: %s", in.PortalProjectUuid, projectId)
	return &pb.EmptyResp{}, nil
}
