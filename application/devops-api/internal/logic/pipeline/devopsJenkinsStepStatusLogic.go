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

type DevopsJenkinsStepStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 Jenkins 步骤状态
func NewDevopsJenkinsStepStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsJenkinsStepStatusLogic {
	return &DevopsJenkinsStepStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsJenkinsStepStatusLogic) DevopsJenkinsStepStatus(req *types.UpdateStatusRequest) error {
	_, err := l.svcCtx.PipelineRpc.JenkinsStepStatus(l.ctx, &pipelineconfigservice.UpdateStatusReq{
		Id:        req.Id,
		Status:    req.Status,
		UpdatedBy: currentUsername(l.ctx),
	})
	return err
}
