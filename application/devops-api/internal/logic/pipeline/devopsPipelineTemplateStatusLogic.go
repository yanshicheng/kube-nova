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

type DevopsPipelineTemplateStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新流水线模板状态
func NewDevopsPipelineTemplateStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineTemplateStatusLogic {
	return &DevopsPipelineTemplateStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineTemplateStatusLogic) DevopsPipelineTemplateStatus(req *types.UpdateStatusRequest) error {
	_, err := l.svcCtx.PipelineRpc.PipelineTemplateStatus(l.ctx, &pipelineconfigservice.UpdateStatusReq{
		Id:            req.Id,
		Status:        req.Status,
		UpdatedBy:     currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
