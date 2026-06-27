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

type DevopsPipelineTemplateVariablesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查看流水线模板变量
func NewDevopsPipelineTemplateVariablesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineTemplateVariablesLogic {
	return &DevopsPipelineTemplateVariablesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineTemplateVariablesLogic) DevopsPipelineTemplateVariables(req *types.DefaultStringIdRequest) (resp *types.PipelineTemplateVariablesResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.PipelineTemplateVariables(l.ctx, &pipelineconfigservice.GetByIdReq{
		Id:            req.Id,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.PipelineTemplateVariable, 0, len(result.Data))
	for _, item := range result.Data {
		items = append(items, pipelineVariableToType(item))
	}

	return &types.PipelineTemplateVariablesResponse{Items: items, Total: result.Total}, nil
}
