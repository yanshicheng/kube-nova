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

type DevopsPipelineEnvironmentUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新流水线环境
func NewDevopsPipelineEnvironmentUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineEnvironmentUpdateLogic {
	return &DevopsPipelineEnvironmentUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineEnvironmentUpdateLogic) DevopsPipelineEnvironmentUpdate(req *types.UpdateDevopsPipelineEnvironmentRequest) error {
	_, err := l.svcCtx.PipelineRpc.PipelineEnvironmentUpdate(l.ctx, &pipelineconfigservice.UpdatePipelineEnvironmentReq{
		Id:            req.Id,
		Name:          req.Name,
		Description:   req.Description,
		Icon:          req.Icon,
		IconColor:     req.IconColor,
		SortOrder:     req.SortOrder,
		Status:        req.Status,
		UpdatedBy:     currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
