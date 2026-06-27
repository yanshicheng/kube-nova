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

type DevopsJenkinsCredentialReferencesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询 Jenkins 凭证引用流水线
func NewDevopsJenkinsCredentialReferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsJenkinsCredentialReferencesLogic {
	return &DevopsJenkinsCredentialReferencesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsJenkinsCredentialReferencesLogic) DevopsJenkinsCredentialReferences(req *types.JenkinsCredentialReferenceRequest) (resp *types.JenkinsCredentialReferenceResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.JenkinsCredentialReferences(l.ctx, &pipelineconfigservice.JenkinsCredentialReferenceReq{
		ProjectId:             req.ProjectId,
		BuildChannelBindingId: req.BuildChannelBindingId,
		CredentialId:          req.CredentialId,
		CurrentUserId:         currentUserID(l.ctx),
		CurrentRoles:          currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.JenkinsCredentialReference, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, jenkinsCredentialReferenceToType(item))
	}

	return &types.JenkinsCredentialReferenceResponse{Items: items, Total: result.Total}, nil
}
