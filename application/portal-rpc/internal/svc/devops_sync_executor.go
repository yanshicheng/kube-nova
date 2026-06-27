package svc

import (
	"context"

	devopspb "github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
)

func (s *ServiceContext) ExecuteDevopsSyncTask(ctx context.Context, task *model.ProjectPlatformSyncTask) error {
	if task == nil {
		return nil
	}
	payload, err := model.UnmarshalProjectPlatformSyncTaskPayload(task.Payload)
	if err != nil {
		return err
	}
	switch task.Action {
	case "project_info":
		_, err = s.DevopsManagerRpc.SyncProjectInfo(ctx, &devopspb.DevopsSyncProjectInfoReq{
			PortalProjectUuid: payload.PortalProjectUuid,
			Name:              payload.Name,
			Description:       payload.Description,
		})
	case "project_delete":
		_, err = s.DevopsManagerRpc.SyncProjectDeleted(ctx, &devopspb.DevopsSyncProjectDeletedReq{
			PortalProjectUuid: payload.PortalProjectUuid,
			UpdatedBy:         payload.UpdatedBy,
		})
	case "project_members":
		inputs := make([]*devopspb.DevopsSyncProjectMemberInput, 0, len(payload.Members))
		for _, item := range payload.Members {
			inputs = append(inputs, &devopspb.DevopsSyncProjectMemberInput{
				UserId:   item.UserId,
				Username: item.Username,
				Nickname: item.Nickname,
				Role:     item.Role,
			})
		}
		_, err = s.DevopsManagerRpc.SyncProjectMembers(ctx, &devopspb.DevopsSyncProjectMembersReq{
			PortalProjectUuid: payload.PortalProjectUuid,
			Members:           inputs,
			UpdatedBy:         payload.UpdatedBy,
		})
	default:
		return nil
	}
	if err != nil {
		_ = s.ProjectPlatformSyncTaskModel.MarkFailed(ctx, task.PortalProjectUuid, task.Action, err.Error())
		return err
	}
	_ = s.ProjectPlatformSyncTaskModel.MarkSuccess(ctx, task.PortalProjectUuid, task.Action)
	return nil
}

