// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pipeline

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsJenkinsCredentialSyncDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 Jenkins 凭证副本
func NewDevopsJenkinsCredentialSyncDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsJenkinsCredentialSyncDeleteLogic {
	return &DevopsJenkinsCredentialSyncDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsJenkinsCredentialSyncDeleteLogic) DevopsJenkinsCredentialSyncDelete(req *types.DeleteJenkinsCredentialSyncRequest) error {
	_, err := l.svcCtx.PipelineRpc.JenkinsCredentialSyncDelete(l.ctx, &pipelineconfigservice.DeleteJenkinsCredentialSyncReq{
		ProjectId:             req.ProjectId,
		BuildChannelBindingId: req.BuildChannelBindingId,
		CredentialId:          req.CredentialId,
		CurrentUserId:         currentUserID(l.ctx),
		CurrentRoles:          currentRoles(l.ctx),
		UpdatedBy:             currentUsername(l.ctx),
	})
	if err != nil {
		return err
	}

	return nil
}
