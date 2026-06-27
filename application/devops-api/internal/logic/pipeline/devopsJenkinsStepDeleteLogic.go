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

type DevopsJenkinsStepDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 Jenkins 步骤
func NewDevopsJenkinsStepDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsJenkinsStepDeleteLogic {
	return &DevopsJenkinsStepDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsJenkinsStepDeleteLogic) DevopsJenkinsStepDelete(req *types.DefaultStringIdRequest) error {
	_, err := l.svcCtx.PipelineRpc.JenkinsStepDelete(l.ctx, &pipelineconfigservice.DeleteByIdReq{
		Id:        req.Id,
		UpdatedBy: currentUsername(l.ctx),
	})
	return err
}
