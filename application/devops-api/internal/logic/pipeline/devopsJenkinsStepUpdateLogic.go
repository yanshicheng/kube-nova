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

type DevopsJenkinsStepUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 Jenkins 步骤
func NewDevopsJenkinsStepUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsJenkinsStepUpdateLogic {
	return &DevopsJenkinsStepUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsJenkinsStepUpdateLogic) DevopsJenkinsStepUpdate(req *types.UpdateDevopsStepTemplateRequest) error {
	_, err := l.svcCtx.PipelineRpc.JenkinsStepUpdate(l.ctx, &pipelineconfigservice.UpdateStepTemplateReq{
		Id:             req.Id,
		Name:           req.Name,
		Icon:           req.Icon,
		IconColor:      req.IconColor,
		Description:    req.Description,
		Type:           req.Type,
		CategoryId:     req.CategoryId,
		StageContent:   req.StageContent,
		Params:         stepParamsToRpc(req.Params),
		ArtifactConfig: artifactConfigToRpc(req.ArtifactConfig),
		Status:         req.Status,
		UpdatedBy:      currentUsername(l.ctx),
	})
	return err
}
