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

type DevopsStepChannelParamMappingDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除步骤参数到渠道分组映射
func NewDevopsStepChannelParamMappingDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsStepChannelParamMappingDeleteLogic {
	return &DevopsStepChannelParamMappingDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsStepChannelParamMappingDeleteLogic) DevopsStepChannelParamMappingDelete(req *types.DefaultStringIdRequest) error {
	_, err := l.svcCtx.PipelineRpc.DeleteStepChannelParamMapping(l.ctx, &pipelineconfigservice.DeleteByIdReq{
		Id:        req.Id,
		UpdatedBy: currentUsername(l.ctx),
	})
	return err
}
