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

type DevopsJenkinsCredentialSyncListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询 Jenkins 凭证同步记录
func NewDevopsJenkinsCredentialSyncListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsJenkinsCredentialSyncListLogic {
	return &DevopsJenkinsCredentialSyncListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsJenkinsCredentialSyncListLogic) DevopsJenkinsCredentialSyncList(req *types.ListJenkinsCredentialSyncRequest) (resp *types.ListJenkinsCredentialSyncResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.JenkinsCredentialSyncList(l.ctx, &pipelineconfigservice.ListJenkinsCredentialSyncReq{
		ProjectId:             req.ProjectId,
		BuildChannelBindingId: req.BuildChannelBindingId,
		Page:                  req.Page,
		PageSize:              req.PageSize,
		CurrentUserId:         currentUserID(l.ctx),
		CurrentRoles:          currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.JenkinsCredentialSyncRecord, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, jenkinsCredentialSyncRecordToType(item))
	}

	return &types.ListJenkinsCredentialSyncResponse{Items: items, Total: result.Total}, nil
}
