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

type DevopsSystemDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除系统
func NewDevopsSystemDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsSystemDeleteLogic {
	return &DevopsSystemDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsSystemDeleteLogic) DevopsSystemDelete(req *types.DefaultStringIdRequest) error {
	_, err := l.svcCtx.PipelineRpc.SystemDelete(l.ctx, &pipelineconfigservice.DeleteByIdReq{
		Id:            req.Id,
		UpdatedBy:     currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	return err
}
