package portalprojectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

const (
	devopsPlatformCode = "devops"
	portalPlatformCode = "portal"
)

func projectToPb(project *model.OnecProject) *pb.PortalProject {
	if project == nil {
		return nil
	}
	return &pb.PortalProject{
		Id:          project.Id,
		Name:        project.Name,
		Uuid:        project.Uuid,
		IsSystem:    project.IsSystem,
		Description: project.Description,
		CreatedBy:   project.CreatedBy,
		UpdatedBy:   project.UpdatedBy,
		CreatedAt:   project.CreatedAt.Unix(),
		UpdatedAt:   project.UpdatedAt.Unix(),
	}
}

func projectsToPb(projects []*model.OnecProject) []*pb.PortalProject {
	items := make([]*pb.PortalProject, 0, len(projects))
	for _, project := range projects {
		items = append(items, projectToPb(project))
	}
	return items
}

func isDevopsPlatform(platform *model.SysPlatform) bool {
	return platform != nil && strings.EqualFold(platform.PlatformCode, devopsPlatformCode)
}

func isPortalPlatform(platform *model.SysPlatform) bool {
	return platform != nil && strings.EqualFold(platform.PlatformCode, portalPlatformCode)
}

func getPortalPlatform(ctx context.Context, svcCtx *svc.ServiceContext) (*model.SysPlatform, error) {
	platform, err := svcCtx.SysPlatformModel.FindOneByPlatformCode(ctx, portalPlatformCode)
	if err != nil || platform == nil || platform.IsDeleted != 0 {
		return nil, errorx.Msg("门户平台不存在")
	}
	if platform.IsEnable != 1 {
		return nil, errorx.Msg("门户平台不可用")
	}
	return platform, nil
}

func ensureProjectPortalPlatform(ctx context.Context, svcCtx *svc.ServiceContext, projectId uint64, operator string) (*model.SysPlatform, error) {
	portalPlatform, err := getPortalPlatform(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	if err := svcCtx.ProjectPlatformBindingModel.BindProjectPlatform(ctx, projectId, portalPlatform.Id, operator); err != nil {
		return nil, err
	}
	if err := svcCtx.ProjectMemberPlatformRole.GrantPlatformToProjectMembers(ctx, projectId, portalPlatform.Id, operator); err != nil {
		return nil, err
	}
	return portalPlatform, nil
}

func isProjectBoundToDevops(ctx context.Context, svcCtx *svc.ServiceContext, projectId uint64) bool {
	platform, err := svcCtx.SysPlatformModel.FindOneByPlatformCode(ctx, devopsPlatformCode)
	if err != nil || platform.IsDeleted != 0 {
		return false
	}
	bound, err := svcCtx.ProjectPlatformBindingModel.IsProjectBoundToPlatform(ctx, projectId, platform.Id)
	return err == nil && bound
}

func syncDevopsProjectInfo(ctx context.Context, svcCtx *svc.ServiceContext, project *model.OnecProject, updatedBy string) error {
	if project == nil || project.Uuid == "" || svcCtx.DevopsManagerRpc == nil {
		return nil
	}
	payload, err := model.MarshalProjectPlatformSyncTaskPayload(&model.ProjectPlatformSyncTaskPayload{
		ProjectId:         project.Id,
		PortalProjectUuid: project.Uuid,
		Name:              project.Name,
		Description:       project.Description,
		UpdatedBy:         updatedBy,
	})
	if err != nil {
		return err
	}
	task := &model.ProjectPlatformSyncTask{
		ProjectId:         project.Id,
		PortalProjectUuid: project.Uuid,
		PlatformCode:      devopsPlatformCode,
		Action:            devopsSyncActionProjectInfo,
		Payload:           payload,
		Status:            model.ProjectPlatformSyncTaskStatusPending,
		CreatedBy:         updatedBy,
		UpdatedBy:         updatedBy,
	}
	return enqueueAndRunDevopsSyncTask(ctx, svcCtx, task)
}

func syncDevopsProjectDeleted(ctx context.Context, svcCtx *svc.ServiceContext, project *model.OnecProject, updatedBy string) error {
	if project == nil || project.Uuid == "" || svcCtx.DevopsManagerRpc == nil {
		return nil
	}
	if updatedBy == "" {
		updatedBy = "system"
	}
	payload, err := model.MarshalProjectPlatformSyncTaskPayload(&model.ProjectPlatformSyncTaskPayload{
		ProjectId:         project.Id,
		PortalProjectUuid: project.Uuid,
		UpdatedBy:         updatedBy,
	})
	if err != nil {
		return err
	}
	task := &model.ProjectPlatformSyncTask{
		ProjectId:         project.Id,
		PortalProjectUuid: project.Uuid,
		PlatformCode:      devopsPlatformCode,
		Action:            devopsSyncActionProjectDelete,
		Payload:           payload,
		Status:            model.ProjectPlatformSyncTaskStatusPending,
		CreatedBy:         updatedBy,
		UpdatedBy:         updatedBy,
	}
	return enqueueAndRunDevopsSyncTask(ctx, svcCtx, task)
}

func syncDevopsProjectMembers(ctx context.Context, svcCtx *svc.ServiceContext, project *model.OnecProject, updatedBy string) error {
	if project == nil || project.Uuid == "" || svcCtx.DevopsManagerRpc == nil {
		return nil
	}
	if updatedBy == "" {
		updatedBy = "system"
	}
	devopsPlatform, err := svcCtx.SysPlatformModel.FindOneByPlatformCode(ctx, devopsPlatformCode)
	if err != nil || devopsPlatform == nil || devopsPlatform.IsDeleted != 0 {
		return nil
	}
	members, err := svcCtx.ProjectMemberPlatformRole.ListMembersByProjectAndPlatform(ctx, project.Id, devopsPlatform.Id)
	if err != nil {
		return err
	}
	items := make([]model.ProjectSyncTaskMember, 0, len(members))
	for _, member := range members {
		if member == nil {
			continue
		}
		items = append(items, model.ProjectSyncTaskMember{
			UserId:   member.UserId,
			Username: member.Username,
			Nickname: member.Nickname,
			Role:     member.Role,
		})
	}
	payload, err := model.MarshalProjectPlatformSyncTaskPayload(&model.ProjectPlatformSyncTaskPayload{
		ProjectId:         project.Id,
		PortalProjectUuid: project.Uuid,
		Members:           items,
		UpdatedBy:         updatedBy,
	})
	if err != nil {
		return err
	}
	task := &model.ProjectPlatformSyncTask{
		ProjectId:         project.Id,
		PortalProjectUuid: project.Uuid,
		PlatformCode:      devopsPlatformCode,
		Action:            devopsSyncActionProjectMembers,
		Payload:           payload,
		Status:            model.ProjectPlatformSyncTaskStatusPending,
		CreatedBy:         updatedBy,
		UpdatedBy:         updatedBy,
	}
	return enqueueAndRunDevopsSyncTask(ctx, svcCtx, task)
}

const (
	devopsSyncActionProjectInfo    = "project_info"
	devopsSyncActionProjectDelete  = "project_delete"
	devopsSyncActionProjectMembers = "project_members"
)

func enqueueAndRunDevopsSyncTask(ctx context.Context, svcCtx *svc.ServiceContext, task *model.ProjectPlatformSyncTask) error {
	if task == nil {
		return nil
	}
	if task.Status == "" {
		task.Status = model.ProjectPlatformSyncTaskStatusPending
	}
	if task.CreatedBy == "" {
		task.CreatedBy = "system"
	}
	if task.UpdatedBy == "" {
		task.UpdatedBy = task.CreatedBy
	}
	if err := svcCtx.ProjectPlatformSyncTaskModel.Upsert(ctx, task); err != nil {
		return err
	}
	return svcCtx.ExecuteDevopsSyncTask(ctx, task)
}
