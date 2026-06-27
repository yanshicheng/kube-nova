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

type DevopsPipelineEnvironmentGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 流水线环境详情
func NewDevopsPipelineEnvironmentGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineEnvironmentGetLogic {
	return &DevopsPipelineEnvironmentGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineEnvironmentGetLogic) DevopsPipelineEnvironmentGet(req *types.DefaultStringIdRequest) (resp *types.DevopsPipelineEnvironment, err error) {
	result, err := l.svcCtx.PipelineRpc.PipelineEnvironmentGet(l.ctx, &pipelineconfigservice.GetByIdReq{
		Id:            req.Id,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	data := environmentToType(result.Data)

	return &data, nil
}
