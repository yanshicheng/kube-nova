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

type DevopsSystemGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 系统详情
func NewDevopsSystemGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsSystemGetLogic {
	return &DevopsSystemGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsSystemGetLogic) DevopsSystemGet(req *types.DefaultStringIdRequest) (resp *types.DevopsSystem, err error) {
	result, err := l.svcCtx.PipelineRpc.SystemGet(l.ctx, &pipelineconfigservice.GetByIdReq{
		Id:            req.Id,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	data := systemToType(result.Data)

	return &data, nil
}
