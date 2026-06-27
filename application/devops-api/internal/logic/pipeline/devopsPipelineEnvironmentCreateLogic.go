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

type DevopsPipelineEnvironmentCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建流水线环境
func NewDevopsPipelineEnvironmentCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineEnvironmentCreateLogic {
	return &DevopsPipelineEnvironmentCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineEnvironmentCreateLogic) DevopsPipelineEnvironmentCreate(req *types.CreateDevopsPipelineEnvironmentRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.PipelineEnvironmentCreate(l.ctx, &pipelineconfigservice.CreatePipelineEnvironmentReq{
		Name:          req.Name,
		Code:          req.Code,
		Description:   req.Description,
		Icon:          req.Icon,
		IconColor:     req.IconColor,
		SortOrder:     req.SortOrder,
		Status:        req.Status,
		CreatedBy:     currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.IdResponse{Id: result.Id}, nil
}
