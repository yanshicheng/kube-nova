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

type DevopsJenkinsStepCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 Jenkins 步骤
func NewDevopsJenkinsStepCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsJenkinsStepCreateLogic {
	return &DevopsJenkinsStepCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsJenkinsStepCreateLogic) DevopsJenkinsStepCreate(req *types.CreateDevopsStepTemplateRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.JenkinsStepCreate(l.ctx, &pipelineconfigservice.CreateStepTemplateReq{
		Name:           req.Name,
		Code:           req.Code,
		Icon:           req.Icon,
		IconColor:      req.IconColor,
		Description:    req.Description,
		Type:           req.Type,
		CategoryId:     req.CategoryId,
		StageContent:   req.StageContent,
		Params:         stepParamsToRpc(req.Params),
		ArtifactConfig: artifactConfigToRpc(req.ArtifactConfig),
		Status:         req.Status,
		CreatedBy:      currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.IdResponse{Id: result.Id}, nil
}
